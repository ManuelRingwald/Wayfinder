package feedmanager

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeReceiver is a UDP-free Receiver: Run blocks until its context is cancelled
// (or, if runErr is set, returns that immediately to simulate a socket failure).
// It records the lifecycle calls so tests can assert clean start/stop.
type fakeReceiver struct {
	listenErr error
	runErr    error // when non-nil, Run returns it at once (genuine failure)

	listened atomic.Bool
	closed   atomic.Bool
	ran      atomic.Bool
}

func (f *fakeReceiver) Listen() error {
	if f.listenErr != nil {
		return f.listenErr
	}
	f.listened.Store(true)
	return nil
}

func (f *fakeReceiver) Run(ctx context.Context) error {
	f.ran.Store(true)
	if f.runErr != nil {
		return f.runErr
	}
	<-ctx.Done()
	return ctx.Err()
}

func (f *fakeReceiver) Close() error {
	f.closed.Store(true)
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestStartRunsAndStopLeaves verifies the happy path: Start listens and runs the
// receiver; Stop cancels it, waits for a clean teardown (Close), and reports it
// was running. After Stop the feed is no longer in the running set.
func TestStartRunsAndStopLeaves(t *testing.T) {
	rcv := &fakeReceiver{}
	m := New(context.Background(), func(Feed) (Receiver, error) { return rcv, nil }, testLogger())

	if err := m.Start(Feed{ID: 7, Name: "north", Group: "239.255.0.62", Port: 8600}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !m.IsRunning(7) {
		t.Fatal("feed 7 should be running after Start")
	}
	// Give the goroutine a moment to enter Run.
	waitFor(t, func() bool { return rcv.ran.Load() })
	if !rcv.listened.Load() {
		t.Error("receiver should have been Listen()ed")
	}

	if stopped := m.Stop(7); !stopped {
		t.Fatal("Stop should report the feed was running")
	}
	if m.IsRunning(7) {
		t.Error("feed 7 should not be running after Stop")
	}
	if !rcv.closed.Load() {
		t.Error("receiver should have been Close()d on stop")
	}
}

// TestStartIdempotent verifies starting the same feed id twice runs only one
// receiver (the second Start is a no-op and does not build a second one).
func TestStartIdempotent(t *testing.T) {
	var builds atomic.Int32
	m := New(context.Background(), func(Feed) (Receiver, error) {
		builds.Add(1)
		return &fakeReceiver{}, nil
	}, testLogger())

	for i := 0; i < 3; i++ {
		if err := m.Start(Feed{ID: 1}); err != nil {
			t.Fatalf("Start #%d: %v", i, err)
		}
	}
	if got := builds.Load(); got != 1 {
		t.Errorf("factory called %d times, want 1 (idempotent Start)", got)
	}
	m.StopAll()
}

// TestStartListenError surfaces a join failure and registers nothing, so the
// caller can skip/abort and a later Start can retry.
func TestStartListenError(t *testing.T) {
	rcv := &fakeReceiver{listenErr: errors.New("address in use")}
	m := New(context.Background(), func(Feed) (Receiver, error) { return rcv, nil }, testLogger())

	if err := m.Start(Feed{ID: 5}); err == nil {
		t.Fatal("Start should return the Listen error")
	}
	if m.IsRunning(5) {
		t.Error("a feed that failed to join must not be registered")
	}
}

// TestStartFactoryError surfaces a build failure and registers nothing.
func TestStartFactoryError(t *testing.T) {
	m := New(context.Background(), func(Feed) (Receiver, error) {
		return nil, errors.New("invalid multicast group")
	}, testLogger())

	if err := m.Start(Feed{ID: 9}); err == nil {
		t.Fatal("Start should return the factory error")
	}
	if m.IsRunning(9) {
		t.Error("a feed that failed to build must not be registered")
	}
}

// TestStopUnknown returns false without blocking.
func TestStopUnknown(t *testing.T) {
	m := New(context.Background(), func(Feed) (Receiver, error) { return &fakeReceiver{}, nil }, testLogger())
	if m.Stop(123) {
		t.Error("Stop of an unknown feed should return false")
	}
}

// TestReceiverSelfErrorForgets verifies that a receiver that returns a genuine
// error (not context.Canceled) is dropped from the running set on its own, so a
// later Start can retry the same id.
func TestReceiverSelfErrorForgets(t *testing.T) {
	first := &fakeReceiver{runErr: errors.New("socket exploded")}
	second := &fakeReceiver{}
	var nth atomic.Int32
	m := New(context.Background(), func(Feed) (Receiver, error) {
		if nth.Add(1) == 1 {
			return first, nil
		}
		return second, nil
	}, testLogger())

	if err := m.Start(Feed{ID: 3}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// The receiver fails immediately; wait until the manager has forgotten it.
	waitFor(t, func() bool { return !m.IsRunning(3) })

	// A retry must build a fresh receiver and run it.
	if err := m.Start(Feed{ID: 3}); err != nil {
		t.Fatalf("retry Start: %v", err)
	}
	if !m.IsRunning(3) {
		t.Error("feed 3 should be running after retry")
	}
	m.StopAll()
}

// TestStopAll stops every running receiver and clears the set.
func TestStopAll(t *testing.T) {
	var mu sync.Mutex
	var recvs []*fakeReceiver
	m := New(context.Background(), func(Feed) (Receiver, error) {
		r := &fakeReceiver{}
		mu.Lock()
		recvs = append(recvs, r)
		mu.Unlock()
		return r, nil
	}, testLogger())

	for id := int64(1); id <= 4; id++ {
		if err := m.Start(Feed{ID: id}); err != nil {
			t.Fatalf("Start %d: %v", id, err)
		}
	}
	if got := len(m.Running()); got != 4 {
		t.Fatalf("Running() = %d, want 4", got)
	}
	m.StopAll()
	if got := len(m.Running()); got != 0 {
		t.Errorf("Running() = %d after StopAll, want 0", got)
	}
	mu.Lock()
	defer mu.Unlock()
	for i, r := range recvs {
		if !r.closed.Load() {
			t.Errorf("receiver %d not Close()d by StopAll", i)
		}
	}
}

// TestBaseContextStopsAll verifies cancelling the base context stops every
// receiver without an explicit StopAll (server-shutdown path).
func TestBaseContextStopsAll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rcv := &fakeReceiver{}
	m := New(ctx, func(Feed) (Receiver, error) { return rcv, nil }, testLogger())

	if err := m.Start(Feed{ID: 1}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	cancel()
	waitFor(t, func() bool { return rcv.closed.Load() })
}

// waitFor polls cond up to ~2s, failing the test if it never becomes true.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
