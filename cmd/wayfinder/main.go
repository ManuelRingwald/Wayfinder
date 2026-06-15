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

	"github.com/manuelringwald/wayfinder/internal/webui"
	"github.com/manuelringwald/wayfinder/pkg/broadcast"
	"github.com/manuelringwald/wayfinder/pkg/cat062"
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

	// Track reception state for health checks.
	var blockCount atomic.Int64
	var trackCount atomic.Int64
	var lastError atomic.Pointer[string]

	// Create receiver with handler that feeds broadcaster.
	recv, err := receiver.New(receiver.Config{
		Group:  cfg.MulticastGroup,
		Port:   cfg.MulticastPort,
		Logger: logger,
		Handler: func(tracks []cat062.DecodedTrack) error {
			blockCount.Add(1)
			trackCount.Add(int64(len(tracks)))
			// Feed tracks to broadcaster (non-blocking).
			select {
			case broadcaster.TracksChan() <- tracks:
			default:
				logger.Warn("broadcaster channel full, dropping block")
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

	// Start health/readiness probe server.
	go startProbeServer(logger, &blockCount, broadcaster, &lastError)

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
	AllowedOrigins []string
	AuthToken      string
	TLSCertFile    string
	TLSKeyFile     string
	LogLevel       slog.Level
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

// loadConfig loads configuration from environment variables.
func loadConfig() Config {
	cfg := Config{
		MulticastGroup: os.Getenv("FIREFLY_CAT062_GROUP"),
		MulticastPort:  8600,
		ProbePort:      8080,
		// Default map center: Frankfurt am Main, matching Firefly's demo scenario.
		MapCenterLat: 50.0379,
		MapCenterLon: 8.5622,
		MapZoom:      8,
		MapStyleURL:  "",
		LogLevel:     slog.LevelInfo,
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

// mapConfigHandler serves the map center/zoom/style as JSON for the frontend.
func mapConfigHandler(cfg Config) http.HandlerFunc {
	style := cfg.MapStyleURL
	var styleValue any = style
	if style == "" {
		styleValue = json.RawMessage(defaultMapStyle)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"center_lat": cfg.MapCenterLat,
			"center_lon": cfg.MapCenterLon,
			"zoom":       cfg.MapZoom,
			"style":      styleValue,
		})
	}
}

// startProbeServer starts an HTTP server for health and readiness checks.
func startProbeServer(logger *slog.Logger, blockCount *atomic.Int64, broadcaster *broadcast.Broadcaster, lastError *atomic.Pointer[string]) {
	mux := http.NewServeMux()

	// /health — liveness check (always ready unless startup failed).
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// /ready — readiness check (ready once we have clients or blocks received).
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		count := blockCount.Load()
		clients := broadcaster.ClientCount()
		if count > 0 || clients > 0 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ready","blocks":` + strconv.FormatInt(count, 10) + `,"clients":` + strconv.Itoa(clients) + `}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready","blocks":0,"clients":0}`))
	})

	addr := ":8080"
	logger.Info("starting probe server", slog.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("probe server error", slog.String("error", err.Error()))
	}
}
