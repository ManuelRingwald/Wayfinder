package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
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
	"github.com/manuelringwald/wayfinder/pkg/coverage"
	"github.com/manuelringwald/wayfinder/pkg/feature"
	"github.com/manuelringwald/wayfinder/pkg/health"
	"github.com/manuelringwald/wayfinder/pkg/impersonation"
	"github.com/manuelringwald/wayfinder/pkg/metrics"
	"github.com/manuelringwald/wayfinder/pkg/receiver"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
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

	// Aeronautical layers (ASD-003, ADR 0004): best-effort OpenAIP overlays.
	// Enabled only when an API key is configured; never affects the track path
	// or readiness. The query window is a box around the configured map center.
	aeroService := aeronautical.NewService(
		aeronautical.NewClient(&http.Client{Timeout: 15 * time.Second}, cfg.OpenAIPBaseURL, cfg.OpenAIPAPIKey),
		aeronautical.Config{
			Enabled: cfg.OpenAIPAPIKey != "",
			BBox:    aeronautical.BoundingBoxFromCenter(cfg.MapCenterLat, cfg.MapCenterLon, cfg.OpenAIPRadiusKM),
			Refresh: cfg.OpenAIPRefresh,
		},
		logger,
	)

	// broadcastFeedSnapshot pushes the per-feed health snapshot to clients
	// subscribed to that feed (Option B, WF-3). Color is derived from the
	// FeedSnapshot and carries CAT065 liveness + CAT063 sensor counts.
	broadcastFeedSnapshot := func(feedID int64, snap health.FeedSnapshot) {
		_ = broadcaster.Send(broadcast.Message{
			FeedStatus: &broadcast.FeedStatusMessage{
				FeedID:        feedID,
				Color:         snap.Color(),
				SensorsActive: snap.SensorsActive,
				SensorsTotal:  snap.SensorsTotal,
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
		for _, s := range statuses {
			if s.Operational {
				active++
			}
		}
		feedRegistry.RecordSensors(feedID, active, len(statuses))
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

	// Multi-tenancy (WF2-12): when WAYFINDER_DB_URL is set, open the DB, migrate
	// and build the tenant-context middleware; otherwise run single-tenant. Done
	// before the receivers so the feed catalogue (WF2-20.2) can drive them.
	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 30*time.Second)
	tenantMW, dbPool, err := setupTenancy(setupCtx, cfg, logger)
	if err != nil {
		cancelSetup()
		logger.Error("tenancy setup", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if dbPool != nil {
		defer dbPool.Close()
	}

	// Resolve the feeds to receive: the DB catalogue (multi-feed, WF2-20.2) when
	// present and non-empty, else the single ENV-configured feed (single-tenant /
	// empty-catalogue fallback).
	var catalogue []store.Feed
	if dbPool != nil {
		catalogue, err = store.NewFeedRepo(dbPool).List(setupCtx)
	}
	cancelSetup()
	if err != nil {
		logger.Error("list feed catalogue", slog.String("error", err.Error()))
		os.Exit(1)
	}
	feeds := resolveFeeds(catalogue, cfg)
	logger.Info("feeds resolved", slog.Int("count", len(feeds)))

	// One receiver per feed; each stamps its feed_id onto decoded tracks.
	recvs, err := buildReceivers(feeds, logger, trackHandler, statusHandler, sensorStatusHandler)
	if err != nil {
		logger.Error("create receivers", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Join each feed. A feed that fails to listen is logged and skipped so one
	// misconfigured feed doesn't take down the others; if none can join, fatal.
	var listening []*receiver.Receiver
	for i, r := range recvs {
		if err := r.Listen(); err != nil {
			logger.Error("feed listen failed",
				slog.Int64("feed_id", feeds[i].ID),
				slog.String("group", feeds[i].Group),
				slog.Int("port", feeds[i].Port),
				slog.String("error", err.Error()))
			continue
		}
		listening = append(listening, r)
	}
	if len(listening) == 0 {
		logger.Error("no feeds could be joined")
		os.Exit(1)
	}
	for _, r := range listening {
		defer r.Close()
	}

	// Graceful shutdown on SIGTERM/SIGINT.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run receivers and broadcaster in parallel.
	var wg sync.WaitGroup

	// Receiver goroutines (one per joined feed).
	for _, r := range listening {
		wg.Add(1)
		go func(r *receiver.Receiver) {
			defer wg.Done()
			if err := r.Run(ctx); err != nil && err != context.Canceled {
				msg := err.Error()
				lastError.Store(&msg)
				logger.Error("receiver error", slog.String("error", err.Error()))
				cancel()
			}
		}(r)
	}

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
					for _, f := range feeds {
						broadcastFeedSnapshot(f.ID, feedRegistry.Snapshot(f.ID, now))
					}
				}
			}
		}
	}()

	// Start the aeronautical refresh loop (best-effort, ADR 0004).
	go aeroService.Run(ctx)

	// Feature entitlements (WF2-50): per-tenant feature flags as data, fail-closed.
	// Built once here so the admin API and /metrics share one instance. Multi-tenant
	// only (needs the tenant DB); single-tenant → nil, no feature gating.
	var featSvc *feature.Service
	if dbPool != nil {
		featSvc = feature.New(store.NewEntitlementRepo(dbPool), logger)
	}

	// Start health/readiness/metrics probe server.
	decodeErrors := func() int64 {
		var n int64
		for _, r := range listening {
			n += r.DecodeErrorCount()
		}
		return n
	}
	go startProbeServer(logger, &blockCount, &trackCount, &tracksCurrent, &heartbeatCount, broadcaster, decodeErrors, feedRegistry, aeroService, &lastError, featSvc)

	// Start WebSocket server.
	if tenantMW == nil && cfg.AuthToken == "" {
		logger.Warn("WAYFINDER_AUTH_TOKEN not set — browser edge relies on " +
			"network isolation / a TLS+auth reverse proxy in front of this " +
			"service (ADR 0003)")
	}

	mux := http.NewServeMux()

	// Scoped fan-out (WF2-21): with multi-tenancy on, each /ws client is filtered
	// to the feeds its tenant subscribes to; single-tenant → nil resolver (unscoped).
	var scopeResolver ws.ScopeResolver
	if dbPool != nil {
		// Cross-tenant read-only impersonation (ADR 0008) needs a signing key;
		// without one (proxy/none mode without WAYFINDER_SESSION_KEY) it stays
		// disabled and the resolver behaves exactly as before (impChecker nil).
		var impChecker impersonation.TenantChecker
		if len(cfg.SessionKey) > 0 {
			impChecker = tenantExistsChecker{repo: store.NewTenantRepo(dbPool)}
		}
		scopeResolver = newScopeResolver(store.NewSubscriptionRepo(dbPool), store.NewViewConfigRepo(dbPool), impChecker, cfg.SessionKey, logger)
	}
	wsHandler := ws.New(broadcaster, logger, cfg.AllowedOrigins, scopeResolver)
	// The live picture is tenant-scoped: gate /ws with the tenant middleware when
	// multi-tenancy is enabled; the middleware sets the Identity the resolver reads.
	if tenantMW != nil {
		mux.Handle("/ws", tenantMW(wsHandler))
	} else {
		mux.Handle("/ws", wsHandler)
	}

	// Serve the ASD frontend (static HTML/JS/CSS) and its map configuration.
	frontend, err := webui.Handler()
	if err != nil {
		logger.Error("create frontend handler", slog.String("error", err.Error()))
		os.Exit(1)
	}
	mux.Handle("/", frontend)
	mux.HandleFunc("/api/map-config", mapConfigHandler(cfg))

	// Aeronautical GeoJSON endpoints (/api/airspace, /api/navaids,
	// /api/waypoints), served from the OpenAIP cache (ADR 0004).
	aeroService.Register(mux)

	// Coverage rings: static GeoJSON computed once from config, served to the
	// browser on demand. An empty FeatureCollection is returned when no sensors
	// are configured so the frontend can always fetch unconditionally.
	mux.HandleFunc("/api/coverage/rings", coverageRingsHandler(cfg))

	// Builtin-mode login/logout (WF2-12.3): only when multi-tenancy is on and the
	// auth mode is builtin (proxy/none mint no local sessions). These routes are
	// intentionally unauthenticated — they hand out the session the middleware
	// later checks.
	if dbPool != nil && cfg.AuthMode == auth.ModeBuiltin {
		loginCfg := tenant.LoginConfig{
			SessionKey: cfg.SessionKey,
			CookieName: cfg.SessionCookie,
			TTL:        cfg.SessionTTL,
			Secure:     cfg.TLSCertFile != "" && cfg.TLSKeyFile != "",
		}
		users := store.NewUserRepo(dbPool)
		creds := store.NewCredentialRepo(dbPool)
		tenants := store.NewTenantRepo(dbPool)
		mux.Handle("/api/login", tenant.LoginHandler(users, creds, tenants, loginCfg))
		mux.Handle("/api/logout", tenant.LogoutHandler(loginCfg))
		logger.Info("builtin login enabled", slog.String("path", "/api/login"))
	}

	// Admin surface (WF2-13/31/32): the tenant-scoped admin REST API is role-gated
	// to admin (ADR 0009) and carries the whoami role probe the SPA reads on
	// entering /admin (GET /api/admin/whoami). The browser route /admin itself is no
	// longer a backend endpoint — it is served by the SPA shell via the history-mode
	// fallback in webui.Handler. Only mounted with multi-tenancy active — the gate
	// needs an Identity from the tenant middleware.
	if tenantMW != nil {
		requireAdmin := tenant.RequireRole(store.RoleAdmin)
		viewRepo := store.NewViewConfigRepo(dbPool)
		subRepo := store.NewSubscriptionRepo(dbPool)
		// Live-apply (WF2-33): when an admin changes a tenant's view or feed
		// grants, re-scope that tenant's connected clients in place — no reconnect.
		rescope := func(ctx context.Context, tenantID int64) {
			rescopeTenant(ctx, broadcaster, subRepo, viewRepo, logger, tenantID)
		}
		adminAPI := adminapi.New(viewRepo, subRepo, store.NewFeedRepo(dbPool), store.NewTenantRepo(dbPool),
			store.NewUserRepo(dbPool), store.NewCredentialRepo(dbPool), featSvc, feedRegistry, logger, rescope)
		mux.Handle("/api/admin/", tenantMW(requireAdmin(adminAPI)))

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
	}

	// The per-tenant middleware (on /ws) supersedes the legacy single shared
	// token; only fall back to the token gate in single-tenant mode.
	var handler http.Handler = mux
	if tenantMW == nil {
		handler = authMiddleware(cfg.AuthToken, mux)
	}

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
	// FeedID is the catalogue feed this single-feed receiver consumes
	// (WAYFINDER_FEED_ID, default 0 = single-tenant/unassigned). Stamped onto
	// every track for the scoped fan-out (WF2-20/21). The DB-driven multi-feed
	// receiver supersedes this single value in WF2-20.2.
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
	AuthToken      string
	TLSCertFile    string
	TLSKeyFile     string
	LogLevel       slog.Level
	// FeedStaleTimeout is how long without a CAT065 heartbeat before the feed
	// is considered stale (Firefly ADR 0018). `WAYFINDER_FEED_STALE_TIMEOUT`
	// in seconds, default 3 s (~3× the 1 s heartbeat period).
	FeedStaleTimeout time.Duration

	// OpenAIP aeronautical layers (ASD-003, ADR 0004). The feature is enabled
	// only when an API key is configured; otherwise the map shows no overlays.
	OpenAIPAPIKey   string        // WAYFINDER_OPENAIP_API_KEY (secret)
	OpenAIPBaseURL  string        // WAYFINDER_OPENAIP_BASE_URL (optional override)
	OpenAIPRefresh  time.Duration // WAYFINDER_OPENAIP_REFRESH (Go duration, default 24h)
	OpenAIPRadiusKM float64       // WAYFINDER_OPENAIP_RADIUS_KM (default 250)

	// Coverage rings overlay (Paket 6, ASD-012 extension).
	// Populated from WAYFINDER_COVERAGE_SENSOR_N_* env-vars.
	CoverageSensors   []coverage.SensorConfig // WAYFINDER_COVERAGE_SENSOR_N_{LAT,LON,...}
	CoverageRingColor string                  // WAYFINDER_COVERAGE_RING_COLOR, default #5B8DEF

	// Multi-tenancy (Wayfinder 2.0, WF2-12, ADR 0005/0006). Multi-tenancy is
	// enabled only when DBURL is set; otherwise the server runs as the legacy
	// single-tenant ASD (ADR 0005 §7, degenerate case) with no database and no
	// tenant middleware.
	DBURL            string        // WAYFINDER_DB_URL (PostgreSQL DSN; empty = single-tenant)
	AuthMode         auth.Mode     // WAYFINDER_AUTH_MODE (proxy|builtin|none, default none)
	NoneSubject      string        // WAYFINDER_NONE_SUBJECT (ModeNone fixed subject, default "default")
	SessionKey       []byte        // WAYFINDER_SESSION_KEY (ModeBuiltin HMAC key)
	SessionCookie    string        // WAYFINDER_SESSION_COOKIE (default "wf_session")
	SessionTTL       time.Duration // WAYFINDER_SESSION_TTL (Go duration, default 12h)
	ImpersonationTTL time.Duration // WAYFINDER_IMPERSONATION_TTL (read-only impersonation grant lifetime, default 30m; ADR 0008)
	OIDCIssuer       string        // WAYFINDER_OIDC_ISSUER (ModeProxy)
	OIDCAudience     string        // WAYFINDER_OIDC_AUDIENCE (ModeProxy)
}

// authConfig projects the runtime Config onto the auth package's Config.
func (c Config) authConfig() auth.Config {
	return auth.Config{
		Mode:         c.AuthMode,
		NoneSubject:  c.NoneSubject,
		CookieName:   c.SessionCookie,
		SessionKey:   c.SessionKey,
		OIDCIssuer:   c.OIDCIssuer,
		OIDCAudience: c.OIDCAudience,
	}
}

// defaultMapStyle is a minimal MapLibre style using OpenStreetMap raster
// tiles. It needs no API key, which keeps the demo self-contained. The "glyphs"
// endpoint (keyless fonts.openmaptiles.org) is required for any text to render:
// a symbol layer with a text-field draws nothing without a font source. For a
// fully air-gapped deployment, self-host glyphs and tiles via WAYFINDER_MAP_STYLE_URL.
const defaultMapStyle = `{
	"version": 8,
	"glyphs": "https://fonts.openmaptiles.org/{fontstack}/{range}.pbf",
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

// darkMapStyle is the "Radar Dark Mode" base: a low-contrast dark raster
// (CARTO dark, no labels) on a dark background. Like OSM it needs no API key,
// which keeps the demo self-contained. The dark, label-free base lets the
// track symbols and aeronautical overlays dominate, the way a controller's
// radar scope does. ASD-003 Häppchen 3a.
const darkMapStyle = `{
	"version": 8,
	"glyphs": "https://fonts.openmaptiles.org/{fontstack}/{range}.pbf",
	"sources": {
		"carto-dark": {
			"type": "raster",
			"tiles": ["https://basemaps.cartocdn.com/dark_nolabels/{z}/{x}/{y}.png"],
			"tileSize": 256,
			"attribution": "© OpenStreetMap contributors © CARTO"
		}
	},
	"layers": [
		{"id": "background", "type": "background", "paint": {"background-color": "#0b0f14"}},
		{"id": "carto-dark", "type": "raster", "source": "carto-dark"}
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
		OpenAIPRefresh:   24 * time.Hour,
		OpenAIPRadiusKM:  250,
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

	cfg.AuthToken = os.Getenv("WAYFINDER_AUTH_TOKEN")
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

	if v := os.Getenv("WAYFINDER_OPENAIP_REFRESH"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.OpenAIPRefresh = d
		}
	}

	if v := os.Getenv("WAYFINDER_OPENAIP_RADIUS_KM"); v != "" {
		if km, err := strconv.ParseFloat(v, 64); err == nil && km > 0 {
			cfg.OpenAIPRadiusKM = km
		}
	}

	// Coverage rings: sensor positions and ranges from env-vars.
	cfg.CoverageSensors = coverage.ParseEnv(os.Getenv)
	cfg.CoverageRingColor = os.Getenv("WAYFINDER_COVERAGE_RING_COLOR")
	if cfg.CoverageRingColor == "" {
		cfg.CoverageRingColor = "#5B8DEF"
	}

	// Multi-tenancy (WF2-12). Enabled only when WAYFINDER_DB_URL is set.
	cfg.DBURL = os.Getenv("WAYFINDER_DB_URL")
	cfg.AuthMode, _ = auth.ParseMode(os.Getenv("WAYFINDER_AUTH_MODE")) // invalid → none
	cfg.NoneSubject = os.Getenv("WAYFINDER_NONE_SUBJECT")
	if v := os.Getenv("WAYFINDER_SESSION_KEY"); v != "" {
		cfg.SessionKey = []byte(v)
	}
	cfg.SessionCookie = os.Getenv("WAYFINDER_SESSION_COOKIE")
	if v := os.Getenv("WAYFINDER_SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.SessionTTL = d
		}
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

// setupTenancy wires up multi-tenancy when WAYFINDER_DB_URL is configured: it
// opens the database, applies migrations, builds the configured authenticator
// and returns the tenant-context middleware. When no database is configured it
// returns (nil, nil, nil) — the server then runs as the legacy single-tenant ASD
// (ADR 0005 §7). The returned pool (if any) must be closed by the caller.
func setupTenancy(ctx context.Context, cfg Config, logger *slog.Logger) (func(http.Handler) http.Handler, *pgxpool.Pool, error) {
	if cfg.DBURL == "" {
		logger.Warn("WAYFINDER_DB_URL not set — running as single-tenant ASD " +
			"(no database, no tenant isolation); set it to enable multi-tenancy (ADR 0005)")
		return nil, nil, nil
	}

	pool, err := store.Open(ctx, cfg.DBURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}
	if err := store.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("migrate schema: %w", err)
	}

	authenticator, err := auth.NewAuthenticator(ctx, cfg.authConfig())
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("build authenticator: %w", err)
	}

	if cfg.AuthMode == auth.ModeNone {
		logger.Warn("WAYFINDER_AUTH_MODE=none — every request is the same fixed " +
			"subject; relies on network isolation (ADR 0003)")
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

// authMiddleware enforces WAYFINDER_AUTH_TOKEN (if configured) on every
// request: a bearer token via the Authorization header, or a "token" query
// parameter (since browsers cannot set custom headers on the WebSocket
// handshake). If no token is configured, requests pass through unchanged
// (ADR 0003: this is a fail-closed *opt-in* on top of the primary
// TLS/Auth-at-the-proxy mechanism).
func authMiddleware(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.URL.Query().Get("token")
		if provided == "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				provided = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="wayfinder"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
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
func startProbeServer(logger *slog.Logger, blockCount, trackCount, tracksCurrent, heartbeatCount *atomic.Int64, broadcaster *broadcast.Broadcaster, decodeErrors func() int64, feedRegistry *health.Registry, aeroService *aeronautical.Service, lastError *atomic.Pointer[string], featSvc *feature.Service) {
	mux := http.NewServeMux()

	// /health — liveness check (always ready unless startup failed).
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
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
			w.Write([]byte(`{"status":"ready","blocks":` + strconv.FormatInt(count, 10) + `,"clients":` + strconv.Itoa(clients) + `,"feed_stale":` + strconv.FormatBool(status.Stale) + `}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready","blocks":` + strconv.FormatInt(count, 10) + `,"clients":` + strconv.Itoa(clients) + `,"feed_stale":` + strconv.FormatBool(status.Stale) + `}`))
	})

	// /metrics — Prometheus text exposition (REQ NFR-OBS-002): track
	// throughput, decode errors, WebSocket client counts/drops, and feed health.
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		feedStale := int64(0)
		if feedRegistry.Status(time.Now()).Stale {
			feedStale = 1
		}
		mset := []metrics.Metric{
			metrics.Counter("wayfinder_cat062_blocks_received_total", "Total number of CAT062 data blocks received via multicast.", blockCount.Load()),
			metrics.Counter("wayfinder_cat062_tracks_received_total", "Total number of track records received across all CAT062 blocks.", trackCount.Load()),
			metrics.Counter("wayfinder_cat062_decode_errors_total", "Total number of CAT062 data blocks that failed to decode.", decodeErrors()),
			metrics.Gauge("wayfinder_tracks_current", "Number of tracks in the most recently received CAT062 block.", tracksCurrent.Load()),
			metrics.Gauge("wayfinder_ws_clients_connected", "Number of currently connected WebSocket clients.", int64(broadcaster.ClientCount())),
			metrics.Counter("wayfinder_ws_clients_evicted_total", "Total number of WebSocket clients evicted due to a full send channel.", broadcaster.EvictedCount()),
			metrics.Counter("wayfinder_impersonation_sessions_total", "Total admin read-only impersonation /ws sessions started (ADR 0008). Excluded from the per-tenant series.", impersonationSessions.Load()),
			metrics.Counter("wayfinder_cat065_heartbeats_received_total", "Total number of CAT065 SDPS-status heartbeats received.", heartbeatCount.Load()),
			metrics.Gauge("wayfinder_feed_stale", "1 if the CAT065 heartbeat feed is currently stale, else 0.", feedStale),
			metrics.Counter("wayfinder_openaip_fetch_success_total", "Total number of successful OpenAIP aeronautical fetches (per kind).", aeroService.FetchSuccessCount()),
			metrics.Counter("wayfinder_openaip_fetch_failures_total", "Total number of failed OpenAIP aeronautical fetches (per kind).", aeroService.FetchFailureCount()),
			metrics.Gauge("wayfinder_openaip_cache_age_seconds", "Seconds since the last successful OpenAIP fetch, or -1 if never.", aeroService.CacheAgeSeconds(time.Now())),
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
