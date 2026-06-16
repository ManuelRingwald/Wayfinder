package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
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

	"github.com/manuelringwald/wayfinder/internal/webui"
	"github.com/manuelringwald/wayfinder/pkg/aeronautical"
	"github.com/manuelringwald/wayfinder/pkg/broadcast"
	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/cat065"
	"github.com/manuelringwald/wayfinder/pkg/health"
	"github.com/manuelringwald/wayfinder/pkg/metrics"
	"github.com/manuelringwald/wayfinder/pkg/receiver"
	"github.com/manuelringwald/wayfinder/pkg/ws"
)

func main() {
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
	var lastServiceID atomic.Uint32
	var lastError atomic.Pointer[string]

	// Feed-health tracker (CAT065 heartbeat staleness, Firefly ADR 0018).
	feedHealth := health.New(cfg.FeedStaleTimeout)

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

	// broadcastFeedStatus pushes the current feed-health state to the browser.
	broadcastFeedStatus := func(status health.Status) {
		state := "unknown"
		if status.EverSeen {
			state = "ok"
			if status.Stale {
				state = "stale"
			}
		}
		_ = broadcaster.Send(broadcast.Message{
			FeedStatus: &broadcast.FeedStatusMessage{
				State:     state,
				ServiceID: uint8(lastServiceID.Load()),
			},
		})
	}

	// Create receiver with handlers that feed the broadcaster and feed health.
	recv, err := receiver.New(receiver.Config{
		Group:  cfg.MulticastGroup,
		Port:   cfg.MulticastPort,
		Logger: logger,
		Handler: func(tracks []cat062.DecodedTrack) error {
			blockCount.Add(1)
			trackCount.Add(int64(len(tracks)))
			tracksCurrent.Store(int64(len(tracks)))
			// Feed tracks to broadcaster (non-blocking).
			select {
			case broadcaster.TracksChan() <- tracks:
			default:
				logger.Warn("broadcaster channel full, dropping block")
			}
			return nil
		},
		StatusHandler: func(status cat065.ServiceStatus) error {
			heartbeatCount.Add(1)
			lastServiceID.Store(uint32(status.ServiceID))
			feedHealth.RecordHeartbeat(time.Now())
			// Notify the browser only on a state transition (e.g. first
			// heartbeat, or recovery from stale).
			if s, changed := feedHealth.Observe(time.Now()); changed {
				broadcastFeedStatus(s)
			}
			return nil
		},
	})
	if err != nil {
		logger.Error("create receiver", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer recv.Close()

	// Listen on multicast.
	if err := recv.Listen(); err != nil {
		logger.Error("listen multicast", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Graceful shutdown on SIGTERM/SIGINT.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run receiver and broadcaster in parallel.
	var wg sync.WaitGroup

	// Receiver goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := recv.Run(ctx); err != nil && err != context.Canceled {
			msg := err.Error()
			lastError.Store(&msg)
			logger.Error("receiver error", slog.String("error", err.Error()))
			cancel()
		}
	}()

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

	// Monitor feed staleness: even with no traffic, periodically re-evaluate
	// the heartbeat age and notify the browser when the feed flips ok→stale
	// (or recovers). Webhook-style pushes alone can't detect "nothing arrived".
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
				if s, changed := feedHealth.Observe(time.Now()); changed {
					broadcastFeedStatus(s)
				}
			}
		}
	}()

	// Start the aeronautical refresh loop (best-effort, ADR 0004).
	go aeroService.Run(ctx)

	// Start health/readiness/metrics probe server.
	go startProbeServer(logger, &blockCount, &trackCount, &tracksCurrent, &heartbeatCount, broadcaster, recv, feedHealth, aeroService, &lastError)

	// Start WebSocket server.
	if cfg.AuthToken == "" {
		logger.Warn("WAYFINDER_AUTH_TOKEN not set — browser edge relies on " +
			"network isolation / a TLS+auth reverse proxy in front of this " +
			"service (ADR 0003)")
	}

	mux := http.NewServeMux()

	wsHandler := ws.New(broadcaster, logger, cfg.AllowedOrigins)
	mux.Handle("/ws", wsHandler)

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

	handler := authMiddleware(cfg.AuthToken, mux)

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
	ProbePort      int
	MapCenterLat   float64
	MapCenterLon   float64
	MapZoom        float64
	MapStyleURL    string
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
}

// defaultMapStyle is a minimal MapLibre style using OpenStreetMap raster
// tiles. It needs no API key, which keeps the demo self-contained.
const defaultMapStyle = `{
	"version": 8,
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
	}

	if cfg.MulticastGroup == "" {
		cfg.MulticastGroup = "239.255.0.62"
	}

	if portStr := os.Getenv("FIREFLY_CAT062_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.MulticastPort = port
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

	return cfg
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
			"center_lat": cfg.MapCenterLat,
			"center_lon": cfg.MapCenterLon,
			"zoom":       cfg.MapZoom,
			"style":      styleValue,
			"theme":      theme,
		})
	}
}

// startProbeServer starts an HTTP server for health, readiness and metrics.
func startProbeServer(logger *slog.Logger, blockCount, trackCount, tracksCurrent, heartbeatCount *atomic.Int64, broadcaster *broadcast.Broadcaster, recv *receiver.Receiver, feedHealth *health.FeedHealth, aeroService *aeronautical.Service, lastError *atomic.Pointer[string]) {
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
		status := feedHealth.Status(time.Now())
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
		if feedHealth.Status(time.Now()).Stale {
			feedStale = 1
		}
		metrics.Handler(
			metrics.Counter("wayfinder_cat062_blocks_received_total", "Total number of CAT062 data blocks received via multicast.", blockCount.Load()),
			metrics.Counter("wayfinder_cat062_tracks_received_total", "Total number of track records received across all CAT062 blocks.", trackCount.Load()),
			metrics.Counter("wayfinder_cat062_decode_errors_total", "Total number of CAT062 data blocks that failed to decode.", recv.DecodeErrorCount()),
			metrics.Gauge("wayfinder_tracks_current", "Number of tracks in the most recently received CAT062 block.", tracksCurrent.Load()),
			metrics.Gauge("wayfinder_ws_clients_connected", "Number of currently connected WebSocket clients.", int64(broadcaster.ClientCount())),
			metrics.Counter("wayfinder_ws_clients_evicted_total", "Total number of WebSocket clients evicted due to a full send channel.", broadcaster.EvictedCount()),
			metrics.Counter("wayfinder_cat065_heartbeats_received_total", "Total number of CAT065 SDPS-status heartbeats received.", heartbeatCount.Load()),
			metrics.Gauge("wayfinder_feed_stale", "1 if the CAT065 heartbeat feed is currently stale, else 0.", feedStale),
			metrics.Counter("wayfinder_openaip_fetch_success_total", "Total number of successful OpenAIP aeronautical fetches (per kind).", aeroService.FetchSuccessCount()),
			metrics.Counter("wayfinder_openaip_fetch_failures_total", "Total number of failed OpenAIP aeronautical fetches (per kind).", aeroService.FetchFailureCount()),
			metrics.Gauge("wayfinder_openaip_cache_age_seconds", "Seconds since the last successful OpenAIP fetch, or -1 if never.", aeroService.CacheAgeSeconds(time.Now())),
		)(w, r)
	})

	addr := ":8080"
	logger.Info("starting probe server", slog.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("probe server error", slog.String("error", err.Error()))
	}
}
