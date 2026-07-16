package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"

	"github.com/manuelringwald/wayfinder/internal/webui"
	"github.com/manuelringwald/wayfinder/pkg/adminapi"
	"github.com/manuelringwald/wayfinder/pkg/aeronautical"
	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/broadcast"
	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/cat063"
	"github.com/manuelringwald/wayfinder/pkg/cat065"
	"github.com/manuelringwald/wayfinder/pkg/correlationapi"
	"github.com/manuelringwald/wayfinder/pkg/coverage"
	"github.com/manuelringwald/wayfinder/pkg/feature"
	"github.com/manuelringwald/wayfinder/pkg/feedmanager"
	"github.com/manuelringwald/wayfinder/pkg/fireflycmd"
	"github.com/manuelringwald/wayfinder/pkg/health"
	"github.com/manuelringwald/wayfinder/pkg/impersonation"
	"github.com/manuelringwald/wayfinder/pkg/metrics"
	"github.com/manuelringwald/wayfinder/pkg/orchestrator"
	"github.com/manuelringwald/wayfinder/pkg/secret"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
	"github.com/manuelringwald/wayfinder/pkg/weather"
	"github.com/manuelringwald/wayfinder/pkg/weathertiles"
	"github.com/manuelringwald/wayfinder/pkg/weatherwarnings"
	"github.com/manuelringwald/wayfinder/pkg/ws"
)

