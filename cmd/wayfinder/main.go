package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/receiver"
)

func main() {
	// Setup logging.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment.
	cfg := loadConfig()

	// Track reception state for health checks.
	var blockCount atomic.Int64
	var trackCount atomic.Int64
	var lastError atomic.Pointer[string]

	// Create receiver with handler that tracks statistics.
	recv, err := receiver.New(receiver.Config{
		Group:  cfg.MulticastGroup,
		Port:   cfg.MulticastPort,
		Logger: logger,
		Handler: func(tracks []cat062.DecodedTrack) error {
			blockCount.Add(1)
			trackCount.Add(int64(len(tracks)))
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

	// Start health/readiness probe server.
	go startProbeServer(logger, &blockCount, &lastError)

	// Graceful shutdown on SIGTERM/SIGINT.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("signal received", slog.String("signal", sig.String()))
		cancel()
	}()

	// Run receiver (blocks until context is cancelled).
	if err := recv.Run(ctx); err != nil && err != context.Canceled {
		msg := err.Error()
		lastError.Store(&msg)
		logger.Error("receiver error", slog.String("error", err.Error()))
		os.Exit(1)
	}

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
func startProbeServer(logger *slog.Logger, blockCount *atomic.Int64, lastError *atomic.Pointer[string]) {
	mux := http.NewServeMux()

	// /health — liveness check (always ready unless startup failed).
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// /ready — readiness check (ready once we've received at least one block).
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		count := blockCount.Load()
		if count > 0 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ready","blocks":` + strconv.FormatInt(count, 10) + `}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready","blocks":0}`))
	})

	addr := ":8080"
	logger.Info("starting probe server", slog.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("probe server error", slog.String("error", err.Error()))
	}
}
