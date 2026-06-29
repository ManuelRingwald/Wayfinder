package instance

import (
	"context"
	"sync"
)

// MemoryBackend is an in-memory Backend: it records the Spec it was asked to run
// per feed id and reports a lifecycle status, but spawns nothing. It is the test
// double for the control plane and reconciler (ORCH-2c/-3) and the single-host
// dev placeholder until the Docker adapter (ORCH-2b) exists. Safe for concurrent
// use.
type MemoryBackend struct {
	// startHook, when set, runs at the start of Start; a non-nil error makes the
	// instance fail (StatusFailed) without being recorded as running — used by
	// tests to exercise the failure path.
	startHook func(Spec) error

	mu      sync.Mutex
	running map[int64]Spec
	failed  map[int64]bool
}

// NewMemoryBackend returns an empty in-memory backend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		running: make(map[int64]Spec),
		failed:  make(map[int64]bool),
	}
}

// WithStartHook installs a hook invoked at the start of every Start call. A
// non-nil return marks the feed as failed and is returned to the caller. Returns
// the backend for chaining in tests.
func (b *MemoryBackend) WithStartHook(hook func(Spec) error) *MemoryBackend {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.startHook = hook
	return b
}

// Start records the spec as the running instance for its feed. It validates the
// spec first, runs the optional start hook, and is idempotent on an equal spec
// (re-applying the same config is a no-op; a changed spec replaces it). A failed
// feed that is re-started successfully clears its failed mark.
func (b *MemoryBackend) Start(_ context.Context, spec Spec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	b.mu.Lock()
	hook := b.startHook
	b.mu.Unlock()

	if hook != nil {
		if err := hook(spec); err != nil {
			b.mu.Lock()
			b.failed[spec.FeedID] = true
			delete(b.running, spec.FeedID)
			b.mu.Unlock()
			return err
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.running[spec.FeedID] = spec
	delete(b.failed, spec.FeedID)
	return nil
}

// Stop removes the running instance for feedID. Stopping an unknown feed is a
// no-op (idempotent).
func (b *MemoryBackend) Stop(_ context.Context, feedID int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.running, feedID)
	delete(b.failed, feedID)
	return nil
}

// Status reports the lifecycle state for feedID: StatusRunning if a spec is
// recorded, StatusFailed if the last Start failed, otherwise StatusStopped.
func (b *MemoryBackend) Status(_ context.Context, feedID int64) (Status, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.running[feedID]; ok {
		return StatusRunning, nil
	}
	if b.failed[feedID] {
		return StatusFailed, nil
	}
	return StatusStopped, nil
}

// RunningSpec returns the recorded spec for feedID and whether one is running.
// Test/introspection helper (the reconciler compares specs to detect drift).
func (b *MemoryBackend) RunningSpec(feedID int64) (Spec, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.running[feedID]
	return s, ok
}

// RunningFeeds returns the ids of feeds with a running instance, in no order.
// Implements instance.Backend (the reconciler's orphan-detection input).
func (b *MemoryBackend) RunningFeeds(_ context.Context) ([]int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ids := make([]int64, 0, len(b.running))
	for id := range b.running {
		ids = append(ids, id)
	}
	return ids, nil
}