func main() {
	// Subcommand dispatch: `wayfinder bootstrap …` provisions the first
	// tenant/admin user (WF2-13); `wayfinder feed …` manages the feed catalogue
	// (WF2-20.2). Each runs and exits; with no subcommand the ASD server runs.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bootstrap":
			if err := bootstrapCommand(os.Args[2:], os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "bootstrap:", err)
				os.Exit(1)
			}
			return
		case "feed":
			if err := feedCommand(os.Args[2:], os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "feed:", err)
				os.Exit(1)
			}
			return
		}
	}

	// Load configuration from environment.
	cfg := loadConfig()

	// Setup logging.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	if cfg.OpenAIPRefreshDeprecated {
		logger.Warn("WAYFINDER_OPENAIP_REFRESH is deprecated and ignored (AERO-1, ADR 0018): " +
			"OpenAIP is fetched once/on-demand and persisted, not on a periodic ticker")
	}

	// Create broadcaster.
	broadcaster := broadcast.New(logger)

	// Track reception state for health checks and /metrics.
	var blockCount atomic.Int64
	var trackCount atomic.Int64
	var tracksCurrent atomic.Int64
	var heartbeatCount atomic.Int64
	var lastError atomic.Pointer[string]

	// Feed-health registry: per-feed heartbeat staleness + track presence (AP4,
	// Firefly ADR 0018). Replaces the former single-instance FeedHealth; the
	// Registry exposes aggregate Status/Observe methods as drop-in replacements.
	feedRegistry := health.NewRegistry(cfg.FeedStaleTimeout)

	// The aeronautical (OpenAIP) global fallback Service is built after the DB pool
	// is open — since AERO-1 (ADR 0018) it reads/writes a persistent DB cache, so it
	// needs the store. See below, next to the per-tenant registry.

	// Weather-radar overlay (WX-A, ADR 0016): best-effort DWD GeoServer WMS tile
	// proxy. Enabled only when a WMS URL is configured; otherwise it serves
	// transparent tiles. Like the aeronautical layers it never touches the track
	// path or readiness. No background loop — tiles are fetched on demand and
	// cached for the refresh window.
	radarEnabled := cfg.DWDRadarEnabled && cfg.DWDWMSURL != ""
	weatherRadar := weathertiles.NewService(
		weathertiles.NewClient(&http.Client{Timeout: 15 * time.Second}, cfg.DWDWMSURL, cfg.DWDRadarLayer).WithStyle(cfg.DWDRadarStyle),
		weathertiles.Config{Enabled: radarEnabled, TTL: cfg.DWDRefresh},
		logger,
	)
	if !radarEnabled {
		logger.Warn("weather radar overlay disabled (WAYFINDER_DWD_RADAR_ENABLED=false); map will show no DWD radar")
	}

	// The QNH poller (WX-B / CBD-3) is built after the DB pool is open — its poll
	// set is the union of the tenants' configured aerodromes, which lives in the DB.

	// Weather-warnings overlay (WX-C, ADR 0016): best-effort DWD WFS GeoJSON.
	// Connected-by-default (ADR 0017): on unless WAYFINDER_DWD_WARN_ENABLED=false.
	// Polls in its own goroutine; the map fetches /api/weather/warnings.geojson.
	warnEnabled := cfg.DWDWarnEnabled && cfg.DWDWarnURL != ""
	weatherWarn := weatherwarnings.NewService(
		weatherwarnings.NewClient(&http.Client{Timeout: 15 * time.Second}, cfg.DWDWarnURL, cfg.DWDWarnLayer),
		weatherwarnings.Config{Enabled: warnEnabled, Refresh: cfg.DWDWarnRefresh},
		logger,
	)

	// broadcastFeedSnapshot pushes the per-feed health snapshot to clients
	// subscribed to that feed (Option B, WF-3). Color is derived from the
	// FeedSnapshot and carries CAT065 liveness + CAT063 sensor counts.
	broadcastFeedSnapshot := func(feedID int64, snap health.FeedSnapshot) {
		var sensors []broadcast.FeedSensor
		for _, s := range snap.Sensors {
			sensors = append(sensors, broadcast.FeedSensor{
				SAC:            s.SAC,
				SIC:            s.SIC,
				Operational:    s.Operational,
				DegradedReason: s.Reason,
				RangeBiasM:     s.RangeBiasM,
				AzimuthBiasDeg: s.AzimuthBiasDeg,
			})
		}
		_ = broadcaster.Send(broadcast.Message{
			FeedStatus: &broadcast.FeedStatusMessage{
				FeedID:         feedID,
				Color:          snap.Color(),
				SensorsActive:  snap.SensorsActive,
				SensorsTotal:   snap.SensorsTotal,
				DegradedReason: snap.DegradedReason,
				Sensors:        sensors,
			},
		})
	}

	// Track handler: feed decoded tracks (tagged with their feed_id, WF2-20) to
	// the broadcaster. Shared by every feed receiver.
	trackHandler := func(feedID int64, tracks []cat062.DecodedTrack) error {
		blockCount.Add(1)
		trackCount.Add(int64(len(tracks)))
		tracksCurrent.Store(int64(len(tracks)))
		feedRegistry.RecordTracks(feedID, len(tracks))
		// Feed tracks to broadcaster (non-blocking), tagged with their feed.
		select {
		case broadcaster.TracksChan() <- broadcast.TrackBatch{FeedID: feedID, Tracks: tracks}:
		default:
			logger.Warn("broadcaster channel full, dropping block")
		}
		return nil
	}
	// statusHandler: CAT065 heartbeats update the per-feed registry (AP4) and
	// broadcast the per-feed snapshot to subscribed clients. buildReceivers wraps
	// this with a per-feed closure so feedID is the catalogue ID of the receiver.
	statusHandler := func(feedID int64, status cat065.ServiceStatus) error {
		heartbeatCount.Add(1)
		feedRegistry.RecordHeartbeat(feedID, time.Now())
		broadcastFeedSnapshot(feedID, feedRegistry.Snapshot(feedID, time.Now()))
		return nil
	}
	// sensorStatusHandler: CAT063 per-sensor status updates the feed registry
	// with the current sensor active/total counts and broadcasts the snapshot.
	sensorStatusHandler := func(feedID int64, statuses []cat063.SensorStatus) error {
		active := 0
		details := make([]health.SensorDetail, len(statuses))
		for i, s := range statuses {
			if s.Operational {
				active++
			}
			// Per-sensor detail (#237): identity, state and applied registration
			// bias, so the ASD/admin can show WHICH sensor is degraded and how far
			// it is being range/azimuth-corrected.
			details[i] = health.SensorDetail{
				SAC:            s.SAC,
				SIC:            s.SIC,
				Operational:    s.Operational,
				Reason:         s.Reason,
				RangeBiasM:     s.RangeBiasM,
				AzimuthBiasDeg: s.AzimuthBiasDeg,
			}
		}
		// The dominant per-source failure reason of the degraded sensors (ADR 0033)
		// — surfaced on the feed-health chip so the operator sees WHY (#197).
		feedRegistry.RecordSensors(feedID, active, len(statuses), cat063.DominantReason(statuses), details)
		broadcastFeedSnapshot(feedID, feedRegistry.Snapshot(feedID, time.Now()))
		return nil
	}

	// Zero-touch onboarding (ONB-1, ADR 0011): builtin mode needs a session-signing
	// key, but requiring the operator to set one re-introduces a manual setup step.
	// When none is configured, generate an ephemeral random key and warn. The cost
	// is explicit: sessions do not survive a restart and are not shared across
	// replicas — a fixed WAYFINDER_SESSION_KEY remains the production recommendation.
	if cfg.AuthMode == auth.ModeBuiltin && len(cfg.SessionKey) == 0 {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			logger.Error("generate ephemeral session key", slog.String("error", err.Error()))
			os.Exit(1)
		}
		cfg.SessionKey = key
		logger.Warn("WAYFINDER_SESSION_KEY not set — generated an ephemeral key; " +
			"sessions reset on restart and are not multi-replica safe. Set a fixed " +
			"key (e.g. openssl rand -hex 32) for production (ADR 0011)")
	}

	// Multi-tenancy (WF2-12, ADR 0014): open the DB, migrate and build the
	// tenant-context middleware. WAYFINDER_DB_URL is mandatory — setupTenancy
	// fails the start when it is missing (no single-tenant fallback). Done before
	// the receivers so the feed catalogue (WF2-20.2) can drive them.
	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 30*time.Second)
	tenantMW, dbPool, err := setupTenancy(setupCtx, cfg, logger)
	if err != nil {
		cancelSetup()
		logger.Error("tenancy setup", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbPool.Close()

	// QNH infobox (WX-B, ADR 0016; per-tenant CBD-3, ADR 0017). Connected-by-default:
	// the NOAA/AWC METAR source is ON unless WAYFINDER_QNH_ENABLED=false. The poll set
	// is dynamic — the union of every tenant's configured aerodrome (view_configs
	// .qnh_icao) — read fresh each refresh so a newly set aerodrome is picked up
	// without a restart; WAYFINDER_METAR_STATIONS remains a deprecated global
	// fallback. Polls in its own goroutine; the header reads /api/weather/qnh scoped
	// to the caller's tenant. Never touches the track path or readiness.
	qnhViews := store.NewViewConfigRepo(dbPool)
	qnhStations := func() []string {
		lookupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		icaos, err := qnhViews.DistinctQNHICAOs(lookupCtx)
		if err != nil {
			logger.Warn("qnh: list per-tenant aerodromes failed; using static fallback only",
				slog.String("error", err.Error()))
			return nil
		}
		return icaos
	}
	weatherQNH := weather.NewService(
		weather.NewClient(&http.Client{Timeout: 15 * time.Second}, cfg.MetarURL, cfg.MetarUserAgent),
		weather.Config{Enabled: cfg.QNHEnabled, Stations: cfg.MetarStations, StationsProvider: qnhStations, Refresh: cfg.QNHRefresh},
		logger,
	)
	if !cfg.QNHEnabled {
		logger.Warn("QNH source disabled (WAYFINDER_QNH_ENABLED=false); header shows no QNH")
	}

	// Aeronautical layers (ASD-003, ADR 0004): best-effort OpenAIP overlays. Enabled
	// only when an API key is configured; never affects the track path or readiness.
	// The query window is a box around the configured map center. Since AERO-1 (ADR
	// 0018) the fetched GeoJSON is persisted in the DB (aeroCache) and fetched
	// once/on-demand instead of on a ticker — WAYFINDER_OPENAIP_REFRESH is obsolete.
	// This global Service is the fallback cache (tenant_id NULL) behind the per-tenant
	// registry.
	aeroCache := newAeroCacheStore(store.NewAeroCacheRepo(dbPool))
	aeroService := aeronautical.NewService(
		aeronautical.NewClient(&http.Client{Timeout: 15 * time.Second}, cfg.OpenAIPBaseURL, cfg.OpenAIPAPIKey),
		aeronautical.Config{
			Enabled:  cfg.OpenAIPAPIKey != "",
			BBox:     aeronautical.BoundingBoxFromCenter(cfg.MapCenterLat, cfg.MapCenterLon, cfg.OpenAIPRadiusKM),
			Store:    aeroCache,
			TenantID: nil, // global fallback row
		},
		logger,
	)

	// Server-side session registry (AP7, ADR 0009 §5). Built once and shared by the
	// login handlers, the admin revocation hooks, the janitor and the /metrics
	// gauge. Only in builtin mode — a proxy session lives in the upstream OIDC
	// proxy, not in this registry — so it stays nil under proxy auth.
	var sessionRepo *store.SessionRepo
	if cfg.AuthMode == auth.ModeBuiltin {
		sessionRepo = store.NewSessionRepo(dbPool)
	}

	// Resolve the feeds to receive from the DB catalogue (multi-feed, WF2-20.2).
	// An empty catalogue falls back to the single ENV-configured feed so a fresh
	// instance always has something to receive (resolveFeeds).
	catalogue, err := store.NewFeedRepo(dbPool).List(setupCtx)
	cancelSetup()
	if err != nil {
		logger.Error("list feed catalogue", slog.String("error", err.Error()))
		os.Exit(1)
	}
	feeds := resolveFeeds(catalogue, cfg)
	logger.Info("feeds resolved", slog.Int("count", len(feeds)))

	// Graceful shutdown on SIGTERM/SIGINT. Created before the feed manager so every
	// receiver is a child of this context (cancel → all feeds leave their groups).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Process-wide, churn-stable decode-error counter (ONB-5): receivers come and go
	// as feeds are added/removed at runtime, so the /metrics counter is accumulated
	// here rather than summed over the live receiver set (which would make the
	// monotonic counter drop when a feed is deleted).
	var decodeErrorCount atomic.Int64

	// Live feed manager (ONB-5, ADR 0011): supervises one receiver per feed and lets
	// the admin API join/leave multicast groups at runtime without a restart. The
	// factory wires the shared handlers and stamps each feed's id onto its tracks.
	factory := newReceiverFactory(logger, trackHandler, statusHandler, sensorStatusHandler, func() { decodeErrorCount.Add(1) })
	feedManager := feedmanager.New(ctx, factory, logger)

	// Start the resolved feeds (DB catalogue or the ENV fallback). A feed that fails
	// to join is logged and skipped so one misconfigured feed doesn't sink the
	// others; if none can join, fatal. The manager owns the receiver goroutines and
	// their clean teardown (StopAll on shutdown).
	for _, f := range feeds {
		if err := feedManager.Start(feedmanager.Feed{ID: f.ID, Name: f.Name, Group: f.Group, Port: f.Port}); err != nil {
			logger.Error("feed start failed",
				slog.Int64("feed_id", f.ID), slog.String("group", f.Group),
				slog.Int("port", f.Port), slog.String("error", err.Error()))
			continue
		}
	}
	if len(feedManager.Running()) == 0 {
		logger.Error("no feeds could be joined")
		os.Exit(1)
	}
	defer feedManager.StopAll()

	// Run the broadcaster in parallel with the manager-owned receivers.
	var wg sync.WaitGroup

	// Broadcaster goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := broadcaster.Run(ctx); err != nil && err != context.Canceled {
			msg := err.Error()
			lastError.Store(&msg)
			logger.Error("broadcaster error", slog.String("error", err.Error()))
			cancel()
		}
	}()

	// Monitor feed staleness: periodically re-evaluate each feed's heartbeat age
	// and broadcast per-feed snapshots when the aggregate state changes (e.g.
	// ok→stale or recovery). Covers the case where no traffic arrives at all.
	// Iterates the *live* feed set (feedManager.Running) so feeds added or removed
	// at runtime (ONB-5) are tracked without a restart.
	go func() {
		interval := cfg.FeedStaleTimeout / 3
		if interval < 250*time.Millisecond {
			interval = 250 * time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, changed := feedRegistry.Observe(time.Now()); changed {
					now := time.Now()
					for _, fid := range feedManager.Running() {
						broadcastFeedSnapshot(fid, feedRegistry.Snapshot(fid, now))
					}
				}
			}
		}
	}()

	// Bootstrap the global fallback aeronautical cache (best-effort, ADR 0004; AERO-1,
	// ADR 0018): hydrate from the persistent DB cache (no network) and fetch once only
	// if there is a key but nothing persisted yet. No background ticker.
	go aeroService.BootstrapOnce(ctx)

	// Start the QNH METAR poller (best-effort, WX-B, ADR 0016). No-op when no
	// stations are configured. Bound to the shutdown context.
	go weatherQNH.Run(ctx)

	// Start the weather-warnings poller (best-effort, WX-C, ADR 0016). No-op when
	// no WFS URL is configured. Bound to the shutdown context.
	go weatherWarn.Run(ctx)

	// Shared AES-256-GCM cipher (WAYFINDER_SECRET_KEY) for at-rest secrets: per-feed
	// source credentials (ORCH-2c) and the global OpenAIP key (AERO-2). nil when the
	// key is unset/invalid — the dependent write paths then fail closed (503) rather
	// than store a plaintext secret. Built once and reused.
	var aeroCipher *secret.Cipher
	if len(cfg.SecretKey) == secret.KeySize {
		if c, err := secret.NewCipher(cfg.SecretKey); err == nil {
			aeroCipher = c
		} else {
			logger.Warn("WAYFINDER_SECRET_KEY rejected — at-rest encryption disabled", slog.String("error", err.Error()))
		}
	} else if os.Getenv("WAYFINDER_SECRET_KEY") != "" {
		logger.Warn("WAYFINDER_SECRET_KEY set but invalid (need base64-encoded 32 bytes) — at-rest encryption disabled")
	}
	// Global OpenAIP key (AERO-2, ADR 0018): runtime-set via the platform UI, sealed
	// at rest with the cipher above; the env WAYFINDER_OPENAIP_API_KEY is the keyless
	// fallback. A true-nil interface when no cipher, so Available() reports false.
	var aeroCipherIface secretCipher
	if aeroCipher != nil {
		aeroCipherIface = aeroCipher
	}
	globalAero := newGlobalOpenAIP(store.NewSettingsRepo(dbPool), aeroCipherIface, cfg.OpenAIPAPIKey, logger)

	// OpenAIP per tenant (ONB-6, ADR 0011): each tenant fetches OpenAIP with its own
	// key (or the global fallback) against its own area of interest. The registry
	// holds one Service per tenant alongside the global fallback; a tenant without
	// its own service falls back to the global cache. Since AERO-1 (ADR 0018) each
	// per-tenant Service reads/writes the shared persistent cache and fetches
	// once/on-demand (no ticker). The global fallback key is dynamic (AERO-2).
	aeroRegistry := aeronautical.NewRegistry(ctx, aeroService, newAeroClientFactory(cfg.OpenAIPBaseURL), aeroCache, logger)
	defer aeroRegistry.StopAll()
	aeroLifeImpl := tenantAeroLifecycle{
		reg:       aeroRegistry,
		tenants:   store.NewTenantRepo(dbPool),
		views:     store.NewViewConfigRepo(dbPool),
		globalKey: globalAero.effectiveKey,
		radiusKM:  cfg.OpenAIPRadiusKM,
		fallback:  aeronautical.BoundingBoxFromCenter(cfg.MapCenterLat, cfg.MapCenterLon, cfg.OpenAIPRadiusKM),
		logger:    logger,
	}
	var aeroLife adminapi.TenantAeroLifecycle = aeroLifeImpl
	// Boot: bring up each tenant's per-tenant refresh from the catalogue, so the
	// per-tenant caches are warm without waiting for an admin edit.
	bootCtx, cancelBoot := context.WithTimeout(ctx, 30*time.Second)
	if tenants, terr := store.NewTenantRepo(dbPool).List(bootCtx); terr != nil {
		logger.Warn("openaip: list tenants for per-tenant refresh failed", slog.String("error", terr.Error()))
	} else {
		for _, t := range tenants {
			aeroLifeImpl.Apply(bootCtx, t.ID)
		}
	}
	cancelBoot()

	// Feature entitlements (WF2-50): per-tenant feature flags as data, fail-closed.
	// Built once here so the admin API and /metrics share one instance.
	featSvc := feature.New(store.NewEntitlementRepo(dbPool), logger)

	// Start health/readiness/metrics probe server. Decode errors are read from the
	// process-wide counter (monotonic across feed churn, ONB-5).
	decodeErrors := func() int64 { return decodeErrorCount.Load() }
	// OpenAIP fetch counters (ONB-6): sum across every per-tenant Service plus the
	// global one so the process-wide counter stays meaningful as tenants come and go.
	aeroCounts := func() (int64, int64) { return aeroRegistry.FetchSuccessCount(), aeroRegistry.FetchFailureCount() }
	go startProbeServer(logger, &blockCount, &trackCount, &tracksCurrent, &heartbeatCount, broadcaster, decodeErrors, feedRegistry, aeroService, aeroCounts, weatherRadar, weatherQNH, weatherWarn, &lastError, featSvc, sessionRepo)

	// Start WebSocket server.
	mux := http.NewServeMux()

	// Scoped fan-out (WF2-21, ADR 0014): every /ws client is filtered to the feeds
	// its tenant subscribes to. Cross-tenant read-only impersonation (ADR 0008)
	// needs a signing key; without WAYFINDER_SESSION_KEY (e.g. proxy mode) it stays
	// disabled and the resolver behaves as if impChecker were nil.
	var impChecker impersonation.TenantChecker
	if len(cfg.SessionKey) > 0 {
		impChecker = tenantExistsChecker{repo: store.NewTenantRepo(dbPool)}
	}
	// The same grant is honoured on the map's plain read-only REST endpoints
	// (whoami, aeronautical overlays, QNH) so an impersonating admin sees the
	// FULL tenant picture, not just the /ws track stream (ADR 0008 Nachtrag).
	// Disabled (identity passthrough) without a signing key, like /ws.
	impReadMW := func(next http.Handler) http.Handler { return next }
	if impChecker != nil {
		impReadMW = impersonationReadMW(impChecker, cfg.SessionKey, logger.With(slog.String("component", "audit")))
	}
	scopeResolver := newScopeResolver(store.NewSubscriptionRepo(dbPool), store.NewViewConfigRepo(dbPool), impChecker, cfg.SessionKey, logger)
	wsHandler := ws.New(broadcaster, logger, cfg.AllowedOrigins, scopeResolver)
	// #208 (ADR 0022): while a principal's must_change_password flag is set, the
	// well-known seed credential must reach NOTHING but the password-change flow.
	// The admin API enforces that via its allowlist (pkg/adminapi); pwGate extends
	// the same fail-closed rule to every operational data path mounted below
	// (/ws, overlays, weather) — path-independent, no matter which URL the
	// principal logged in through.
	pwGate := tenant.RequirePasswordChanged
	// The live picture is tenant-scoped: gate /ws with the tenant middleware, which
	// sets the Identity the resolver reads.
	mux.Handle("/ws", tenantMW(pwGate(wsHandler)))

	// Serve the ASD frontend (static HTML/JS/CSS) and its map configuration.
	frontend, err := webui.Handler()
	if err != nil {
		logger.Error("create frontend handler", slog.String("error", err.Error()))
		os.Exit(1)
	}
	mux.Handle("/", frontend)
	// Self-hosted MapLibre glyph PBFs (Roboto Mono) — the scope's data-block
	// font, served from the binary so no runtime font CDN is needed (air-gap).
	glyphs, err := webui.GlyphsHandler()
	if err != nil {
		logger.Error("create glyphs handler", slog.String("error", err.Error()))
		os.Exit(1)
	}
	mux.Handle("/glyphs/", glyphs)
	mux.HandleFunc("/api/map-config", mapConfigHandler(cfg))

	// Aeronautical GeoJSON endpoints (/api/airspace, /api/navaids,
	// /api/waypoints), served from the OpenAIP cache (ADR 0004). They are
	// tenant-aware (ONB-6): behind the tenant middleware, each serves the
	// requesting tenant's own per-tenant cache (with global fallback).
	// tenantMW sets the Identity, impReadMW resolves an impersonation grant into
	// the effective read tenant (ADR 0008 Nachtrag), readTenantOf serves that
	// tenant's cache — so an impersonating admin sees the target's overlays and
	// the feature gate below judges the target's entitlements.
	aeroMW := func(next http.Handler) http.Handler { return tenantMW(pwGate(impReadMW(next))) }
	aeroRegistry.Register(mux, aeroMW, readTenantOf, func(ctx context.Context, tenantID int64, kind aeronautical.Kind) bool {
		// Server-enforced feature gate: a tenant whose airspaces/vor_ndb/waypoints
		// entitlement is off receives an empty collection for that kind (the overlay
		// does not appear). The frontend toggle is cosmetic; the server is the boundary.
		key, ok := aeroFeatureKey(kind)
		if !ok {
			return true // unmapped kind → not gated
		}
		return featSvc.HasFeature(ctx, tenantID, key)
	})

	// Coverage rings: static GeoJSON computed once from config, served to the
	// browser on demand. An empty FeatureCollection is returned when no sensors
	// are configured so the frontend can always fetch unconditionally.
	mux.HandleFunc("/api/coverage/rings", coverageRingsHandler(cfg))

	// Weather-radar tile proxy (WX-A, ADR 0016): MapLibre requests XYZ tiles from
	// Wayfinder; the backend translates each into a DWD WMS GetMap. Behind the
	// tenant middleware so only authenticated principals reach our egress (like
	// the aeronautical endpoints); the overlay itself is gated per tenant in the
	// UI via the weather_radar entitlement. A disabled/unreachable source serves
	// transparent tiles, never an error.
	mux.Handle("GET /api/weather/radar/{z}/{x}/{y}", tenantMW(pwGate(weatherRadar.TileHandler())))

	// QNH infobox (WX-B, ADR 0016; per-tenant CBD-3): the header polls this for the
	// current QNH of the caller's own aerodrome (view_configs.qnh_icao), resolved
	// from the tenant context; a tenant without one falls back to the deprecated
	// global list. Behind the tenant middleware; best-effort JSON (empty station
	// list when disabled/unset), never an error. UI-gated per tenant via the qnh
	// entitlement.
	// impReadMW: while impersonating, the aerodrome resolves against the TARGET
	// tenant's view config (ADR 0008 Nachtrag) — an impersonating admin has no
	// user override there, so this is the tenant default, exactly what a fresh
	// tenant user would see.
	mux.Handle("GET /api/weather/qnh", tenantMW(pwGate(impReadMW(weatherQNH.TenantHandler(func(r *http.Request) []string {
		id, ok := tenant.FromContext(r.Context())
		if !ok {
			return nil
		}
		vc, err := qnhViews.GetEffective(r.Context(), tenant.ReadTenant(r.Context(), id.TenantID), id.UserID)
		if err != nil || vc.QNHICAO == nil {
			return nil
		}
		return []string{*vc.QNHICAO}
	})))))

	// Weather-warnings GeoJSON (WX-C, ADR 0016): the map fetches this and renders
	// warning polygons coloured by severity. Behind the tenant middleware;
	// best-effort (empty collection when disabled), never an error. UI-gated per
	// tenant via the weather_warnings entitlement.
	mux.Handle("GET /api/weather/warnings.geojson", tenantMW(pwGate(weatherWarn.Handler())))

	// Airport reference-point overlay (#192): GeoJSON markers of aerodromes inside
	// the caller's view AOI, from the embedded offline OurAirports directory.
	// Behind the tenant middleware (impReadMW resolves impersonation into the read
	// tenant, like the aeronautical overlays); feature-gated per tenant
	// (feature.Airport) — no entitlement → empty collection.
	mux.Handle("GET /api/airports.geojson", tenantMW(pwGate(impReadMW(airportsHandler(qnhViews, featSvc, cfg.OpenAIPRadiusKM)))))

	// Runway centreline overlay (#192): GeoJSON LineStrings of runways inside the
	// caller's view AOI, from the embedded offline OurAirports directory. Same
	// tenant/impersonation/feature-gate posture as /api/airports.geojson.
	mux.Handle("GET /api/runways.geojson", tenantMW(pwGate(impReadMW(runwaysHandler(qnhViews, featSvc, cfg.OpenAIPRadiusKM)))))

	// Builtin-mode login/logout (WF2-12.3): only when the auth mode is builtin
	// (proxy mints no local sessions). These routes are intentionally
	// unauthenticated — they hand out the session the middleware later checks.
	if cfg.AuthMode == auth.ModeBuiltin {
		users := store.NewUserRepo(dbPool)
		creds := store.NewCredentialRepo(dbPool)
		tenants := store.NewTenantRepo(dbPool)
		loginCfg := tenant.LoginConfig{
			SessionKey:  cfg.SessionKey,
			CookieName:  cfg.SessionCookie,
			TTL:         cfg.SessionTTL,
			MaxLifetime: cfg.SessionMaxLife,
			Secure:      cfg.TLSCertFile != "" && cfg.TLSKeyFile != "",
			// Registry-backed sessions (AP7, ADR 0009 §5): login opens a session with
			// the concurrent-session limit enforced, logout deletes it, renew slides it.
			// Users lets the renew handler resolve a per-access limit when converting a
			// legacy cookie, so that path cannot bypass the configured limit.
			Sessions:            sessionRepo,
			Users:               users,
			SessionLimitDefault: cfg.SessionLimitDefault,
			SessionLimitPolicy:  cfg.SessionLimitPolicy,
			OnSessionOpened:     func() { sessionsOpened.Add(1) },
			OnLoginRejected:     func() { sessionsRejected.Add(1) },
		}
		mux.Handle("/api/login", tenant.LoginHandler(users, creds, tenants, loginCfg))
		mux.Handle("/api/logout", tenant.LogoutHandler(loginCfg))
		// Sliding-session refresh (WF2-12.5): re-mint the cookie for an already
		// authenticated principal. Behind the tenant middleware (needs the Identity),
		// unlike login/logout. The ASD calls it periodically while the picture is
		// open so an active console never logs out; an abandoned one still lapses.
		mux.Handle("POST /api/session/renew", tenantMW(tenant.RenewHandler(loginCfg)))
		// Reap expired session rows in the background (expiry is also enforced at
		// resolve time; this only stops dead rows accumulating). Bound to the shutdown
		// context so it stops cleanly.
		go runSessionJanitor(ctx, sessionRepo, sessionJanitorInterval, logger)
		logger.Info("builtin login enabled", slog.String("path", "/api/login"),
			slog.Int("session_limit_default", cfg.SessionLimitDefault),
			slog.String("session_limit_policy", string(loginCfg.SessionLimitPolicy)))
	}

	// Admin surface (WF2-13/31/32): the tenant-scoped admin REST API is role-gated
	// to admin (ADR 0009) and carries the whoami role probe the SPA reads on
	// entering /admin (GET /api/admin/whoami). The browser route /admin itself is no
	// longer a backend endpoint — it is served by the SPA shell via the history-mode
	// fallback in webui.Handler.
	requireAdmin := tenant.RequireRole(store.RoleAdmin)
	viewRepo := store.NewViewConfigRepo(dbPool)
	subRepo := store.NewSubscriptionRepo(dbPool)
	// Live-apply (WF2-33): when an admin changes a tenant's view or feed
	// grants, re-scope that tenant's connected clients in place — no reconnect.
	rescope := func(ctx context.Context, tenantID int64) {
		rescopeTenant(ctx, broadcaster, subRepo, viewRepo, logger, tenantID)
		// A view edit may have changed this tenant's QNH aerodrome; kick the poller
		// so a freshly set station is fetched promptly instead of at the next tick.
		weatherQNH.Refresh()
	}
	// Feed lifecycle (ONB-5): the admin API joins/leaves multicast groups live
	// via the feed manager, and forgets a deleted feed's health (adapter below).
	feedLife := feedLifecycle{mgr: feedManager, registry: feedRegistry}
	// Per-feed source-credential sealing (ORCH-2c 3a, ADR 0012 §6). Wired only when
	// the shared cipher above is available; otherwise the secret routes stay disabled
	// (503) rather than storing credentials unencrypted.
	var secretSvc adminapi.SecretService
	if aeroCipher != nil {
		secretSvc = orchestrator.NewSecretSealer(store.NewSecretRepo(dbPool), aeroCipher)
		logger.Info("per-feed source-credential encryption enabled (ORCH-2c)",
			slog.String("path", "/api/admin/feeds/{id}/secrets"))
	}
	feedRepo := store.NewFeedRepo(dbPool).WithMulticastPool(cfg.feedPool())
	adminAPI := adminapi.New(viewRepo, subRepo, feedRepo, store.NewTenantRepo(dbPool),
		store.NewUserRepo(dbPool), store.NewCredentialRepo(dbPool), featSvc, feedRegistry, feedLife, aeroLife, secretSvc, logger, rescope).
		WithAeroCache(aeroCache).                                  // AERO-1: expose OpenAIP cache freshness on the status route
		WithGlobalOpenAIP(globalAero).                             // AERO-2: platform-wide OpenAIP key + fetch-all
		WithAeroChanges(aeroCache).                                // AERO-3: per-tenant change-impact of the last refresh
		WithAirspaceLister(aeroAirspaceLister{reg: aeroRegistry}). // ASD-014: AoR editor airspace picker
		WithViewProfiles(store.NewViewProfileRepo(dbPool))         // VP-2 (ADR 0023): per-user view profiles
	// AP7: pausing an access or tenant revokes its live sessions immediately. Only
	// in builtin mode (registry present); the counting adapter feeds the metric.
	if sessionRepo != nil {
		adminAPI = adminAPI.WithSessionRevoker(countingRevoker{repo: sessionRepo})
	}
	mux.Handle("/api/admin/", tenantMW(requireAdmin(adminAPI)))

	// Role-agnostic identity probe for the ASD map (any authenticated principal,
	// not just admins): the map reads it to decide between its login screen and
	// the live picture. Behind the tenant middleware (sets the Identity) but NOT
	// requireAdmin — a plain tenant user must be able to resolve its own session.
	// impReadMW: while impersonating, the probe reports the TARGET tenant's
	// features/sensor classes/FL band/ICAO (plus impersonated_tenant_id), so the
	// map renders exactly what that tenant's users see (ADR 0008 Nachtrag). The
	// identity fields (subject/role/tenant_id) stay the admin's own.
	mux.Handle("GET /api/whoami", tenantMW(impReadMW(adminAPI.WhoamiHandler())))

	// VP-2 (ADR 0023): per-user view profiles — the acting user's own named ASD
	// display presets. Behind tenantMW (any authenticated user) + pwGate (locked
	// while a password change is pending), NOT the admin gate: a profile is
	// strictly the caller's own (the handler reads the user id from the session,
	// never the request). Mounted at both the collection path and its subtree.
	viewProfiles := tenantMW(pwGate(adminAPI.ViewProfilesHandler()))
	mux.Handle("/api/view-profiles", viewProfiles)
	mux.Handle("/api/view-profiles/", viewProfiles)

	// Manual flight-plan correlation (ADR 0024, #245 Teil B): the FIRST tenant-user
	// feed-WRITE action. Behind tenantMW (authenticated) + pwGate; the handler's own
	// gate enforces "subscribed to this feed, not under read-only impersonation"
	// (ADR 0024 §E3). The command reaches the feed's Firefly over the host-loopback
	// back-channel; an empty WAYFINDER_FIREFLY_COMMAND_TOKEN disables the endpoint
	// (503), so the feature is opt-in per deployment (ADR 0024 §E2).
	corrClient := fireflycmd.New(&http.Client{Timeout: 15 * time.Second}, fireflycmd.HostLoopbackAddresser{}, cfg.FireflyCommandToken)
	corrSvc := correlationapi.New(corrClient, subRepo, cfg.FireflyCommandToken != "", logger)
	mux.Handle("POST /api/correlation", tenantMW(pwGate(corrSvc.SetHandler())))
	mux.Handle("DELETE /api/correlation/{feedID}/{trackNumber}", tenantMW(pwGate(corrSvc.ClearHandler())))
	if cfg.FireflyCommandToken != "" {
		logger.Info("manual correlation command channel enabled (ADR 0024)",
			slog.String("path", "/api/correlation"))
	}

	// Cross-tenant read-only impersonation (ADR 0008, WF2-34): mint and clear
	// the grant cookie. The more-specific method+path patterns take precedence
	// over the /api/admin/ subtree. Only wired when a signing key is configured;
	// otherwise impersonation stays disabled platform-wide (fail-closed),
	// matching the /ws read path.
	if len(cfg.SessionKey) > 0 {
		impAudit := logger.With(slog.String("component", "audit"))
		impCfg := impersonationCookieConfig{
			key:    cfg.SessionKey,
			ttl:    cfg.ImpersonationTTL,
			secure: cfg.TLSCertFile != "" && cfg.TLSKeyFile != "",
		}
		impChecker := tenantExistsChecker{repo: store.NewTenantRepo(dbPool)}
		mux.Handle("GET /api/admin/impersonation", tenantMW(requireAdmin(impersonationStatusHandler(impChecker, impCfg))))
		mux.Handle("POST /api/admin/impersonation", tenantMW(requireAdmin(startImpersonationHandler(impChecker, impCfg, impAudit))))
		mux.Handle("DELETE /api/admin/impersonation", tenantMW(requireAdmin(stopImpersonationHandler(impCfg, impAudit))))
		logger.Info("impersonation enabled (ADR 0008)",
			slog.String("path", "/api/admin/impersonation"), slog.Duration("ttl", cfg.ImpersonationTTL))
	}

	// The per-tenant middleware (on /ws) is the browser-edge gate; the whole mux
	// is served directly (ADR 0014).
	var handler http.Handler = mux

	go func() {
		addr := ":8081"
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			logger.Info("starting websocket server (TLS)", slog.String("addr", addr))
			if err := http.ListenAndServeTLS(addr, cfg.TLSCertFile, cfg.TLSKeyFile, handler); err != nil && err != http.ErrServerClosed {
				logger.Error("websocket server error", slog.String("error", err.Error()))
				cancel()
			}
			return
		}

		logger.Info("starting websocket server", slog.String("addr", addr))
		if err := http.ListenAndServe(addr, handler); err != nil && err != http.ErrServerClosed {
			logger.Error("websocket server error", slog.String("error", err.Error()))
			cancel()
		}
	}()

	// Wait for shutdown signal.
	go func() {
		sig := <-sigChan
		logger.Info("signal received", slog.String("signal", sig.String()))
		cancel()
	}()

	// Wait for goroutines to finish.
	wg.Wait()

	logger.Info("shutdown complete")
}

