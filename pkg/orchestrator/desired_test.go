package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int       { return &i }

// fakeFeeds / fakeSources are in-memory stand-ins for the subscription and feed
// repos.
type fakeFeeds struct {
	feeds []store.Feed
	err   error
}

func (f fakeFeeds) ListSubscribedFeeds(_ context.Context) ([]store.Feed, error) {
	return f.feeds, f.err
}

type fakeSources struct {
	cfg map[int64]store.SourceConfig
	cov map[int64]*store.BBox
	err error
}

func (s fakeSources) GetSourceConfig(_ context.Context, id int64) (store.SourceConfig, *store.BBox, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.cfg[id], s.cov[id], nil
}

func TestDesiredSpecsBuildsSpecsFromSubscribedFeeds(t *testing.T) {
	cov := &store.BBox{MinLat: 48, MinLon: 7, MaxLat: 50, MaxLon: 9}
	feeds := fakeFeeds{feeds: []store.Feed{
		{ID: 1, Name: "ffm", MulticastGroup: "239.0.0.1", Port: 8600},
		{ID: 2, Name: "speyer", MulticastGroup: "239.0.0.2", Port: 8601},
	}}
	sources := fakeSources{
		cfg: map[int64]store.SourceConfig{
			1: {{Type: store.SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)}},
			2: {{Type: store.SourceADSBOpenSky, BBox: cov, CredRef: ptrStr("secret/speyer")}},
		},
		cov: map[int64]*store.BBox{2: cov},
	}

	specs, err := NewStoreDesiredState(feeds, sources).DesiredSpecs(context.Background())
	if err != nil {
		t.Fatalf("DesiredSpecs: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("got %d specs, want 2", len(specs))
	}
	// Feed 1: radar, no coverage, no secrets.
	if specs[0].FeedID != 1 || specs[0].Group != "239.0.0.1" || specs[0].Port != 8600 {
		t.Fatalf("spec[0] endpoint wrong: %+v", specs[0])
	}
	if specs[0].Coverage != nil || len(specs[0].SecretRefs) != 0 {
		t.Errorf("spec[0] should have no coverage/secrets: %+v", specs[0])
	}
	// Feed 2: adsb with coverage + a secret reference (handle only).
	if specs[1].FeedID != 2 || specs[1].Coverage != cov {
		t.Fatalf("spec[1] wrong: %+v", specs[1])
	}
	if len(specs[1].SecretRefs) != 1 || specs[1].SecretRefs[0] != "secret/speyer" {
		t.Errorf("spec[1] secret refs = %v, want [secret/speyer]", specs[1].SecretRefs)
	}
}

func TestDesiredSpecsEmptyWhenNoSubscribedFeeds(t *testing.T) {
	specs, err := NewStoreDesiredState(fakeFeeds{}, fakeSources{}).DesiredSpecs(context.Background())
	if err != nil {
		t.Fatalf("DesiredSpecs: %v", err)
	}
	if len(specs) != 0 {
		t.Fatalf("got %d specs, want 0", len(specs))
	}
}

func TestDesiredSpecsListErrorAborts(t *testing.T) {
	feeds := fakeFeeds{err: errors.New("db down")}
	if _, err := NewStoreDesiredState(feeds, fakeSources{}).DesiredSpecs(context.Background()); err == nil {
		t.Fatal("DesiredSpecs should fail when the feed list cannot be read")
	}
}

func TestDesiredSpecsSourceConfigErrorAborts(t *testing.T) {
	// A failure to read one feed's source config must abort the whole pass, never
	// return a partial desired set (which would orphan instances the reconciler
	// merely failed to read).
	feeds := fakeFeeds{feeds: []store.Feed{{ID: 1, Name: "ffm", MulticastGroup: "239.0.0.1", Port: 8600}}}
	sources := fakeSources{err: errors.New("source read failed")}
	if _, err := NewStoreDesiredState(feeds, sources).DesiredSpecs(context.Background()); err == nil {
		t.Fatal("DesiredSpecs should fail when a source config cannot be read")
	}
}
