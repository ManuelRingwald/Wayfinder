package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// TestIntegrationFeedCatalogue exercises the feed CLI and resolveFeeds against a
// real Postgres: `feed add` inserts rows, `feed list` shows them, and resolveFeeds
// maps the catalogue to one feedConfig per row (feed_id = the DB id that
// subscriptions reference). Skips without WAYFINDER_TEST_DB_URL.
func TestIntegrationFeedCatalogue(t *testing.T) {
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run the feed catalogue integration test")
	}
	t.Setenv("WAYFINDER_DB_URL", dsn) // the feed CLI reads WAYFINDER_DB_URL

	ctx := context.Background()
	pool, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE feeds RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// Empty catalogue → resolveFeeds falls back to the single ENV feed.
	empty, err := store.NewFeedRepo(pool).List(ctx)
	if err != nil {
		t.Fatalf("list (empty): %v", err)
	}
	if feeds := resolveFeeds(empty, Config{MulticastGroup: "239.255.0.62", MulticastPort: 8600}); len(feeds) != 1 || feeds[0].Name != "default" {
		t.Fatalf("empty catalogue should fall back to 1 default feed, got %+v", feeds)
	}

	// `feed add` twice.
	var out bytes.Buffer
	if err := feedAddCommand([]string{"-name", "Frankfurt", "-group", "239.255.0.62", "-port", "8600", "-sensor-mix", "PSR,SSR,ADS-B"}, &out); err != nil {
		t.Fatalf("feed add 1: %v", err)
	}
	if err := feedAddCommand([]string{"-name", "Stuttgart", "-group", "239.255.0.63", "-port", "8601"}, &out); err != nil {
		t.Fatalf("feed add 2: %v", err)
	}

	// `feed list` shows both.
	out.Reset()
	if err := feedListCommand(nil, &out); err != nil {
		t.Fatalf("feed list: %v", err)
	}
	for _, want := range []string{"Frankfurt", "Stuttgart", "239.255.0.63"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("feed list output missing %q:\n%s", want, out.String())
		}
	}

	// resolveFeeds maps the catalogue to one feedConfig per row, in id order.
	catalogue, err := store.NewFeedRepo(pool).List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	feeds := resolveFeeds(catalogue, Config{})
	if len(feeds) != 2 {
		t.Fatalf("want 2 feeds, got %d", len(feeds))
	}
	if feeds[0].Name != "Frankfurt" || feeds[0].Group != "239.255.0.62" || feeds[0].Port != 8600 {
		t.Errorf("feed[0] = %+v", feeds[0])
	}
	if feeds[1].Name != "Stuttgart" || feeds[1].Port != 8601 {
		t.Errorf("feed[1] = %+v", feeds[1])
	}
	// feed_id must be the distinct non-zero DB id (what subscriptions reference).
	if feeds[0].ID == 0 || feeds[1].ID == 0 || feeds[0].ID == feeds[1].ID {
		t.Errorf("feed ids should be distinct non-zero DB ids: %d, %d", feeds[0].ID, feeds[1].ID)
	}
}
