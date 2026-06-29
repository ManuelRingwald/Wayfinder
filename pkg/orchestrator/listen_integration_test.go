package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// TestIntegrationListenerSignalsOnChange verifies the end-to-end change-driven
// path (ORCH-2c 3b): migration 00012's triggers NOTIFY on feeds/subscriptions
// changes, and the Listener relays each as a reconcile signal — plus one resync
// signal on connect. Requires a real PostgreSQL via WAYFINDER_TEST_DB_URL.
func TestIntegrationListenerSignalsOnChange(t *testing.T) {
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run listener integration tests")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	trigger := make(chan struct{}, 1)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	listener := NewListener(dsn, logger)
	go func() { _ = listener.Listen(ctx, trigger) }()

	// 1. Resync signal on connect.
	waitForSignal(t, trigger, "resync on connect")

	// 2. A feed INSERT fires the feeds trigger → a signal.
	feeds := store.NewFeedRepo(pool)
	f, err := feeds.Create(ctx, "listen-test", "239.255.0.91", 8901, nil, nil)
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	waitForSignal(t, trigger, "feed insert")

	// 3. A subscription INSERT fires the subscriptions trigger → a signal.
	tenants := store.NewTenantRepo(pool)
	ten, err := tenants.Create(ctx, "listen-tenant", "Listen Tenant")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	// The tenant create also touches no desired-state table; drain any signal the
	// preceding feed/other writes may still have queued before the real assertion.
	drainSignal(trigger)
	if err := store.NewSubscriptionRepo(pool).Subscribe(ctx, ten.ID, f.ID); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	waitForSignal(t, trigger, "subscription insert")

	// 4. A feed DELETE fires the feeds trigger → a signal (cascades the sub away).
	drainSignal(trigger)
	if err := feeds.Delete(ctx, f.ID); err != nil {
		t.Fatalf("delete feed: %v", err)
	}
	waitForSignal(t, trigger, "feed delete")
}

func waitForSignal(t *testing.T, trigger <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-trigger:
	case <-time.After(5 * time.Second):
		t.Fatalf("no reconcile signal within deadline (%s)", what)
	}
}

func drainSignal(trigger <-chan struct{}) {
	select {
	case <-trigger:
	default:
	}
}
