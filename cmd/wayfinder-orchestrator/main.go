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
// The process is wired to the real store-backed desired state and reconciler
// (ORCH-2c 2/3) and can drive either the in-memory placeholder backend (default,
// dev/CI) or the real Docker backend (ORCH-2b, WAYFINDER_ORCHESTRATOR_BACKEND=docker).
// A change-driven trigger (ORCH-2c 3b) makes it converge the instant a feed or
// subscription changes (Postgres LISTEN/NOTIFY), with the interval as the safety
// net. When a deployment key (WAYFINDER_SECRET_KEY) is configured, this process —
// and only this process — decrypts each feed's source credentials and injects them
// into the spawned tracker (ORCH-5b, ADR 0012 §6); without a key, credentialled
// sources run anonymously.
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

	"github.com/manuelringwald/wayfinder/pkg/dockerbackend"
	"github.com/manuelringwald/wayfinder/pkg/instance"
	"github.com/manuelringwald/wayfinder/pkg/orchestrator"
	"github.com/manuelringwald/wayfinder/pkg/reconciler"
	"github.com/manuelringwald/wayfinder/pkg/secret"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// defaultInterval is the reconcile period when WAYFINDER_ORCHESTRATOR_INTERVAL is
// unset. The reconciler is idempotent, so a modest period is safe; feed changes
// converge within one interval until the change-driven trigger lands (ORCH-2c 3/3).
const defaultInterval = 15 * time.Second

// backend kinds selectable via WAYFINDER_ORCHESTRATOR_BACKEND.
const (
	backendMemory = "memory" // in-memory placeholder (dev/CI; spawns nothing)
	backendDocker = "docker" // real Docker containers (ORCH-2b)
)

// config is the orchestrator's resolved runtime configuration.
type config struct {
	dsn      string        // WAYFINDER_DB_URL
	interval time.Duration // WAYFINDER_ORCHESTRATOR_INTERVAL
	logLevel slog.Level    // WAYFINDER_LOG_LEVEL
	once     bool          // --once: run a single reconcile pass and exit

	// Backend selection (ORCH-2b). The default is the in-memory placeholder so a
	// bare run never accidentally talks to a Docker socket; "docker" is opt-in.
	backend    string // WAYFINDER_ORCHESTRATOR_BACKEND: memory | docker
	fireflyImg string // WAYFINDER_FIREFLY_IMAGE (required for the docker backend)
	fireflyNet string // WAYFINDER_FIREFLY_NETWORK (default "host"; multicast)
	fireflyScn string // WAYFINDER_FIREFLY_SCENE (optional placeholder source)

	// secretKey is the deployment key (WAYFINDER_SECRET_KEY, base64 32 bytes) used
	// to decrypt per-feed source credentials at launch (ORCH-5b, ADR 0012 §6). nil
	// when unset/invalid — credentialled sources then run anonymously. This process
	// is the only one that decrypts credentials; the browser-facing server only
	// seals them.
	secretKey []byte
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

	backend := backendMemory
	if v := getenv("WAYFINDER_ORCHESTRATOR_BACKEND"); v != "" {
		backend = v
	}
	if backend != backendMemory && backend != backendDocker {
		return config{}, fmt.Errorf("WAYFINDER_ORCHESTRATOR_BACKEND must be %q or %q, got %q", backendMemory, backendDocker, backend)
	}
	fireflyImg := getenv("WAYFINDER_FIREFLY_IMAGE")
	if backend == backendDocker && fireflyImg == "" {
		return config{}, errors.New("WAYFINDER_FIREFLY_IMAGE is required when WAYFINDER_ORCHESTRATOR_BACKEND=docker")
	}
	fireflyNet := getenv("WAYFINDER_FIREFLY_NETWORK")
	if fireflyNet == "" {
		fireflyNet = "host"
	}

	return config{
		dsn: dsn, interval: interval, logLevel: level, once: *once,
		backend: backend, fireflyImg: fireflyImg, fireflyNet: fireflyNet,
		fireflyScn: getenv("WAYFINDER_FIREFLY_SCENE"),
		secretKey:  parseSecretKey(getenv("WAYFINDER_SECRET_KEY")),
	}, nil
}

// parseSecretKey decodes the deployment secret key (base64, 32 bytes) used to
// decrypt per-feed source credentials at launch (ORCH-5b, ADR 0012 §6). An empty
// or malformed value yields nil — credentialled sources then run anonymously (the
// run wiring logs a set-but-invalid key loudly). The decoded key never appears in
// the config string output or logs.
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

// newBackend builds the instance backend selected by the config. The Docker
// backend is the only path that touches the container runtime (ADR 0012 §6); the
// memory backend spawns nothing and is the safe default for dev/CI.
func newBackend(cfg config, logger *slog.Logger) (instance.Backend, error) {
	switch cfg.backend {
	case backendDocker:
		client, err := dockerbackend.NewDockerClient()
		if err != nil {
			return nil, fmt.Errorf("connect to docker: %w", err)
		}
		return dockerbackend.New(client, cfg.fireflyImg, cfg.fireflyNet, cfg.fireflyScn, logger), nil
	default:
		return instance.NewMemoryBackend(), nil
	}
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
	// Credential resolution (ORCH-5b, ADR 0012 §6): when a deployment key is
	// configured, this least-privilege process decrypts per-feed source credentials
	// and injects them into the spawned tracker. Without a usable key, credentialled
	// sources run anonymously (resolution is best-effort, never fatal).
	if cfg.secretKey != nil {
		cipher, err := secret.NewCipher(cfg.secretKey)
		if err != nil {
			return fmt.Errorf("build secret cipher: %w", err)
		}
		resolver := orchestrator.NewSecretResolver(store.NewSecretRepo(pool), cipher)
		desired.WithSecretResolver(resolver, logger)
		logger.Info("orchestrator: source credential resolution enabled")
	} else if os.Getenv("WAYFINDER_SECRET_KEY") != "" {
		logger.Warn("WAYFINDER_SECRET_KEY set but invalid (need base64-encoded 32 bytes) — credentialled sources will run anonymously")
	}
	backend, err := newBackend(cfg, logger)
	if err != nil {
		return err
	}
	logger.Info("orchestrator: backend selected", slog.String("backend", cfg.backend))
	rec := reconciler.New(desired, backend, logger)

	if cfg.once {
		logger.Info("orchestrator: single reconcile pass (--once)")
		return rec.Reconcile(ctx)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Change-driven trigger (ORCH-2c 3b): a buffered, size-1 channel so a burst of
	// notifications coalesces into a single pending reconcile. The listener runs on
	// its own dedicated connection and converts Postgres NOTIFYs (and reconnects)
	// into signals; the reconcile loop reads them alongside the interval safety net.
	trigger := make(chan struct{}, 1)
	listener := orchestrator.NewListener(cfg.dsn, logger)
	go func() {
		if err := listener.Listen(ctx, trigger); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("reconcile listener stopped", slog.String("error", err.Error()))
		}
	}()

	logger.Info("orchestrator: starting reconcile loop",
		slog.Duration("interval", cfg.interval), slog.String("trigger_channel", orchestrator.ReconcileChannel))
	err = rec.Run(ctx, cfg.interval, trigger)
	if errors.Is(err, context.Canceled) {
		logger.Info("orchestrator: shutting down cleanly")
		return nil
	}
	return err
}
