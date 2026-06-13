package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/manuelringwald/wayfinder/pkg/broadcast"
	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/receiver"
	"github.com/manuelringwald/wayfinder/pkg/ws"
)

func main() {
	// Setup logging.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment.
	cfg := loadConfig()

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
	wsHandler := ws.New(broadcaster, logger)
	http.Handle("/ws", wsHandler)

	go func() {
		logger.Info("starting websocket server", slog.String("addr", ":8081"))
		if err := http.ListenAndServe(":8081", nil); err != nil && err != http.ErrServerClosed {
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
}

// loadConfig loads configuration from environment variables.
func loadConfig() Config {
	cfg := Config{
		MulticastGroup: os.Getenv("FIREFLY_CAT062_GROUP"),
		MulticastPort:  8600,
		ProbePort:      8080,
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

	return cfg
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