// Config holds runtime configuration.
type Config struct {
	MulticastGroup string
	MulticastPort  int
	// FeedID is the catalogue id of the single ENV-configured fallback feed used
	// only when the DB catalogue is empty (WAYFINDER_FEED_ID, default 0 =
	// unassigned). Stamped onto every track for the scoped fan-out (WF2-20/21).
	// The DB-driven multi-feed receiver supersedes this single value (WF2-20.2).
	FeedID       int64
	ProbePort    int
	MapCenterLat float64
	MapCenterLon float64
	MapZoom      float64
	MapStyleURL  string
	// MapTheme selects the built-in base map theme when no explicit
	// MapStyleURL is configured: "dark" (Radar Dark Mode, the controller
	// default) or "osm" (the bright OpenStreetMap raster). `WAYFINDER_MAP_THEME`,
	// default "dark". An explicit MapStyleURL always overrides the theme.
	MapTheme       string
	AllowedOrigins []string
	TLSCertFile    string
	TLSKeyFile     string
	LogLevel       slog.Level
	// FeedStaleTimeout is how long without a CAT065 heartbeat before the feed
	// is considered stale (Firefly ADR 0018). `WAYFINDER_FEED_STALE_TIMEOUT`
	// in seconds, default 3 s (~3× the 1 s heartbeat period).
	FeedStaleTimeout time.Duration

	// OpenAIP aeronautical layers (ASD-003, ADR 0004). The feature is enabled
	// only when an API key is configured; otherwise the map shows no overlays.
	OpenAIPAPIKey  string // WAYFINDER_OPENAIP_API_KEY (secret)
	OpenAIPBaseURL string // WAYFINDER_OPENAIP_BASE_URL (optional override)
	// OpenAIPRefresh is DEPRECATED and ignored since AERO-1 (ADR 0018): OpenAIP is
	// fetched once/on-demand and persisted, not on a ticker. Still parsed so a set
	// WAYFINDER_OPENAIP_REFRESH does not break startup; a set value is warned about.
	OpenAIPRefreshDeprecated bool
	OpenAIPRadiusKM          float64 // WAYFINDER_OPENAIP_RADIUS_KM (default 250)

	// DWD weather-radar overlay (WX-A, ADR 0016). Connected-by-default (ADR 0017):
	// the WMS URL defaults to the public DWD GeoServer, so the overlay is ON by
	// default; disable it with WAYFINDER_DWD_RADAR_ENABLED=false (best-effort — an
	// unreachable source just yields transparent tiles, never an error).
	DWDRadarEnabled bool          // WAYFINDER_DWD_RADAR_ENABLED (default true; false = opt-out)
	DWDWMSURL       string        // WAYFINDER_DWD_WMS_URL (DWD GeoServer WMS base, default maps.dwd.de)
	DWDRadarLayer   string        // WAYFINDER_DWD_RADAR_LAYER (default dwd:Niederschlagsradar)
	DWDRadarStyle   string        // WAYFINDER_DWD_RADAR_STYLE (WMS style; empty = layer default; #189 echo-only)
	DWDRefresh      time.Duration // WAYFINDER_DWD_REFRESH (radar tile cache TTL, default 5m)

	// QNH infobox (WX-B, ADR 0016; per-tenant CBD-3, ADR 0017): best-effort NOAA/AWC
	// METAR poller. QNH is not in the CAT062 contract and not in open DWD data; the
	// open source is NOAA METAR (altim field, hPa). Connected-by-default: the source
	// is ON unless WAYFINDER_QNH_ENABLED=false. The poll set is the union of the
	// tenants' aerodromes (view_configs.qnh_icao, set in the Admin UI); MetarStations
	// is a deprecated global fallback.
	QNHEnabled     bool          // WAYFINDER_QNH_ENABLED (default true; false = opt-out of the NOAA source)
	MetarURL       string        // WAYFINDER_METAR_URL (NOAA AWC METAR data API, default aviationweather.gov)
	MetarStations  []string      // WAYFINDER_METAR_STATIONS (deprecated global fallback; per-tenant qnh_icao preferred)
	MetarUserAgent string        // WAYFINDER_METAR_USER_AGENT (required distinctive UA; default Wayfinder-ASD/1.0)
	QNHRefresh     time.Duration // WAYFINDER_QNH_REFRESH (METAR poll interval, default 15m)

	// DWD weather-warnings overlay (WX-C, ADR 0016). Connected-by-default
	// (ADR 0017): the WFS URL defaults to the public DWD GeoServer, so the overlay
	// is ON by default; disable it with WAYFINDER_DWD_WARN_ENABLED=false.
	DWDWarnEnabled bool          // WAYFINDER_DWD_WARN_ENABLED (default true; false = opt-out)
	DWDWarnURL     string        // WAYFINDER_DWD_WARN_URL (DWD GeoServer WFS/OWS base, default maps.dwd.de)
	DWDWarnLayer   string        // WAYFINDER_DWD_WARN_LAYER (default dwd:Warnungen_Gemeinden_vereinigt)
	DWDWarnRefresh time.Duration // WAYFINDER_DWD_WARN_REFRESH (default 5m)

	// SecretKey is the deployment-managed AES-256 key (32 bytes, base64) that seals
	// per-feed source credentials at rest (ORCH-2c, ADR 0012 §6). Unset disables the
	// write-only secret admin routes (they return 503); the same key is configured
	// on the orchestrator to decrypt at launch. WAYFINDER_SECRET_KEY.
	SecretKey []byte

	// FireflyCommandToken is the deployment-wide bearer token for the manual
	// flight-plan correlation command back-channel (ADR 0024 §E2, #245 Teil B):
	// the server sends it, and the orchestrator injects the same value into every
	// Firefly as FIREFLY_WS_TOKEN. Unset disables the /api/correlation endpoint (it
	// returns 503). WAYFINDER_FIREFLY_COMMAND_TOKEN.
	FireflyCommandToken string

	// Multicast endpoint pool for auto-allocation (ORCH-4, ADR 0012). When an admin
	// creates a feed without a group/port, the server assigns the next free group
	// from this /24 (one group per feed, fixed port) — collision-free via the
	// feeds_endpoint_unique constraint. WAYFINDER_FEED_GROUP_BASE (default
	// 239.255.0), WAYFINDER_FEED_PORT (8600), WAYFINDER_FEED_OCTET_MIN/MAX (1/254).
	FeedGroupBase24 string
	FeedPort        int
	FeedOctetMin    int
	FeedOctetMax    int

	// Coverage rings overlay (Paket 6, ASD-012 extension).
	// Populated from WAYFINDER_COVERAGE_SENSOR_N_* env-vars.
	CoverageSensors   []coverage.SensorConfig // WAYFINDER_COVERAGE_SENSOR_N_{LAT,LON,...}
	CoverageRingColor string                  // WAYFINDER_COVERAGE_RING_COLOR, default #5B8DEF

	// Multi-tenancy (Wayfinder 2.0, WF2-12, ADR 0005/0006/0014). Multi-tenant is
	// the only mode: DBURL is mandatory and the start fails without it.
	DBURL          string        // WAYFINDER_DB_URL (PostgreSQL DSN; required)
	AuthMode       auth.Mode     // WAYFINDER_AUTH_MODE (proxy|builtin, default builtin)
	SessionKey     []byte        // WAYFINDER_SESSION_KEY (ModeBuiltin HMAC key)
	SessionCookie  string        // WAYFINDER_SESSION_COOKIE (default "wf_session")
	SessionTTL     time.Duration // WAYFINDER_SESSION_TTL (Go duration, sliding idle window, default 12h)
	SessionMaxLife time.Duration // WAYFINDER_SESSION_MAX_LIFETIME (absolute cap since first login; 0/unset = disabled, default off)
	// SessionLimitDefault is the per-access concurrent-session cap applied when an
	// access has no override (AP7); 0/unset = unlimited (opt-in, default off).
	SessionLimitDefault int // WAYFINDER_SESSION_LIMIT_DEFAULT
	// SessionLimitPolicy decides what happens at the limit: reject (default) or
	// evict_oldest (WAYFINDER_SESSION_LIMIT_POLICY).
	SessionLimitPolicy store.SessionLimitPolicy
	ImpersonationTTL   time.Duration // WAYFINDER_IMPERSONATION_TTL (read-only impersonation grant lifetime, default 30m; ADR 0008)
	OIDCIssuer         string        // WAYFINDER_OIDC_ISSUER (ModeProxy)
	OIDCAudience       string        // WAYFINDER_OIDC_AUDIENCE (ModeProxy)
}

