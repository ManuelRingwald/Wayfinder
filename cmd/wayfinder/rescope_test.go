package main

import (
	"context"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/broadcast"
	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// TestResolveScopeBuildsScope checks the resolution shared by the /ws connect
// path and the live re-scope (WF2-33): feed allow-set + effective view, with the
// tenant/user stamped on the scope so a re-scope can re-resolve the same user.
func TestResolveScopeBuildsScope(t *testing.T) {
	flMin, flMax := 100, 300
	views := fakeViewGetter{vc: store.ViewConfig{
		CenterLat: 50, CenterLon: 9, Zoom: 8,
		AOI:   &store.BBox{MinLat: 49, MinLon: 8, MaxLat: 51, MaxLon: 10},
		FLMin: &flMin, FLMax: &flMax,
	}}
	scope, feedIDs, view, err := resolveScope(context.Background(),
		fakeFeedLister{feeds: []int64{1, 2}}, views, 7, 3)
	if err != nil {
		t.Fatalf("resolveScope: %v", err)
	}
	if scope.TenantID != 7 || scope.UserID != 3 {
		t.Errorf("scope identity = (t%d,u%d), want (t7,u3)", scope.TenantID, scope.UserID)
	}
	if !scope.AllowsFeed(1) || !scope.AllowsFeed(2) || scope.AllowsFeed(3) {
		t.Error("feed allow-set wrong: want {1,2}")
	}
	if len(feedIDs) != 2 {
		t.Errorf("feedIDs = %v, want two", feedIDs)
	}
	if view == nil || view.AOI == nil ||
		view.FLMinFt == nil || *view.FLMinFt != 10000 ||
		view.FLMaxFt == nil || *view.FLMaxFt != 30000 {
		t.Errorf("view filter not built correctly: %+v", view)
	}
}

// TestRescopeTenantAppliesLive wires resolveScope + rescopeTenant against a real
// broadcaster (exported API only): a client subscribed to feed 1 starts seeing
// feed 2 after a live grant, with no reconnect.
func TestRescopeTenantAppliesLive(t *testing.T) {
	b := broadcast.New(discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = b.Run(ctx) }()

	sendCh := make(chan broadcast.Message, 64)
	s0, _, _, err := resolveScope(ctx, fakeFeedLister{feeds: []int64{1}}, noView, 7, 1)
	if err != nil {
		t.Fatalf("resolveScope: %v", err)
	}
	b.RegisterClient(sendCh, s0)
	for i := 0; i < 500 && b.ClientCount() != 1; i++ {
		time.Sleep(time.Millisecond)
	}

	feed2 := func(num uint16) broadcast.TrackBatch {
		return broadcast.TrackBatch{FeedID: 2, Tracks: []cat062.DecodedTrack{
			{TrackNum: num, WGS84: cat062.WGS84Position{Latitude: 50, Longitude: 9}},
		}}
	}

	// Before the grant: feed 2 is not delivered.
	b.TracksChan() <- feed2(1)
	select {
	case <-sendCh:
		t.Fatal("feed 2 must not be delivered before the grant")
	case <-time.After(100 * time.Millisecond):
	}

	// Live grant of feed 2 (subscriptions now return {1,2}).
	rescopeTenant(ctx, b, fakeFeedLister{feeds: []int64{1, 2}}, noView, discardLogger(), 7)

	// After the grant: feed 2 is delivered. Retry to absorb the async apply —
	// once the scope swap lands, every later feed-2 batch comes through.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("feed 2 not delivered after the live grant")
		default:
		}
		b.TracksChan() <- feed2(2)
		select {
		case msg := <-sendCh:
			if len(msg.Tracks) == 1 && msg.Tracks[0].FeedID == 2 {
				return // success
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
}
