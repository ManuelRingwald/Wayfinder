// Package feedmanager supervises the live set of CAT062/065/063 multicast feed
// receivers (ONB-5, ADR 0011).
//
// Until ONB-5 the receiver set was fixed at process start: changing it meant a
// restart. Zero-touch onboarding requires adding and removing data sources from
// the admin UI while the ASD keeps running, so the Manager owns one running
// receiver per feed id and exposes Start/Stop to join/leave a multicast group at
// runtime. Each receiver runs in its own goroutine driven by a per-feed context
// derived from a shared base context; cancelling that context makes the receiver
// leave the group and release its socket promptly (the receiver's read watchdog
// guarantees this even for a dead feed).
//
// The Manager depends on an injected Factory rather than importing the concrete
// receiver, so it is unit-testable without opening real UDP sockets.
package feedmanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// Feed is the minimal descriptor the manager needs to start a receiver: the
// catalogue id (stamped onto decoded tracks for the scoped fan-out) plus the
// multicast group/port to join. Name is carried only for logging.
type Feed struct {
	ID    int64
	Name  string
	Group string
	Port  int
}

// Receiver is the lifecycle the manager drives for one feed. Listen opens the
// socket and joins the multicast group (synchronous, may fail); Run blocks the
// receive loop until its context is cancelled and then returns context.Canceled
// (or another error on a genuine socket failure); Close releases the socket.
// Satisfied by *receiver.Receiver; faked in tests.
type Receiver interface {
	Listen() error
	Run(ctx context.Context) error
	Close() error
}

// Factory builds a not-yet-listening Receiver for a feed. Injected so the manager
// is unit-testable with a fake that needs no UDP socket. A factory error aborts
// the Start without registering the feed.
type Factory func(Feed) (Receiver, error)

// Manager supervises the running receivers. Safe for concurrent use: every access
// to the running map is guarded by mu, and a goroutine never holds mu while
// blocked, so Stop/StopAll can run concurrently with a receiver's own teardown.
type Manager struct {
	base    context.Context
	factory Factory
	logger  *slog.Logger

	mu      sync.Mutex
	running map[int64]*handle
}

// handle tracks one running receiver: cancel stops its context, done is closed
// once its goroutine has fully returned (socket closed).
type handle struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a Manager. base is the parent context for every receiver: when it
// is cancelled (server shutdown) all receivers stop. factory builds a receiver
// per feed; logger records lifecycle events and fatal receiver errors.
func New(base context.Context, factory Factory, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		base:    base,
		factory: factory,
		logger:  logger,
		running: make(map[int64]*handle),
	}
}

// Start builds, joins and runs the receiver for f in a background goroutine.
// It is idempotent: starting a feed id that is already running is a no-op. A
// failure to build (factory) or to join the group (Listen) returns the error and
// leaves nothing registered, so the caller can surface it (boot: skip the feed;
// admin API: fail the create). On success the receiver is live before Start
// returns.
func (m *Manager) Start(f Feed) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.running[f.ID]; ok {
		return nil // already running — idempotent
	}

	rcv, err := m.factory(f)
	if err != nil {
		return fmt.Errorf("build receiver for feed %q (id=%d): %w", f.Name, f.ID, err)
	}
	if err := rcv.Listen(); err != nil {
		return fmt.Errorf("join feed %q (id=%d) %s:%d: %w", f.Name, f.ID, f.Group, f.Port, err)
	}

	ctx, cancel := context.WithCancel(m.base)
	done := make(chan struct{})
	m.running[f.ID] = &handle{cancel: cancel, done: done}

	go func() {
		defer close(done)
		err := rcv.Run(ctx)
		_ = rcv.Close()
		if err != nil && !errors.Is(err, context.Canceled) {
			// A genuine receiver failure (socket error), not an orchestrated stop.
			// Forget the entry so a later Start can retry this feed, and log it.
			m.forget(f.ID)
			m.logger.Error("feed receiver stopped on error",
				slog.Int64("feed_id", f.ID), slog.String("name", f.Name),
				slog.String("error", err.Error()))
		}
	}()

	m.logger.Info("feed receiver started",
		slog.Int64("feed_id", f.ID), slog.String("name", f.Name),
		slog.String("group", f.Group), slog.Int("port", f.Port))
	return nil
}

// Stop cancels the receiver for feedID and waits for it to leave the group and
// release its socket before returning. It returns true if a receiver was running,
// false if the id was unknown (already stopped or never started). Safe to call
// concurrently with the receiver's own error teardown.
func (m *Manager) Stop(feedID int64) bool {
	m.mu.Lock()
	h, ok := m.running[feedID]
	if ok {
		delete(m.running, feedID)
	}
	m.mu.Unlock()
	if !ok {
		return false
	}
	h.cancel()
	<-h.done // wait for clean multicast leave (bounded by the receiver watchdog)
	m.logger.Info("feed receiver stopped", slog.Int64("feed_id", feedID))
	return true
}

// StopAll cancels every running receiver and waits for all to release their
// sockets. Used on server shutdown. Idempotent.
func (m *Manager) StopAll() {
	m.mu.Lock()
	handles := make([]*handle, 0, len(m.running))
	for id, h := range m.running {
		handles = append(handles, h)
		delete(m.running, id)
	}
	m.mu.Unlock()
	for _, h := range handles {
		h.cancel()
	}
	for _, h := range handles {
		<-h.done
	}
}

// Running returns the ids of the currently running feeds, in no particular order.
// Used by the staleness monitor (to broadcast per-feed snapshots for the live
// set) and by tests.
func (m *Manager) Running() []int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]int64, 0, len(m.running))
	for id := range m.running {
		ids = append(ids, id)
	}
	return ids
}

// IsRunning reports whether a receiver for feedID is currently running.
func (m *Manager) IsRunning(feedID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.running[feedID]
	return ok
}

// forget removes a feed's entry without cancelling it — used by a receiver
// goroutine that has stopped on its own error. If Stop already removed the entry
// (and is waiting on done), this is a harmless no-op.
func (m *Manager) forget(feedID int64) {
	m.mu.Lock()
	delete(m.running, feedID)
	m.mu.Unlock()
}
