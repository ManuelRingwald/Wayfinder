package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReconcileChannel is the Postgres LISTEN/NOTIFY channel the database triggers
// (migration 00012) signal on every change to the desired-state tables (feeds,
// subscriptions). It is a fixed identifier, not user input.
const ReconcileChannel = "wayfinder_reconcile"

// listenBackoff is the wait before reconnecting after a listen-connection loss,
// so a flapping database does not spin the reconnect loop.
const listenBackoff = 2 * time.Second

// Listener turns Postgres change notifications into reconcile signals (ORCH-2c 3b,
// ADR 0012 §5). It holds a dedicated connection (LISTEN binds to one session, so a
// pooled connection is unsuitable) and emits a signal on every notification — and
// once after every (re)connect, to resync any change missed while disconnected.
//
// It lives only in the orchestrator control plane: it is the fast path that makes
// the reconciler converge the instant a feed or subscription changes, instead of
// waiting up to one reconcile interval. The interval loop remains the safety net.
type Listener struct {
	dsn    string
	logger *slog.Logger
}

// NewListener builds a Listener over the given database DSN.
func NewListener(dsn string, logger *slog.Logger) *Listener {
	if logger == nil {
		logger = slog.Default()
	}
	return &Listener{dsn: dsn, logger: logger}
}

// Listen connects, LISTENs on ReconcileChannel and emits a signal on trigger for
// every notification, reconnecting with a fixed backoff on connection loss. After
// each (re)connect it emits one signal unconditionally, so a change that happened
// during a connection gap still drives a reconcile (the reconciler recomputes the
// full desired state, so an extra signal is always safe). Sends are non-blocking
// against the buffered trigger channel, so a burst of notifications coalesces into
// a single pending reconcile. Listen blocks until ctx is cancelled and returns
// ctx.Err(); transient connect/listen errors are logged and retried.
func (l *Listener) Listen(ctx context.Context, trigger chan<- struct{}) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := l.listenOnce(ctx, trigger); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			l.logger.Warn("reconcile listener connection lost; reconnecting",
				slog.String("error", err.Error()), slog.Duration("backoff", listenBackoff))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(listenBackoff):
			}
		}
	}
}

// listenOnce opens a dedicated connection, LISTENs, emits a resync signal and then
// relays every notification as a signal until the connection drops or ctx ends. It
// returns a non-nil error on connection trouble (the caller backs off and retries)
// and ctx.Err() on cancellation.
func (l *Listener) listenOnce(ctx context.Context, trigger chan<- struct{}) error {
	conn, err := pgx.Connect(ctx, l.dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	// ReconcileChannel is a fixed constant identifier, so direct interpolation is
	// safe (no user input reaches this string).
	if _, err := conn.Exec(ctx, "LISTEN "+ReconcileChannel); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	l.logger.Info("reconcile listener connected", slog.String("channel", ReconcileChannel))

	// Resync on (re)connect: a change may have happened before LISTEN was in place
	// (first start) or during a reconnect gap. One signal makes the reconciler
	// recompute the full desired state, closing that window.
	signalReconcile(trigger)

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait for notification: %w", err)
		}
		_ = notification // payload is empty by design; the signal is what matters
		signalReconcile(trigger)
	}
}

// signalReconcile performs a non-blocking send: if a signal is already pending in
// the buffered channel, this one is dropped (a reconcile is already queued, and it
// will read the full current state anyway). This coalesces bursts into one pass.
func signalReconcile(trigger chan<- struct{}) {
	select {
	case trigger <- struct{}{}:
	default:
	}
}
