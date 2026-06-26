package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/feedmanager"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

func TestResolveFeedsFallbackWhenCatalogueEmpty(t *testing.T) {
	cfg := Config{FeedID: 0, MulticastGroup: "239.255.0.62", MulticastPort: 8600}

	for name, catalogue := range map[string][]store.Feed{
		"nil catalogue":   nil,
		"empty catalogue": {},
	} {
		t.Run(name, func(t *testing.T) {
			feeds := resolveFeeds(catalogue, cfg)
			if len(feeds) != 1 {
				t.Fatalf("want 1 fallback feed, got %d", len(feeds))
			}
			if feeds[0] != (feedConfig{ID: 0, Name: "default", Group: "239.255.0.62", Port: 8600}) {
				t.Fatalf("fallback feed = %+v", feeds[0])
			}
		})
	}
}

func TestResolveFeedsFromCatalogue(t *testing.T) {
	catalogue := []store.Feed{
		{ID: 1, Name: "Frankfurt", MulticastGroup: "239.255.0.62", Port: 8600},
		{ID: 2, Name: "Stuttgart", MulticastGroup: "239.255.0.63", Port: 8601},
	}
	feeds := resolveFeeds(catalogue, Config{FeedID: 99, MulticastGroup: "x", MulticastPort: 1})

	if len(feeds) != 2 {
		t.Fatalf("want 2 feeds, got %d", len(feeds))
	}
	want := []feedConfig{
		{ID: 1, Name: "Frankfurt", Group: "239.255.0.62", Port: 8600},
		{ID: 2, Name: "Stuttgart", Group: "239.255.0.63", Port: 8601},
	}
	for i := range want {
		if feeds[i] != want[i] {
			t.Errorf("feed[%d] = %+v, want %+v (ENV fallback must not leak in)", i, feeds[i], want[i])
		}
	}
}

func TestNewReceiverFactory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	factory := newReceiverFactory(logger, nil, nil, nil, nil)

	// A valid feed builds a receiver (the manager later Listens/Runs it).
	r, err := factory(feedmanager.Feed{ID: 1, Name: "a", Group: "239.255.0.62", Port: 8600})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if r == nil {
		t.Fatal("factory returned a nil receiver")
	}

	// An invalid multicast group is a build error (surfaced to Start).
	if _, err := factory(feedmanager.Feed{ID: 7, Name: "bad", Group: "not-an-ip", Port: 8600}); err == nil {
		t.Fatal("expected error for invalid multicast group, got nil")
	}
}