// authConfig projects the runtime Config onto the auth package's Config. sessions
// is the server-side session registry (AP7); in builtin mode the authenticator
// resolves session-id cookies against it. Pass nil to keep the stateless
// behaviour (e.g. proxy mode, which mints no local cookies).
func (c Config) authConfig(sessions auth.SessionResolver) auth.Config {
	return auth.Config{
		Mode:         c.AuthMode,
		CookieName:   c.SessionCookie,
		SessionKey:   c.SessionKey,
		Sessions:     sessions,
		OIDCIssuer:   c.OIDCIssuer,
		OIDCAudience: c.OIDCAudience,
	}
}

// defaultMapStyle is a minimal MapLibre style using OpenStreetMap raster
// tiles. It needs no API key, which keeps the demo self-contained. The "glyphs"
// endpoint is served BY WAYFINDER ITSELF (/glyphs/…, webui.GlyphsHandler,
// embedded Roboto Mono PBFs) — a symbol layer with a text-field draws nothing
// without a font source, and self-hosting keeps the scope font off any runtime
// CDN (air-gap; ADR 0015). The raster tiles are still external; a fully
// air-gapped deployment self-hosts those too via WAYFINDER_MAP_STYLE_URL.
const defaultMapStyle = `{
	"version": 8,
	"glyphs": "/glyphs/{fontstack}/{range}.pbf",
	"sources": {
		"osm": {
			"type": "raster",
			"tiles": ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
			"tileSize": 256,
			"attribution": "© OpenStreetMap contributors"
		}
	},
	"layers": [{"id": "osm", "type": "raster", "source": "osm"}]
}`

