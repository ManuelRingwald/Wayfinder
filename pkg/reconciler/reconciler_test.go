package reconciler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/instance"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeDesired is a swappable desired-state source. specs and err are read under
// mu so a test can change them between reconcile passes (simulating catalogue
// changes) safely while Run reconciles concurrently.
type fakeDesired struct {
	mu    sync.Mutex
	specs []instance.Spec
	err   error
	calls int
}

func (d *fakeDesired) DesiredSpecs(_ context.Context) ([]instance.Spec, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	return d.specs, d.err
}

func (d *fakeDesired) set(specs []instance.Spec) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.specs = specs
}

func spec(id int64, port int) instance.Spec {
	return instance.Spec{FeedID: id, FeedName: "f", Group: "239.0.0.1", Port: port}
}

func TestReconcileStartsDesired(t *testing.T) {
	ctx := context.Background()
	desired := &fakeDesired{specs: []instance.Spec{spec(1, 8600), spec(2, 8601)}}
	backend := instance.NewMemoryBackend()
	r := New(desired, backend, discardLogger())

	if err := r.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	for _, id := range []int64{1, 2} {
		if st, _ := backend.Status(ctx, id); st != instance.StatusRunning {
			t.Errorf("feed %d status = %q, want running", id, st)
		}
	}
}

func TestReconcileStopsOrphans(t *testing.T) {
	ctx := context.Background()
	backend := instance.NewMemoryBackend()
	// Pre-existing instances 1, 2, 3 are running.
	for _, id := range []int64{1, 2, 3} {
		_ = backend.Start(ctx, spec(id, 8600))
	}
	// Desired state now only wants feed 2.
	desired := &fakeDesired{specs: []instance.Spec{spec(2, 8600)}}
	r := New(desired, backend, discardLogger())

	if err := r.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if st, _ := backend.Status(ctx, 2); st != instance.StatusRunning {
		t.Error("desired feed 2 should still run")
	}
	for _, id := range []int64{1, 3} {
		if st, _ := backend.Status(ctx, id); st != instance.StatusStopped {
			t.Errorf("orphan feed %d status = %q, want stopped", id, st)
		}
	}
}

func TestReconcileIsIdempotent(t *testing.T) {
	ctx := context.Background()
	desired := &fakeDesired{specs: []instance.Spec{spec(1, 8600)}}
	backend := instance.NewMemoryBackend()
	r := New(desired, backend, discardLogger())

	for i := 0; i < 3; i++ {
		if err := r.Reconcile(ctx); err != nil {
			t.Fatalf("reconcile %d: %v", i, err)
		}
	}
	feeds, _ := backend.RunningFeeds(ctx)
	if len(feeds) != 1 || feeds[0] != 1 {
		t.Fatalf("running feeds = %v, want exactly [1]", feeds)
	}
}

func TestReconcileReappliesChangedSpec(t *testing.T) {
	ctx := context.Background()
	desired := &fakeDesired{specs: []instance.Spec{spec(1, 8600)}}
	backend := instance.NewMemoryBackend()
	r := New(desired, backend, discardLogger())
	_ = r.Reconcile(ctx)

	// The feed's endpoint changes; the next reconcile must re-apply it.
	desired.set([]instance.Spec{spec(1, 8700)})
	if err := r.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got, ok := backend.RunningSpec(1)
	if !ok || got.Port != 8700 {
		t.Fatalf("spec not re-applied: %+v ok=%v", got, ok)
	}
}

func TestReconcileRecoversCrashedInstance(t *testing.T) {
	ctx := context.Background()
	desired := &fakeDesired{specs: []instance.Spec{spec(1, 8600)}}
	backend := instance.NewMemoryBackend()
	r := New(desired, backend, discardLogger())
	_ = r.Reconcile(ctx)

	// Simulate a crash: the instance vanishes from the backend out of band.
	_ = backend.Stop(ctx, 1)
	if st, _ := backend.Status(ctx, 1); st != instance.StatusStopped {
		t.Fatal("precondition: feed 1 should be stopped after simulated crash")
	}

	// The next reconcile re-derives desired and restarts it.
	if err := r.Reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if st, _ := backend.Status(ctx, 1); st != instance.StatusRunning {
		t.Fatalf("crashed instance not recovered: %q", st)
	}
}

func TestReconcileDesiredStateErrorAborts(t *testing.T) {
	desired := &fakeDesired{err: errors.New("db down")}
	r := New(desired, instance.NewMemoryBackend(), discardLogger())
	if err := r.Reconcile(context.Background()); err == nil {
		t.Fatal("reconcile should fail when desired state cannot be read")
	}
}

func TestReconcilePerFeedErrorDoesNotAbort(t *testing.T) {
	ctx := context.Background()
	// Feed 2's start fails; feeds 1 and 3 must still be started.
	backend := instance.NewMemoryBackend().WithStartHook(func(s instance.Spec) error {
		if s.FeedID == 2 {
			return errors.New("backend rejects feed 2")
		}
		return nil
	})
	desired := &fakeDesired{specs: []instance.Spec{spec(1, 8600), spec(2, 8601), spec(3, 8602)}}
	r := New(desired, backend, discardLogger())

	err := r.Reconcile(ctx)
	if err == nil {
		t.Fatal("reconcile should return the per-feed error")
	}
	for _, id := range []int64{1, 3} {
		if st, _ := backend.Status(ctx, id); st != instance.StatusRunning {
			t.Errorf("feed %d should run despite feed 2 failing; got %q", id, st)
		}
	}
	if st, _ := backend.Status(ctx, 2); st != instance.StatusFailed {
		t.Errorf("feed 2 status = %q, want failed", st)
	}
}

func TestRunReconcilesUntilCancelled(t *testing.T) {
	desired := &fakeDesired{specs: []instance.Spec{spec(1, 8600)}}
	backend := instance.NewMemoryBackend()
	r := New(desired, backend, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx, time.Millisecond) }()

	// Wait until at least the initial reconcile + a tick have happened.
	deadline := time.After(2 * time.Second)
	for {
		desired.mu.Lock()
		calls := desired.calls
		desired.mu.Unlock()
		if calls >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Run did not reconcile repeatedly")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want context.Canceled", err)
	}
	if st, _ := backend.Status(context.Background(), 1); st != instance.StatusRunning {
		t.Error("feed 1 should be running after Run")
	}
}
