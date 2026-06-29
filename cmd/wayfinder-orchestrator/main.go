// Command wayfinder-orchestrator is the control-plane process that auto-spawns
// one Firefly tracker instance per subscribed feed (ORCH-2c, ADR 0012).
//
// It is deliberately a SEPARATE binary from the browser-facing wayfinder server
// (ADR 0012 §6, least-privilege): only this process is granted the power to start
// tracker instances (a container runtime later, ORCH-2b/-6). The browser edge
// never holds that privilege — it only writes the desired state (feeds + source
// config) to the database; this process reads that desired state and converges
// the running set toward it via the reconciler. The two communicate only through
// the catalogue, never point-to-point.
//
// This step (ORCH-2c, 2/3) provides the process skeleton wired to the real
// store-backed desired state and the reconciler. The instance backend is still
// the in-memory placeholder (the real Docker adapter is ORCH-2b); secret
// resolution and a change-driven trigger are ORCH-2c (3/3).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/instance"
	"github.com/manuelringwald/wayfinder/pkg/orchestrator"
	"github.com/manuelringwald/wayfinder/pkg/reconciler"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// defaultInterval is the reconcile period when WAYFINDER_ORCHESTRATOR_INTERVAL is
// unset. The reconciler is idempotent, so a modest period is safe; feed changes
// converge within one interval until the change-driven trigger lands (ORCH-2c 3/3).
const defaultInterval = 15 * time.Second

// config is the orchestrator's resolved runtime configuration.
type config struct {
	dsn      string        // WAYFINDER_DB_URL
	interval time.Duration // WAYFINDER_ORCHESTRATOR_INTERVAL
	logLevel slog.Level    // WAYFINDER_LOG_LEVEL
	once     bool          // --once: run a single reconcile pass and exit
}

func main() {
	cfg, err := loadConfig(os.Getenv, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchestrator: "+err.Error())
		os.Exit(2)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.logLevel}))

	if err := run(context.Background(), cfg, logger); err != nil {
		logger.Error("orchestrator exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// loadConfig resolves configuration from environment and flags. getenv and args
// are injected so the parsing is unit-testable. An unset WAYFINDER_DB_URL is a
// hard error (the orchestrator has nothing to read without the catalogue); an
// invalid interval or log level falls back to the default rather than aborting
// (12-factor leniency, FR-CFG-002).
func loadConfig(getenv func(string) string, args []string) (config, error) {
	fs := flag.NewFlagSet("wayfinder-orchestrator", flag.ContinueOnError)
	once := fs.Bool("once", false, "run a single reconcile pass and exit (for CI/dev/k8s Job)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	dsn := getenv("WAYFINDER_DB_URL")
	if dsn == "" {
		return config{}, errors.New("WAYFINDER_DB_URL is not set")
	}

	interval := defaultInterval
	if v := getenv("WAYFINDER_ORCHESTRATOR_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		}
	}

	level := slog.LevelInfo
	if v := getenv("WAYFINDER_LOG_LEVEL"); v != "" {
		_ = level.UnmarshalText([]byte(v)) // invalid → leaves the default
	}

	return config{dsn: dsn, interval: interval, logLevel: level, once: *once}, nil
}

// run opens the catalogue, wires the store-backed desired state, the reconciler
// and the (placeholder) instance backend, and either reconciles once or runs the
// reconcile loop until a termination signal. It does NOT run schema migrations:
// the browser-facing wayfinder server owns the schema, so the control plane only
// reads — a single migrator avoids races and keeps this process's DB grants
// read-only-shaped.
func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	pool, err := store.Open(ctx, cfg.dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pool.Close()

	desired := orchestrator.NewStoreDesiredState(
		store.NewSubscriptionRepo(pool),
		store.NewFeedRepo(pool),
	)
	// Placeholder backend until the Docker adapter (ORCH-2b). In-memory state is
	// per-process: meaningful for the lifetime of a loop run, reset on restart —
	// real cross-restart persistence arrives with the container backend.
	backend := instance.NewMemoryBackend()
	rec := reconciler.New(desired, backend, logger)

	if cfg.once {
		logger.Info("orchestrator: single reconcile pass (--once)")
		return rec.Reconcile(ctx)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger.Info("orchestrator: starting reconcile loop",
		slog.Duration("interval", cfg.interval))
	err = rec.Run(ctx, cfg.interval)
	if errors.Is(err, context.Canceled) {
		logger.Info("orchestrator: shutting down cleanly")
		return nil
	}
	return err
}