// darkMapStyle is the "Radar Dark Mode" base: a low-contrast CARTO dark raster
// (no labels), dimmed (raster-opacity 0.4) over a near-black background so the
// scope backdrop shows through while coastlines/borders remain as faint
// geographic context. Like OSM it needs no API key, which keeps the demo
// self-contained. The dimmed, label-free base lets the track symbols and
// aeronautical overlays dominate, the way a controller's radar scope does.
// Background is the near-black --wf-background (#070b12) per ADR 0015
// Nachtrag-2 (design-template authoritative); ASD-003 Häppchen 3a. The raster
// stays (real geographic context is a deliberate product choice — the pure
// synthetic scope in the design export is a standalone-demo artefact).
const darkMapStyle = `{
	"version": 8,
	"glyphs": "/glyphs/{fontstack}/{range}.pbf",
	"sources": {
		"carto-dark": {
			"type": "raster",
			"tiles": ["https://basemaps.cartocdn.com/dark_nolabels/{z}/{x}/{y}.png"],
			"tileSize": 256,
			"attribution": "© OpenStreetMap contributors © CARTO"
		}
	},
	"layers": [
		{"id": "background", "type": "background", "paint": {"background-color": "#070b12"}},
		{"id": "carto-dark", "type": "raster", "source": "carto-dark", "paint": {"raster-opacity": 0.4}}
	]
}`

// mapThemeDark and mapThemeOSM are the recognised built-in theme names.
const (
	mapThemeDark = "dark"
	mapThemeOSM  = "osm"
)

// yamlFileConfig mirrors the structure of wayfinder.yaml. All fields are
// optional; absent fields leave the corresponding Config defaults untouched.
// Env-vars always take precedence (12-Factor: env > file > hardcoded default).
type yamlFileConfig struct {
	Map struct {
		CenterLat float64 `yaml:"center_lat"`
		CenterLon float64 `yaml:"center_lon"`
		Zoom      float64 `yaml:"zoom"`
	} `yaml:"map"`
	OpenAIP struct {
		RadiusKM float64 `yaml:"radius_km"`
	} `yaml:"openaip"`
}

// loadYAMLFile reads wayfinder.yaml from the given path and applies non-zero
// fields to cfg. Missing file or parse errors are treated as non-fatal — the
// caller falls back to hardcoded defaults and env-vars. The path is resolved
// relative to the working directory; an empty path disables YAML loading.
func loadYAMLFile(path string, cfg *Config, logger *slog.Logger) {
	if path == "" {
		return
	}
	data, err := os.ReadFile(path) //nolint:gosec // path comes from trusted env/default
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("wayfinder.yaml unreadable, using defaults", "path", path, "err", err)
		}
		return
	}
	var fc yamlFileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		logger.Warn("wayfinder.yaml parse error, using defaults", "path", path, "err", err)
		return
	}
	if fc.Map.CenterLat != 0 {
		cfg.MapCenterLat = fc.Map.CenterLat
	}
	if fc.Map.CenterLon != 0 {
		cfg.MapCenterLon = fc.Map.CenterLon
	}
	if fc.Map.Zoom != 0 {
		cfg.MapZoom = fc.Map.Zoom
	}
	if fc.OpenAIP.RadiusKM != 0 {
		cfg.OpenAIPRadiusKM = fc.OpenAIP.RadiusKM
	}
	logger.Info("loaded wayfinder.yaml", "path", path)
}

