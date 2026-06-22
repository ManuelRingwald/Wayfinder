package broadcast

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// --- WF2-33 live re-scope tests --------------------------------------------
//
// These exercise the actor-model re-scope: a client's scope is swapped live, on
// the Run goroutine, without a reconnect. The broadcaster being a single
// goroutine is what makes this race-free; TestRescopeRaceUnderLoad proves it
// under -race.

func discardBroadcaster() *Broadcaster {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func scopeFor(tenantID, userID int64, feeds []int64, view *ViewFilter) *Scope {
	s := NewScopeWithView(feeds, view)
	s.TenantID = tenantID
	s.UserID = userID
	return s
}

func trackAt(num uint16, lat, lon float64) cat062.DecodedTrack {
	return cat062.DecodedTrack{TrackNum: num, WGS84: cat062.WGS84Position{Latitude: lat, Longitude: lon}}
}

func feedBatch(feedID int64, tracks ...cat062.DecodedTrack) TrackBatch {
	return TrackBatch{FeedID: feedID, Tracks: tracks}
}

func waitClients(t *testing.T, b *Broadcaster, n int) {
	t.Helper()
	for i := 0; i < 500 && b.ClientCount() != n; i++ {
		time.Sleep(time.Millisecond)
	}
	if got := b.ClientCount(); got != n {
		t.Fatalf("client count = %d, want %d", got, n)
	}
}

// waitRescopeApplied blocks until the Run loop has dequeued the pending re-scope
// command. Because Run completes the current case (applyScopes) before returning
// to select, once the channel is drained any *subsequent* track batch is
// guaranteed to be evaluated under the new scope — making the tests deterministic
// despite the random select between rescopeChan and trackChan.
func waitRescopeApplied(t *testing.T, b *Broadcaster) {
	t.Helper()
	for i := 0; i < 500 && len(b.rescopeChan) > 0; i++ {
		time.Sleep(time.Millisecond)
	}
	if len(b.rescopeChan) > 0 {
		t.Fatal("re-scope command was not consumed by the Run loop")
	}
}

func expectTrack(t *testing.T, ch <-chan Message, trackNum uint16) {
	t.Helper()
	select {
	case msg := <-ch:
		if len(msg.Tracks) != 1 || msg.Tracks[0].TrackNum != trackNum {
			t.Fatalf("got %+v, want single track %d", msg.Tracks, trackNum)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout waiting for track %d", trackNum)
	}
}

func expectNoTrack(t *testing.T, ch <-chan Message) {
	t.Helper()
	select {
	case msg := <-ch:
		t.Fatalf("unexpected delivery: %+v", msg.Tracks)
	case <-time.After(100 * time.Millisecond):
	}
}

// expectPurge asserts that the next message on ch is an empty-tracks frame sent
// by applyScopes after a feed revoke (NFR-SEC-003). Call this before
// expectNoTrack/expectTrack when a revoke was applied.
func expectPurge(t *testing.T, ch <-chan Message) {
	t.Helper()
	select {
	case msg := <-ch:
		if len(msg.Tracks) != 0 {
			t.Fatalf("expected empty-tracks purge after feed revoke, got tracks: %+v", msg.Tracks)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout: no purge frame received after feed revoke")
	}
}

func TestClientsForTenantSnapshot(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go b.Run(ctx)

	b.RegisterClient(make(chan Message, 10), scopeFor(7, 1, []int64{1}, nil))
	b.RegisterClient(make(chan Message, 10), scopeFor(7, 2, []int64{1}, nil))
	b.RegisterClient(make(chan Message, 10), scopeFor(9, 3, []int64{1}, nil))
	b.RegisterClient(make(chan Message, 10), nil) // unscoped / single-tenant
	waitClients(t, b, 4)

	refs7 := b.ClientsForTenant(7)
	if len(refs7) != 2 {
		t.Fatalf("tenant 7: got %d refs, want 2", len(refs7))
	}
	users := map[int64]bool{}
	for _, r := range refs7 {
		users[r.UserID] = true
	}
	if !users[1] || !users[2] {
		t.Errorf("tenant 7 users = %v, want {1,2}", users)
	}
	if got := len(b.ClientsForTenant(9)); got != 1 {
		t.Errorf("tenant 9: got %d refs, want 1", got)
	}
	if refs := b.ClientsForTenant(0); refs != nil {
		t.Errorf("tenant 0 (unattributed) must never be re-scoped, got %v", refs)
	}
}

// TestApplyScopesShrinkAOILive is the headline WF2-33 case: an admin shrinks the
// AOI on a live connection; a track that was inside is no longer delivered, while
// the connection stays up (a track inside the new AOI still arrives) — no
// reconnect, and no explicit delete is sent (the frontend coasts the dropped one).
func TestApplyScopesShrinkAOILive(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go b.Run(ctx)

	ch := make(chan Message, 10)
	big := &ViewFilter{AOI: &BBox{MinLat: 40, MinLon: 0, MaxLat: 60, MaxLon: 20}}
	c := b.RegisterClient(ch, scopeFor(7, 1, []int64{1}, big))
	waitClients(t, b, 1)

	// A track at (45,5) is inside the big AOI → delivered.
	b.trackChan <- feedBatch(1, trackAt(1, 45, 5))
	expectTrack(t, ch, 1)

	// Live shrink: AOI now only around (50,9), excluding (45,5).
	small := &ViewFilter{AOI: &BBox{MinLat: 49.9, MinLon: 8.9, MaxLat: 50.1, MaxLon: 9.1}}
	if err := b.ApplyScopes(ctx, map[*Client]*Scope{c: scopeFor(7, 1, []int64{1}, small)}); err != nil {
		t.Fatalf("ApplyScopes: %v", err)
	}
	waitRescopeApplied(t, b)

	// Same outside track is now silently dropped (no delete signal).
	b.trackChan <- feedBatch(1, trackAt(1, 45, 5))
	expectNoTrack(t, ch)
	// The connection is alive: a track inside the new AOI is still delivered.
	b.trackChan <- feedBatch(1, trackAt(2, 50, 9))
	expectTrack(t, ch, 2)
}

// TestApplyScopesGrantAndRevokeFeedLive proves a feed grant/revoke takes effect
// live: a feed the client could not see becomes visible after a grant, and a feed
// it could see disappears after a revoke — no reconnect.
func TestApplyScopesGrantAndRevokeFeedLive(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go b.Run(ctx)

	ch := make(chan Message, 10)
	c := b.RegisterClient(ch, scopeFor(7, 1, []int64{1}, nil)) // feed 1 only
	waitClients(t, b, 1)

	// Feed 2 not subscribed → nothing.
	b.trackChan <- feedBatch(2, trackAt(1, 50, 9))
	expectNoTrack(t, ch)

	// Grant feed 2 (now feeds 1+2).
	if err := b.ApplyScopes(ctx, map[*Client]*Scope{c: scopeFor(7, 1, []int64{1, 2}, nil)}); err != nil {
		t.Fatalf("ApplyScopes grant: %v", err)
	}
	waitRescopeApplied(t, b)
	b.trackChan <- feedBatch(2, trackAt(2, 50, 9))
	expectTrack(t, ch, 2)

	// Revoke back to feed 1 only → applyScopes sends a purge to clear feed-2
	// tracks that are still on the client's screen (NFR-SEC-003).
	if err := b.ApplyScopes(ctx, map[*Client]*Scope{c: scopeFor(7, 1, []int64{1}, nil)}); err != nil {
		t.Fatalf("ApplyScopes revoke: %v", err)
	}
	waitRescopeApplied(t, b)
	expectPurge(t, ch) // drain the NFR-SEC-003 purge frame
	b.trackChan <- feedBatch(2, trackAt(3, 50, 9))
	expectNoTrack(t, ch)
}

// TestApplyScopesOnlyTargetClients verifies a re-scope touches exactly the named
// clients and leaves others (even of another tenant) untouched.
func TestApplyScopesOnlyTargetClients(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go b.Run(ctx)

	chA := make(chan Message, 10)
	chB := make(chan Message, 10)
	cA := b.RegisterClient(chA, scopeFor(7, 1, []int64{1}, nil))
	b.RegisterClient(chB, scopeFor(9, 3, []int64{1}, nil))
	waitClients(t, b, 2)

	// Re-scope only A: move it to feed 2 (feed 1 revoked → purge sent to A).
	if err := b.ApplyScopes(ctx, map[*Client]*Scope{cA: scopeFor(7, 1, []int64{2}, nil)}); err != nil {
		t.Fatalf("ApplyScopes: %v", err)
	}
	waitRescopeApplied(t, b)
	expectPurge(t, chA) // drain the NFR-SEC-003 purge frame for A

	// A feed-1 track now reaches only B (A was moved off feed 1, B untouched).
	b.trackChan <- feedBatch(1, trackAt(1, 50, 9))
	expectNoTrack(t, chA)
	expectTrack(t, chB, 1)
	// A feed-2 track reaches only A.
	b.trackChan <- feedBatch(2, trackAt(2, 50, 9))
	expectTrack(t, chA, 2)
	expectNoTrack(t, chB)
}

// TestApplyScopesSkipsUnknownClient confirms a stale/never-registered handle in
// the batch is ignored (the Load guard), so a disconnect racing a re-scope can
// never resurrect a client or panic.
func TestApplyScopesSkipsUnknownClient(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go b.Run(ctx)

	ch := make(chan Message, 10)
	c := b.RegisterClient(ch, scopeFor(7, 1, []int64{1}, nil))
	waitClients(t, b, 1)

	stale := &Client{} // never registered
	if err := b.ApplyScopes(ctx, map[*Client]*Scope{
		stale: scopeFor(7, 9, []int64{2}, nil),
		c:     scopeFor(7, 1, []int64{2}, nil), // feed 1 → feed 2 = revoke
	}); err != nil {
		t.Fatalf("ApplyScopes: %v", err)
	}
	waitRescopeApplied(t, b)
	expectPurge(t, ch) // drain the NFR-SEC-003 purge (feed 1 revoked from c)

	// Real client picked up its new scope; the stale handle was harmlessly skipped.
	b.trackChan <- feedBatch(2, trackAt(1, 50, 9))
	expectTrack(t, ch, 1)
}

// TestApplyScopesPurgesOnFeedRevoke is the NFR-SEC-003 regression test: when a
// feed subscription is revoked, applyScopes must immediately send an empty-tracks
// frame to the affected client so the frontend clears stale tracks without waiting
// for the next batch — which will never arrive (scope is already updated).
func TestApplyScopesPurgesOnFeedRevoke(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go b.Run(ctx)

	ch := make(chan Message, 10)
	c := b.RegisterClient(ch, scopeFor(7, 1, []int64{1}, nil))
	waitClients(t, b, 1)

	// Establish a track in the client's view.
	b.trackChan <- feedBatch(1, trackAt(10, 50, 9))
	expectTrack(t, ch, 10)

	// Revoke feed 1 → purge frame must arrive before any next batch.
	if err := b.ApplyScopes(ctx, map[*Client]*Scope{c: scopeFor(7, 1, []int64{}, nil)}); err != nil {
		t.Fatalf("ApplyScopes: %v", err)
	}
	waitRescopeApplied(t, b)
	expectPurge(t, ch)

	// Feed-1 tracks are no longer delivered (scope is now empty).
	b.trackChan <- feedBatch(1, trackAt(10, 50, 9))
	expectNoTrack(t, ch)
}

// TestApplyScopesPurgeNotSentOnGrant verifies that a pure feed grant (no revoke)
// does not produce a spurious purge frame — the client has no stale tracks from
// the newly allowed feed, so clearing would be wrong.
func TestApplyScopesPurgeNotSentOnGrant(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go b.Run(ctx)

	ch := make(chan Message, 10)
	c := b.RegisterClient(ch, scopeFor(7, 1, []int64{1}, nil))
	waitClients(t, b, 1)

	// Grant feed 2 (feed 1 remains) — no revoke, no purge.
	if err := b.ApplyScopes(ctx, map[*Client]*Scope{c: scopeFor(7, 1, []int64{1, 2}, nil)}); err != nil {
		t.Fatalf("ApplyScopes: %v", err)
	}
	waitRescopeApplied(t, b)
	// No purge expected — no tracks were revoked.
	b.trackChan <- feedBatch(2, trackAt(5, 50, 9))
	expectTrack(t, ch, 5) // feed 2 track arrives directly, no purge drained first
}

// TestApplyScopesPurgeNotSentOnAOIShrink verifies that a pure AOI shrink (feed
// set unchanged) does not produce a purge — dropped-AOI tracks coast out via the
// frontend's own mechanism (WF2-33 design decision).
func TestApplyScopesPurgeNotSentOnAOIShrink(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go b.Run(ctx)

	ch := make(chan Message, 10)
	big := &ViewFilter{AOI: &BBox{MinLat: 40, MinLon: 0, MaxLat: 60, MaxLon: 20}}
	c := b.RegisterClient(ch, scopeFor(7, 1, []int64{1}, big))
	waitClients(t, b, 1)

	b.trackChan <- feedBatch(1, trackAt(1, 45, 5))
	expectTrack(t, ch, 1)

	// Shrink AOI only — feed set unchanged → no purge.
	small := &ViewFilter{AOI: &BBox{MinLat: 49.9, MinLon: 8.9, MaxLat: 50.1, MaxLon: 9.1}}
	if err := b.ApplyScopes(ctx, map[*Client]*Scope{c: scopeFor(7, 1, []int64{1}, small)}); err != nil {
		t.Fatalf("ApplyScopes: %v", err)
	}
	waitRescopeApplied(t, b)
	// No purge — next in-AOI track arrives cleanly.
	b.trackChan <- feedBatch(1, trackAt(2, 50, 9))
	expectTrack(t, ch, 2)
}

// TestRescopeRaceUnderLoad drives concurrent track batches and re-scopes through
// the broadcaster. Run with -race it proves the actor model is race-free: the
// scope swap and the track evaluation share one goroutine, so they never collide.
func TestRescopeRaceUnderLoad(t *testing.T) {
	b := discardBroadcaster()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	const nClients = 6
	clients := make([]*Client, nClients)
	chans := make([]chan Message, nClients)
	for i := range clients {
		chans[i] = make(chan Message, 64)
		clients[i] = b.RegisterClient(chans[i], scopeFor(7, int64(i+1), []int64{1, 2}, nil))
	}
	waitClients(t, b, nClients)

	// Drainers keep the send channels from filling (and exit when closed).
	drainCtx, drainCancel := context.WithCancel(context.Background())
	var dw sync.WaitGroup
	for _, ch := range chans {
		dw.Add(1)
		go func(ch chan Message) {
			defer dw.Done()
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
				case <-drainCtx.Done():
					return
				}
			}
		}(ch)
	}

	var w sync.WaitGroup
	w.Add(2)
	go func() { // producer
		defer w.Done()
		for i := 0; i < 3000; i++ {
			select {
			case b.trackChan <- feedBatch(int64(1+i%3), trackAt(uint16(i), 50, 9)):
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { // rescoper
		defer w.Done()
		for i := 0; i < 800; i++ {
			m := make(map[*Client]*Scope, nClients)
			for j, c := range clients {
				var view *ViewFilter
				if i%2 == 0 {
					view = &ViewFilter{AOI: &BBox{MinLat: 40, MinLon: 0, MaxLat: 60, MaxLon: 20}}
				}
				m[c] = scopeFor(7, int64(j+1), []int64{1, 2}, view)
			}
			if err := b.ApplyScopes(ctx, m); err != nil {
				return
			}
		}
	}()

	w.Wait()
	cancel()      // stop Run → closes client channels → drainers see ok=false
	drainCancel() // belt-and-suspenders
	dw.Wait()
}
