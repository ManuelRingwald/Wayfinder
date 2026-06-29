// Package reconciler keeps the set of running Firefly instances in sync with the
// set the catalogue says should be running (ORCH-3, ADR 0012 §5).
//
// It is the cloud-native operator pattern: rather than imperatively starting an
// instance when a feed gains a subscription and stopping it on the last
// unsubscribe, the reconciler periodically (and on demand) computes the *desired*
// state — the specs of all feeds that should be running — and drives the
// instance.Backend toward it. This is idempotent and crash-safe: on restart it
// re-derives desired from the catalogue and observes the backend's actual running
// set, correcting any drift (a crashed instance is restarted, an orphan with no
// feed is stopped). Instance identity is the feed id.
//
// The reconciler depends only on a DesiredState source and an instance.Backend,
// both injected, so it is unit-testable without a database or a real container
// runtime (the MemoryBackend stands in).
package reconciler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/instance"
)

// DesiredState yields the specs of every feed that should currently have a
// running instance — i.e. feeds with at least one active subscription, each
// derived from its source configuration (ADR 0012 §5). Satisfied in production by
// an adapter over the store (feeds ⨝ subscriptions + source config, wired in the
// control plane, ORCH-2c); faked in tests.
type DesiredState interface {
	DesiredSpecs(ctx context.Context) ([]instance.Spec, error)
}

// Reconciler drives an instance.Backend toward the desired state.
type Reconciler struct {
	desired DesiredState
	backend instance.Backend
	logger  *slog.Logger
}

// New creates a Reconciler. desired supplies the target specs; backend is driven
// toward them; logger records reconcile actions and per-feed errors.
func New(desired DesiredState, backend instance.Backend, logger *slog.Logger) *Reconciler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reconciler{desired: desired, backend: backend, logger: logger}
}

// Reconcile performs one pass toward the desired state and returns once it has
// attempted every action. It is idempotent:
//
//   - every desired spec is Start-ed (the backend is idempotent: a no-op when the
//     instance already runs with an equal spec, a re-apply when the spec changed —
//     this also recovers a crashed/failed instance);
//   - every running instance whose feed is no longer desired is Stop-ped (orphan
//     cleanup).
//
// A per-feed failure does not abort the pass: the reconciler logs it, keeps going,
// and returns the joined error so the caller (and the next tick) can react. This
// keeps one broken feed from blocking every other feed's convergence.
func (r *Reconciler) Reconcile(ctx context.Context) error {
	specs, err := r.desired.DesiredSpecs(ctx)
	if err != nil {
		return fmt.Errorf("reconciler: desired state: %w", err)
	}

	desired := make(map[int64]struct{}, len(specs))
	var errs []error

	// Converge toward desired: start/re-apply every spec that should run.
	for _, spec := range specs {
		desired[spec.FeedID] = struct{}{}
		if err := r.backend.Start(ctx, spec); err != nil {
			errs = append(errs, fmt.Errorf("start feed %d: %w", spec.FeedID, err))
			r.logger.Error("reconcile: start instance failed",
				slog.Int64("feed_id", spec.FeedID), slog.String("error", err.Error()))
		}
	}

	// Orphan cleanup: stop any running instance whose feed is no longer desired.
	running, err := r.backend.RunningFeeds(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("list running: %w", err))
		return errors.Join(errs...)
	}
	for _, id := range running {
		if _, want := desired[id]; want {
			continue
		}
		if err := r.backend.Stop(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("stop orphan feed %d: %w", id, err))
			r.logger.Error("reconcile: stop orphan failed",
				slog.Int64("feed_id", id), slog.String("error", err.Error()))
			continue
		}
		r.logger.Info("reconcile: stopped orphan instance", slog.Int64("feed_id", id))
	}

	return errors.Join(errs...)
}

// Run reconciles once immediately, then on every tick of interval until ctx is
// cancelled. A reconcile error is logged (Reconcile already logs per-feed detail)
// and the loop continues — transient failures self-heal on the next tick. Run
// blocks until ctx is done and returns ctx.Err().
func (r *Reconciler) Run(ctx context.Context, interval time.Duration) error {
	if err := r.Reconcile(ctx); err != nil {
		r.logger.Warn("initial reconcile had errors", slog.String("error", err.Error()))
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil {
				r.logger.Warn("reconcile had errors", slog.String("error", err.Error()))
			}
		}
	}
}