// loadConfig loads configuration from environment variables.
func loadConfig() Config {
	cfg := Config{
		MulticastGroup: os.Getenv("FIREFLY_CAT062_GROUP"),
		MulticastPort:  8600,
		ProbePort:      8080,
		// Default map center: Frankfurt am Main, matching Firefly's demo scenario.
		MapCenterLat:     50.0379,
		MapCenterLon:     8.5622,
		MapZoom:          8,
		MapStyleURL:      "",
		MapTheme:         mapThemeDark,
		LogLevel:         slog.LevelInfo,
		FeedStaleTimeout: 3 * time.Second,
		OpenAIPRadiusKM:  250,
		// DWD weather overlays are connected-by-default (ADR 0017): the public DWD
		// GeoServer URLs are the default, and the ENABLED flags default true, so the
		// radar + warnings overlays are ON out of the box (opt-out via
		// WAYFINDER_DWD_RADAR_ENABLED / _WARN_ENABLED = false).
		DWDRadarEnabled: true,
		DWDWMSURL:       "https://maps.dwd.de/geoserver/dwd/wms",
		DWDRadarLayer:   "dwd:Niederschlagsradar",
		DWDRefresh:      5 * time.Minute,
		// QNH (NOAA) is connected-by-default too (ADR 0017): the source is ON, the
		// poll set comes from the tenants' aerodromes. Opt out with WAYFINDER_QNH_ENABLED=false.
		QNHEnabled:       true,
		QNHRefresh:       15 * time.Minute,
		DWDWarnEnabled:   true,
		DWDWarnURL:       "https://maps.dwd.de/geoserver/dwd/ows",
		DWDWarnLayer:     "dwd:Warnungen_Gemeinden_vereinigt",
		DWDWarnRefresh:   5 * time.Minute,
		ImpersonationTTL: 30 * time.Minute,
	}

	if cfg.MulticastGroup == "" {
		cfg.MulticastGroup = "239.255.0.62"
	}

	// Load optional wayfinder.yaml before env-vars so that env-vars win.
	// WAYFINDER_CONFIG_FILE overrides the default path; set to "" to disable.
	yamlPath := os.Getenv("WAYFINDER_CONFIG_FILE")
	if yamlPath == "" {
		yamlPath = "wayfinder.yaml"
	}
	loadYAMLFile(yamlPath, &cfg, slog.Default())

	if portStr := os.Getenv("FIREFLY_CAT062_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.MulticastPort = port
		}
	}

	if v := os.Getenv("WAYFINDER_FEED_ID"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.FeedID = id
		}
	}

	if portStr := os.Getenv("WAYFINDER_PROBE_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.ProbePort = port
		}
	}

	if v := os.Getenv("WAYFINDER_MAP_CENTER_LAT"); v != "" {
		if lat, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MapCenterLat = lat
		}
	}

	if v := os.Getenv("WAYFINDER_MAP_CENTER_LON"); v != "" {
		if lon, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MapCenterLon = lon
		}
	}

	if v := os.Getenv("WAYFINDER_MAP_ZOOM"); v != "" {
		if zoom, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MapZoom = zoom
		}
	}

	cfg.MapStyleURL = os.Getenv("WAYFINDER_MAP_STYLE_URL")

	// Map theme: only the documented built-in names are accepted; anything else
	// falls back to the default (FR-CFG-002: invalid config falls back rather
	// than crashing).
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("WAYFINDER_MAP_THEME"))); v == mapThemeDark || v == mapThemeOSM {
		cfg.MapTheme = v
	}

	if v := os.Getenv("WAYFINDER_ALLOWED_ORIGINS"); v != "" {
		for _, origin := range strings.Split(v, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
			}
		}
	}

	cfg.TLSCertFile = os.Getenv("WAYFINDER_TLS_CERT")
	cfg.TLSKeyFile = os.Getenv("WAYFINDER_TLS_KEY")

	if v := os.Getenv("WAYFINDER_LOG_LEVEL"); v != "" {
		if level, err := parseLogLevel(v); err == nil {
			cfg.LogLevel = level
		}
	}

	if v := os.Getenv("WAYFINDER_FEED_STALE_TIMEOUT"); v != "" {
		if secs, err := strconv.ParseFloat(v, 64); err == nil && secs > 0 {
			cfg.FeedStaleTimeout = time.Duration(secs * float64(time.Second))
		}
	}

	cfg.OpenAIPAPIKey = os.Getenv("WAYFINDER_OPENAIP_API_KEY")
	cfg.OpenAIPBaseURL = os.Getenv("WAYFINDER_OPENAIP_BASE_URL")

	// Per-feed source-credential encryption key (ORCH-2c, ADR 0012 §6). Invalid or
	// unset leaves SecretKey nil — the secret admin routes then stay disabled (503)
	// rather than storing credentials unencrypted. Parse errors are surfaced at
	// wiring time (run) so a typo is loud, not silently insecure.
	cfg.SecretKey = parseSecretKey(os.Getenv("WAYFINDER_SECRET_KEY"))
	cfg.FireflyCommandToken = os.Getenv("WAYFINDER_FIREFLY_COMMAND_TOKEN")

	// Multicast endpoint pool (ORCH-4). Invalid/unset values fall back to the
	// store's DefaultMulticastPool via feedPool().
	cfg.FeedGroupBase24 = os.Getenv("WAYFINDER_FEED_GROUP_BASE")
	cfg.FeedPort = atoiDefault(os.Getenv("WAYFINDER_FEED_PORT"), 0)
	cfg.FeedOctetMin = atoiDefault(os.Getenv("WAYFINDER_FEED_OCTET_MIN"), -1)
	cfg.FeedOctetMax = atoiDefault(os.Getenv("WAYFINDER_FEED_OCTET_MAX"), -1)

	// WAYFINDER_OPENAIP_REFRESH is deprecated and ignored since AERO-1 (ADR 0018):
	// OpenAIP is fetched once/on-demand and persisted, not on a ticker. Record that
	// it was set so loadConfig's caller can warn (kept out of loadConfig, which has
	// no logger).
	cfg.OpenAIPRefreshDeprecated = strings.TrimSpace(os.Getenv("WAYFINDER_OPENAIP_REFRESH")) != ""

	if v := os.Getenv("WAYFINDER_OPENAIP_RADIUS_KM"); v != "" {
		if km, err := strconv.ParseFloat(v, 64); err == nil && km > 0 {
			cfg.OpenAIPRadiusKM = km
		}
	}

	// DWD weather-radar overlay (WX-A, ADR 0016). Connected-by-default (ADR 0017):
	// the WMS URL keeps its public-DWD default unless explicitly overridden with a
	// non-empty value; the overlay is disabled with WAYFINDER_DWD_RADAR_ENABLED=false.
	cfg.DWDRadarEnabled = envBool("WAYFINDER_DWD_RADAR_ENABLED", true)
	if v := strings.TrimSpace(os.Getenv("WAYFINDER_DWD_WMS_URL")); v != "" {
		cfg.DWDWMSURL = v
	}
	if v := strings.TrimSpace(os.Getenv("WAYFINDER_DWD_RADAR_LAYER")); v != "" {
		cfg.DWDRadarLayer = v
	}
	// #189: optional non-default WMS style (e.g. an echo-only rendering without
	// the measurement-domain fill / station range rings). Empty = layer default.
	cfg.DWDRadarStyle = strings.TrimSpace(os.Getenv("WAYFINDER_DWD_RADAR_STYLE"))
	if v := os.Getenv("WAYFINDER_DWD_REFRESH"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.DWDRefresh = d
		}
	}

	// QNH infobox (WX-B, ADR 0016; per-tenant CBD-3, ADR 0017). Connected-by-default:
	// the NOAA source keeps its public default unless explicitly disabled; the URL/UA
	// keep their client defaults unless overridden with a non-empty value.
	cfg.QNHEnabled = envBool("WAYFINDER_QNH_ENABLED", true)
	if v := strings.TrimSpace(os.Getenv("WAYFINDER_METAR_URL")); v != "" {
		cfg.MetarURL = v
	}
	if v := strings.TrimSpace(os.Getenv("WAYFINDER_METAR_USER_AGENT")); v != "" {
		cfg.MetarUserAgent = v
	}
	// Deprecated global fallback (per-tenant qnh_icao in the Admin UI is preferred).
	if v := os.Getenv("WAYFINDER_METAR_STATIONS"); v != "" {
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				cfg.MetarStations = append(cfg.MetarStations, s)
			}
		}
	}
	if v := os.Getenv("WAYFINDER_QNH_REFRESH"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.QNHRefresh = d
		}
	}

	// DWD weather-warnings overlay (WX-C, ADR 0016). Connected-by-default (ADR 0017):
	// the WFS URL keeps its public-DWD default unless explicitly overridden;
	// disabled with WAYFINDER_DWD_WARN_ENABLED=false.
	cfg.DWDWarnEnabled = envBool("WAYFINDER_DWD_WARN_ENABLED", true)
	if v := strings.TrimSpace(os.Getenv("WAYFINDER_DWD_WARN_URL")); v != "" {
		cfg.DWDWarnURL = v
	}
	if v := strings.TrimSpace(os.Getenv("WAYFINDER_DWD_WARN_LAYER")); v != "" {
		cfg.DWDWarnLayer = v
	}
	if v := os.Getenv("WAYFINDER_DWD_WARN_REFRESH"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.DWDWarnRefresh = d
		}
	}

	// Coverage rings: sensor positions and ranges from env-vars.
	cfg.CoverageSensors = coverage.ParseEnv(os.Getenv)
	cfg.CoverageRingColor = os.Getenv("WAYFINDER_COVERAGE_RING_COLOR")
	if cfg.CoverageRingColor == "" {
		cfg.CoverageRingColor = "#5B8DEF"
	}

	// Multi-tenancy (WF2-12, ADR 0014). WAYFINDER_DB_URL is mandatory; the start
	// fails without it (setupTenancy).
	cfg.DBURL = os.Getenv("WAYFINDER_DB_URL")
	cfg.AuthMode, _ = auth.ParseMode(os.Getenv("WAYFINDER_AUTH_MODE")) // invalid/empty → builtin
	if v := os.Getenv("WAYFINDER_SESSION_KEY"); v != "" {
		cfg.SessionKey = []byte(v)
	}
	cfg.SessionCookie = os.Getenv("WAYFINDER_SESSION_COOKIE")
	if v := os.Getenv("WAYFINDER_SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.SessionTTL = d
		}
	}
	// Absolute session maximum since first login (opt-in; 0/unset = disabled). For
	// a trial run set e.g. WAYFINDER_SESSION_MAX_LIFETIME=30m to watch it fire
	// without waiting out the sliding TTL.
	if v := os.Getenv("WAYFINDER_SESSION_MAX_LIFETIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.SessionMaxLife = d
		}
	}
	// Per-access concurrent-session limit (AP7): default cap for accesses without
	// an override. Unset/0/negative → unlimited (enforcement off, opt-in).
	if v := os.Getenv("WAYFINDER_SESSION_LIMIT_DEFAULT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SessionLimitDefault = n
		}
	}
	// Limit-overflow policy: reject (default) or evict_oldest. An unrecognised
	// value falls back to reject (enforced downstream by LoginConfig.policy()).
	if p := store.SessionLimitPolicy(strings.ToLower(strings.TrimSpace(os.Getenv("WAYFINDER_SESSION_LIMIT_POLICY")))); p.Valid() {
		cfg.SessionLimitPolicy = p
	}
	if v := os.Getenv("WAYFINDER_IMPERSONATION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.ImpersonationTTL = d
		}
	}
	cfg.OIDCIssuer = os.Getenv("WAYFINDER_OIDC_ISSUER")
	cfg.OIDCAudience = os.Getenv("WAYFINDER_OIDC_AUDIENCE")

	return cfg
}

// parseSecretKey decodes the deployment secret key (base64, 32 bytes) used to seal
// per-feed source credentials (ORCH-2c, ADR 0012 §6). An empty or malformed value
// yields nil — the secret routes then stay disabled (the wiring logs a set-but-
// invalid key loudly), so credentials are never stored unencrypted by accident.
func parseSecretKey(s string) []byte {
	if s == "" {
		return nil
	}
	key, err := secret.KeyFromBase64(s)
	if err != nil {
		return nil
	}
	return key
}

// atoiDefault parses s as an int, returning def for empty or malformed input
// (12-Factor leniency: a typo'd numeric env falls back, it does not abort).
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// envBool reads a boolean env var, returning def when unset or unparseable
// (FR-CFG-002: invalid config falls back to the default rather than crashing).
// Accepts the strconv.ParseBool spellings (1/0, t/f, true/false, …). Used for the
// connected-by-default opt-out flags (ADR 0017), which default true.
func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// feedPool builds the multicast endpoint pool from config, overriding only the
// fields that are explicitly set; the rest keep store.DefaultMulticastPool
// (ORCH-4). WithMulticastPool validates the result and falls back wholesale to the
// default if the combination is unusable.
func (c Config) feedPool() store.MulticastPool {
	p := store.DefaultMulticastPool
	if c.FeedGroupBase24 != "" {
		p.Base24 = c.FeedGroupBase24
	}
	if c.FeedPort > 0 {
		p.Port = c.FeedPort
	}
	if c.FeedOctetMin >= 0 {
		p.OctetMin = c.FeedOctetMin
	}
	if c.FeedOctetMax >= 0 {
		p.OctetMax = c.FeedOctetMax
	}
	return p
}

// setupTenancy wires up multi-tenancy: it opens the database, applies
// migrations, builds the configured authenticator and returns the
// tenant-context middleware. WAYFINDER_DB_URL is mandatory (ADR 0014) — a
// missing DSN fails the start rather than degrading to an unauthenticated,
// unscoped ASD. The returned pool must be closed by the caller.
func setupTenancy(ctx context.Context, cfg Config, logger *slog.Logger) (func(http.Handler) http.Handler, *pgxpool.Pool, error) {
	if cfg.DBURL == "" {
		return nil, nil, fmt.Errorf("WAYFINDER_DB_URL is required: multi-tenant is the only mode (ADR 0014); set it to a PostgreSQL DSN")
	}

	pool, err := store.Open(ctx, cfg.DBURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}
	if err := store.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("migrate schema: %w", err)
	}

	// Registry-backed authentication (AP7, ADR 0009 §5): in builtin mode the
	// authenticator resolves session-id cookies against the server-side registry
	// so sessions are revocable, while still accepting legacy stateless cookies
	// during the rollout (sanfte Übernahme). Building it here (not only in the
	// login wiring) is what makes every authenticated request registry-checked.
	var sessions auth.SessionResolver
	if cfg.AuthMode == auth.ModeBuiltin {
		sessions = store.NewSessionRepo(pool)
	}
	authenticator, err := auth.NewAuthenticator(ctx, cfg.authConfig(sessions))
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("build authenticator: %w", err)
	}

	// Zero-touch onboarding (ONB-1, ADR 0011): in builtin mode, provision a default
	// tenant + admin on first boot so the deployment is usable from the browser
	// without any terminal step. Idempotent — a no-op once an admin exists.
	if cfg.AuthMode == auth.ModeBuiltin {
		var seedLog strings.Builder
		if err := autoSeedDefaultAdmin(ctx, pool, &seedLog); err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("auto-seed default admin: %w", err)
		}
		if s := strings.TrimSpace(seedLog.String()); s != "" {
			logger.Info("auto-seed", slog.String("detail", s))
		}
	}

	logger.Info("multi-tenancy enabled", slog.String("auth_mode", string(cfg.AuthMode)))
	return tenant.Middleware(authenticator, store.NewUserRepo(pool), logger), pool, nil
}

// feedLister and viewGetter are the slices of *store.SubscriptionRepo /
// *store.ViewConfigRepo the scope resolver needs (kept small so the resolver is
// unit-testable with fakes).
type feedLister interface {
	ListFeedIDsByTenant(ctx context.Context, tenantID int64) ([]int64, error)
}
type viewGetter interface {
	GetEffective(ctx context.Context, tenantID, userID int64) (store.ViewConfig, error)
}

// resolveScope resolves a tenant/user to the broadcast scope that filters their
// stream: the feeds the tenant subscribes to (WF2-21.1) plus the effective view
// filter (WF2-21.2). It is shared by the /ws connect resolver and the live
// re-scope (WF2-33), so a freshly connected client and a re-scoped one always end
// up with an identical scope. The feed ids and view filter are returned alongside
// the scope for the connect-time audit record (those fields are unexported on
// Scope).
func resolveScope(ctx context.Context, subs feedLister, views viewGetter, tenantID, userID int64) (*broadcast.Scope, []int64, *broadcast.ViewFilter, error) {
	feedIDs, err := subs.ListFeedIDsByTenant(ctx, tenantID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve subscriptions: %w", err)
	}
	view, err := resolveViewFilter(ctx, views, tenantID, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve view: %w", err)
	}
	scope := broadcast.NewScopeWithView(feedIDs, view)
	scope.TenantID = tenantID // for per-tenant metrics (WF2-23.2)
	scope.UserID = userID     // lets the live re-scope re-resolve this user (WF2-33)
	return scope, feedIDs, view, nil
}

// newScopeResolver builds the /ws scope resolver (WF2-21): it reads the tenant
// Identity placed in the request context by the middleware, resolves the scope,
// and emits a structured audit record (WF2-23.1). Fail-closed: a request without
// an identity, or whose subscription/view lookup fails, is rejected.
//
// Cross-tenant read-only impersonation (ADR 0008): when impersonation is enabled
// (tenants != nil and a signing key is configured) and the request carries a
// valid grant from an admin, the read scope AND view are resolved against the
// TARGET tenant instead of the caller's own. The authenticated Identity is
// untouched; the resulting scope is detached from the target's accounting
// (TenantID zeroed) so it is excluded from per-tenant metrics and live re-scope.
// A valid grant from a non-admin, or one naming a missing tenant, is a loud
// failure (handshake reject + audit); an absent/invalid/expired grant falls back
// to the normal, byte-identical path (so the WF2-22 isolation tests stay valid).
func newScopeResolver(subs feedLister, views viewGetter, tenants impersonation.TenantChecker, key []byte, logger *slog.Logger) ws.ScopeResolver {
	audit := logger.With(slog.String("component", "audit"))
	impersonationOn := tenants != nil && len(key) > 0
	return func(r *http.Request) (*broadcast.Scope, error) {
		id, ok := tenant.FromContext(r.Context())
		if !ok {
			return nil, fmt.Errorf("scoped /ws requires a tenant identity")
		}

		var decision impersonation.Decision
		if impersonationOn {
			d, err := impersonation.Resolve(r.Context(), impersonationGrantCookie(r), id, key, tenants)
			if err != nil {
				logImpersonationDenied(audit, r, id, err)
				return nil, err
			}
			decision = d
		}

		// #208 (ADR 0022): a platform admin has no own air picture. Admins are
		// tenant-less by database invariant (migration 00007: admin XOR tenant),
		// so the only legitimate ASD read for an admin is through an ACTIVE
		// read-only impersonation grant (guest mode, ADR 0008). Reject the
		// handshake fail-closed otherwise — including when impersonation is
		// disabled platform-wide (no signing key) and when a grant has expired;
		// the earlier "empty own picture" fallback is deliberately gone.
		if id.Role == store.RoleAdmin && !decision.Active {
			audit.Warn("ws scope denied: admin without impersonation grant",
				slog.String("event", "ws_admin_denied"),
				slog.Int64("user_id", id.UserID),
				slog.String("subject", id.Subject),
				slog.String("remote", r.RemoteAddr))
			return nil, fmt.Errorf("admin has no own ASD scope: start read-only guest mode (ADR 0022)")
		}

		readTenant := id.TenantID
		if decision.Active {
			readTenant = decision.TargetTenantID
		}
		scope, feedIDs, view, err := resolveScope(r.Context(), subs, views, readTenant, id.UserID)
		if err != nil {
			return nil, err
		}
		if decision.Active {
			// Detach the impersonation session from the target tenant: zeroing
			// TenantID excludes it from per-tenant billing/SLA metrics (ADR 0008 §6)
			// and from live re-scope (§4 snapshot-v1) — both key on scope.TenantID —
			// while the target's feeds+view still drive what the operator reads.
			scope.TenantID = 0
			impersonationSessions.Add(1)
		}
		logScopeAudit(audit, r, id, feedIDs, view, decision)
		return scope, nil
	}
}

// rescopeTenant re-resolves the scope of every connected client of a tenant and
// applies it live (WF2-33), so a view or subscription change takes effect without
// a reconnect. The DB resolution runs here, off the broadcaster's Run goroutine;
// only the in-memory swap is handed to Run via ApplyScopes. Feeds are per-tenant
// (resolved once); the effective view may differ per user (resolved per distinct
// user). On any resolution error the whole batch is skipped — clients keep their
// current scope (safe: a reconnect or the next change reconciles them). A no-op
// when no client of the tenant is connected.
func rescopeTenant(ctx context.Context, b *broadcast.Broadcaster, subs feedLister, views viewGetter, logger *slog.Logger, tenantID int64) {
	refs := b.ClientsForTenant(tenantID)
	if len(refs) == 0 {
		return
	}
	byUser := make(map[int64]*broadcast.Scope)
	out := make(map[*broadcast.Client]*broadcast.Scope, len(refs))
	for _, ref := range refs {
		scope, ok := byUser[ref.UserID]
		if !ok {
			s, _, _, err := resolveScope(ctx, subs, views, tenantID, ref.UserID)
			if err != nil {
				logger.Error("live re-scope: resolve failed, keeping current scopes",
					slog.Int64("tenant_id", tenantID), slog.Int64("user_id", ref.UserID),
					slog.String("error", err.Error()))
				return
			}
			byUser[ref.UserID] = s
			scope = s
		}
		out[ref.Client] = scope
	}
	if err := b.ApplyScopes(ctx, out); err != nil {
		logger.Warn("live re-scope: apply interrupted",
			slog.Int64("tenant_id", tenantID), slog.String("error", err.Error()))
	}
}

// logScopeAudit records which tenant/user was authorized for which scope at /ws
// connect (WF2-23.1, NFR-SEC-003 audit trail). High-cardinality identity
// (user_id, subject) belongs here in the structured log — shipped to an external
// sink (12-factor) — never as a metric label.
func logScopeAudit(audit *slog.Logger, r *http.Request, id tenant.Identity, feedIDs []int64, view *broadcast.ViewFilter, imp impersonation.Decision) {
	attrs := []any{
		slog.String("event", "ws_connect"),
		slog.Int64("tenant_id", id.TenantID),
		slog.Int64("user_id", id.UserID),
		slog.String("subject", id.Subject),
		slog.String("role", string(id.Role)),
		slog.Any("feeds", feedIDs),
		slog.String("remote", r.RemoteAddr),
	}
	if imp.Active {
		// The admin actor is already logged above (user_id/subject); record
		// which tenant they viewed read-only (ADR 0008 §7).
		attrs = append(attrs,
			slog.Bool("impersonation", true),
			slog.Int64("impersonated_tenant_id", imp.TargetTenantID))
	}
	if view != nil {
		if view.AOI != nil {
			attrs = append(attrs, slog.Group("aoi",
				slog.Float64("min_lat", view.AOI.MinLat), slog.Float64("min_lon", view.AOI.MinLon),
				slog.Float64("max_lat", view.AOI.MaxLat), slog.Float64("max_lon", view.AOI.MaxLon)))
		}
		if view.FLMinFt != nil {
			attrs = append(attrs, slog.Float64("fl_min_ft", *view.FLMinFt))
		}
		if view.FLMaxFt != nil {
			attrs = append(attrs, slog.Float64("fl_max_ft", *view.FLMaxFt))
		}
	}
	audit.Info("ws scope authorized", attrs...)
}

// resolveViewFilter maps the tenant/user's effective view config (if any) to a
// broadcast.ViewFilter. No config — or a config with neither AOI nor FL bounds —
// yields nil (no view restriction within the allowed feeds). Flight levels are
// stored in FL (hundreds of feet) and converted to feet for comparison.
func resolveViewFilter(ctx context.Context, views viewGetter, tenantID, userID int64) (*broadcast.ViewFilter, error) {
	vc, err := views.GetEffective(ctx, tenantID, userID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	vf := &broadcast.ViewFilter{}
	if vc.AOI != nil {
		vf.AOI = &broadcast.BBox{MinLat: vc.AOI.MinLat, MinLon: vc.AOI.MinLon, MaxLat: vc.AOI.MaxLat, MaxLon: vc.AOI.MaxLon}
	}
	if vc.FLMin != nil {
		ft := float64(*vc.FLMin) * 100
		vf.FLMinFt = &ft
	}
	if vc.FLMax != nil {
		ft := float64(*vc.FLMax) * 100
		vf.FLMaxFt = &ft
	}
	if vf.AOI == nil && vf.FLMinFt == nil && vf.FLMaxFt == nil {
		return nil, nil // config exists but imposes no restriction → fast path
	}
	return vf, nil
}

// parseLogLevel parses the documented slog level names ("debug", "info",
// "warn", "error", case-insensitive). Invalid values are rejected so callers
// can fall back to the default (FR-CFG-002: invalid config falls back to
// defaults rather than crashing).
func parseLogLevel(v string) (slog.Level, error) {
	var level slog.Level
	err := level.UnmarshalText([]byte(v))
	return level, err
}

// mapConfigHandler serves the map center/zoom/style/theme as JSON for the
// frontend. The style is chosen as follows: an explicit WAYFINDER_MAP_STYLE_URL
// always wins; otherwise the built-in theme decides ("osm" → bright OSM raster,
// "dark" → Radar Dark Mode). The reported `theme` lets the frontend pick a
// matching foreground palette (light labels on the dark base, dark on OSM).
func mapConfigHandler(cfg Config) http.HandlerFunc {
	var styleValue any
	theme := cfg.MapTheme
	switch {
	case cfg.MapStyleURL != "":
		// A custom style is opaque to us; report the configured theme so the
		// operator can still steer the palette via WAYFINDER_MAP_THEME.
		styleValue = cfg.MapStyleURL
	case cfg.MapTheme == mapThemeOSM:
		styleValue = json.RawMessage(defaultMapStyle)
	default:
		styleValue = json.RawMessage(darkMapStyle)
		theme = mapThemeDark
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"center_lat":            cfg.MapCenterLat,
			"center_lon":            cfg.MapCenterLon,
			"zoom":                  cfg.MapZoom,
			"style":                 styleValue,
			"theme":                 theme,
			"coverage_ring_color":   cfg.CoverageRingColor,
			"coverage_sensor_count": len(cfg.CoverageSensors),
			// WX-A / ADR 0017: whether the DWD radar overlay is active (connected-by-
			// default; off only when explicitly disabled). Gates the sidebar switch.
			"weather_radar_available": cfg.DWDRadarEnabled && cfg.DWDWMSURL != "",
			// WX-C / ADR 0017: same for the DWD warnings overlay.
			"weather_warnings_available": cfg.DWDWarnEnabled && cfg.DWDWarnURL != "",
			// WX-B / CBD-3 / ADR 0017: whether the QNH (NOAA) source is on. The header
			// infobox additionally needs the tenant's qnh entitlement and a configured
			// aerodrome (view_configs.qnh_icao); this flag only reports the source.
			"qnh_available": cfg.QNHEnabled,
			// #245 Teil B / ADR 0024: whether manual flight-plan correlation is enabled
			// (a command token is configured). Gates the correlation controls in the
			// detail panel; the server enforces the feature edge independently (503
			// when the token is unset), so this is a UI convenience, not the guard.
			"correlation_available": cfg.FireflyCommandToken != "",
		})
	}
}

// coverageRingsHandler serves the sensor coverage rings as a static GeoJSON
// FeatureCollection. The GeoJSON is computed once at startup from the
// configured sensors; an empty FeatureCollection is returned when no sensors
// are configured so the frontend can always fetch unconditionally.
func coverageRingsHandler(cfg Config) http.HandlerFunc {
	body, err := coverage.RingsGeoJSON(cfg.CoverageSensors, cfg.CoverageRingColor)
	if err != nil {
		// RingsGeoJSON only fails when json.Marshal fails — effectively never.
		// Fall back to a minimal empty collection rather than crashing.
		body = []byte(`{"type":"FeatureCollection","features":[]}`)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(body)
	}
}

// startProbeServer starts an HTTP server for health, readiness and metrics.
func startProbeServer(logger *slog.Logger, blockCount, trackCount, tracksCurrent, heartbeatCount *atomic.Int64, broadcaster *broadcast.Broadcaster, decodeErrors func() int64, feedRegistry *health.Registry, aeroService *aeronautical.Service, aeroCounts func() (success, failure int64), weatherRadar *weathertiles.Service, weatherQNH *weather.Service, weatherWarn *weatherwarnings.Service, lastError *atomic.Pointer[string], featSvc *feature.Service, sessionRepo *store.SessionRepo) {
	mux := http.NewServeMux()

	// /health — liveness check (always ready unless startup failed).
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// /ready — readiness check. Ready once we have clients or blocks received,
	// and — if the CAT065 heartbeat has ever been seen — only while the feed is
	// not stale (Firefly ADR 0018). A feed that never heartbeats (CAT062-only)
	// falls back to the traffic-based check.
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		count := blockCount.Load()
		clients := broadcaster.ClientCount()
		status := feedRegistry.Status(time.Now())
		healthy := !status.EverSeen || !status.Stale
		if (count > 0 || clients > 0) && healthy {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ready","blocks":` + strconv.FormatInt(count, 10) + `,"clients":` + strconv.Itoa(clients) + `,"feed_stale":` + strconv.FormatBool(status.Stale) + `}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"not_ready","blocks":` + strconv.FormatInt(count, 10) + `,"clients":` + strconv.Itoa(clients) + `,"feed_stale":` + strconv.FormatBool(status.Stale) + `}`))
	})

	// /metrics — Prometheus text exposition (REQ NFR-OBS-002): track
	// throughput, decode errors, WebSocket client counts/drops, and feed health.
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		feedStale := int64(0)
		if feedRegistry.Status(time.Now()).Stale {
			feedStale = 1
		}
		aeroSuccess, aeroFailure := aeroCounts()
		mset := []metrics.Metric{
			metrics.Counter("wayfinder_cat062_blocks_received_total", "Total number of CAT062 data blocks received via multicast.", blockCount.Load()),
			metrics.Counter("wayfinder_cat062_tracks_received_total", "Total number of track records received across all CAT062 blocks.", trackCount.Load()),
			metrics.Counter("wayfinder_cat062_decode_errors_total", "Total number of CAT062 data blocks that failed to decode.", decodeErrors()),
			metrics.Gauge("wayfinder_tracks_current", "Number of tracks in the most recently received CAT062 block.", tracksCurrent.Load()),
			metrics.Gauge("wayfinder_ws_clients_connected", "Number of currently connected WebSocket clients.", int64(broadcaster.ClientCount())),
			metrics.Counter("wayfinder_ws_clients_evicted_total", "Total number of WebSocket clients evicted due to a full send channel.", broadcaster.EvictedCount()),
			metrics.Counter("wayfinder_impersonation_sessions_total", "Total admin read-only impersonation /ws sessions started (ADR 0008). Excluded from the per-tenant series.", impersonationSessions.Load()),
			metrics.Counter("wayfinder_sessions_opened_total", "Total login sessions opened in the server-side registry (AP7).", sessionsOpened.Load()),
			metrics.Counter("wayfinder_session_logins_rejected_total", "Total logins refused by the concurrent-session limit under the reject policy (AP7).", sessionsRejected.Load()),
			metrics.Counter("wayfinder_sessions_revoked_total", "Total sessions revoked by an access/tenant pause or delete (AP7).", sessionsRevoked.Load()),
			metrics.Counter("wayfinder_sessions_expired_swept_total", "Total expired session rows removed by the janitor (AP7).", sessionsExpiredSwept.Load()),
			metrics.Counter("wayfinder_cat065_heartbeats_received_total", "Total number of CAT065 SDPS-status heartbeats received.", heartbeatCount.Load()),
			metrics.Gauge("wayfinder_feed_stale", "1 if the CAT065 heartbeat feed is currently stale, else 0.", feedStale),
			metrics.Counter("wayfinder_openaip_fetch_success_total", "Total successful OpenAIP fetches (per kind), summed across the global and all per-tenant caches.", aeroSuccess),
			metrics.Counter("wayfinder_openaip_fetch_failures_total", "Total failed OpenAIP fetches (per kind), summed across the global and all per-tenant caches.", aeroFailure),
			metrics.Gauge("wayfinder_openaip_cache_age_seconds", "Seconds since the last successful OpenAIP fetch of the global fallback cache, or -1 if never.", aeroService.CacheAgeSeconds(time.Now())),
		}
		// Weather feed metrics (WX-A, ADR 0016), source-labelled so the QNH/warnings
		// features (WX-B/C) can add their own series under the same names.
		if weatherRadar != nil {
			radar := metrics.Label{Name: "source", Value: "dwd_radar"}
			mset = append(mset,
				metrics.Counter("wayfinder_weather_fetch_success_total", "Total successful weather-source fetches, by source.", weatherRadar.FetchSuccessCount()).With(radar),
				metrics.Counter("wayfinder_weather_fetch_failures_total", "Total failed weather-source fetches, by source.", weatherRadar.FetchFailureCount()).With(radar),
				metrics.Gauge("wayfinder_weather_cache_age_seconds", "Seconds since the last successful weather-source fetch, or -1 if never, by source.", weatherRadar.CacheAgeSeconds(time.Now())).With(radar),
			)
		}
		if weatherQNH != nil {
			metar := metrics.Label{Name: "source", Value: "noaa_metar"}
			mset = append(mset,
				metrics.Counter("wayfinder_weather_fetch_success_total", "Total successful weather-source fetches, by source.", weatherQNH.FetchSuccessCount()).With(metar),
				metrics.Counter("wayfinder_weather_fetch_failures_total", "Total failed weather-source fetches, by source.", weatherQNH.FetchFailureCount()).With(metar),
				metrics.Gauge("wayfinder_weather_cache_age_seconds", "Seconds since the last successful weather-source fetch, or -1 if never, by source.", weatherQNH.CacheAgeSeconds(time.Now())).With(metar),
			)
		}
		if weatherWarn != nil {
			warn := metrics.Label{Name: "source", Value: "dwd_warnings"}
			mset = append(mset,
				metrics.Counter("wayfinder_weather_fetch_success_total", "Total successful weather-source fetches, by source.", weatherWarn.FetchSuccessCount()).With(warn),
				metrics.Counter("wayfinder_weather_fetch_failures_total", "Total failed weather-source fetches, by source.", weatherWarn.FetchFailureCount()).With(warn),
				metrics.Gauge("wayfinder_weather_cache_age_seconds", "Seconds since the last successful weather-source fetch, or -1 if never, by source.", weatherWarn.CacheAgeSeconds(time.Now())).With(warn),
			)
		}
		// Active-session gauge (AP7): a live count from the registry at scrape time,
		// under a short timeout so a slow database cannot wedge the scrape. Skipped
		// under proxy auth (no local registry) and on a query error (better to omit
		// the series than to report a wrong value).
		if sessionRepo != nil {
			countCtx, cancelCount := context.WithTimeout(r.Context(), 2*time.Second)
			if n, cerr := sessionRepo.CountActiveSessions(countCtx); cerr == nil {
				mset = append(mset, metrics.Gauge("wayfinder_active_sessions", "Currently active (unexpired) sessions in the server-side registry (AP7).", int64(n)))
			}
			cancelCount()
		}
		// Per-tenant series (WF2-23.2). Labelled only by the stable tenant_id —
		// never by high-cardinality identity (user/session), which stays in the
		// audit log. Emitted only in multi-tenant mode (tenants present).
		for _, tm := range broadcaster.TenantMetrics() {
			lbl := metrics.Label{Name: "tenant", Value: strconv.FormatInt(tm.TenantID, 10)}
			mset = append(mset,
				metrics.Gauge("wayfinder_tenant_ws_clients_connected", "Currently connected WebSocket clients per tenant.", tm.Connected).With(lbl),
				metrics.Counter("wayfinder_tenant_tracks_delivered_total", "Total track messages delivered to a tenant's clients.", tm.Delivered).With(lbl),
			)
		}
		// Feature entitlement fail-closed counters (WF2-50): non-zero means a check
		// denied access due to a store error or an unknown key. Multi-tenant only.
		if featSvc != nil {
			const help = "Total feature checks that failed closed (denied), by reason."
			mset = append(mset,
				metrics.Counter("wayfinder_feature_check_failclosed_total", help, featSvc.DBErrorCount()).With(metrics.Label{Name: "reason", Value: "db_error"}),
				metrics.Counter("wayfinder_feature_check_failclosed_total", help, featSvc.UnknownKeyCount()).With(metrics.Label{Name: "reason", Value: "unknown_key"}),
			)
		}
		metrics.Handler(mset...)(w, r)
	})

	addr := ":8080"
	logger.Info("starting probe server", slog.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("probe server error", slog.String("error", err.Error()))
	}
}
