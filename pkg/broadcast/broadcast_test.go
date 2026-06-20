package broadcast

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// TestBroadcasterBasic tests basic track broadcasting.
func TestBroadcasterBasic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	// Register a client.
	sendChan := make(chan Message, 10)
	client := b.RegisterClient(sendChan, nil)

	// Run broadcaster in background.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go b.Run(ctx)

	// Give broadcaster time to start.
	time.Sleep(10 * time.Millisecond)

	// Send a track.
	track := cat062.DecodedTrack{
		TrackNum: 42,
		Source:   cat062.DataSourceID{SAC: 0x19, SIC: 0x02},
		WGS84:    cat062.WGS84Position{Latitude: 45.0, Longitude: 11.25},
		Velocity: cat062.Velocity{Vx: 100.0, Vy: -50.0},
	}

	b.trackChan <- TrackBatch{Tracks: []cat062.DecodedTrack{track}}

	// Receive broadcast.
	select {
	case msg := <-sendChan:
		if len(msg.Tracks) != 1 {
			t.Errorf("expected 1 track, got %d", len(msg.Tracks))
		}
		if msg.Tracks[0].TrackNum != 42 {
			t.Errorf("track_num: expected 42, got %d", msg.Tracks[0].TrackNum)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast")
	}

	// Unregister client.
	b.UnregisterClient(client)

	// Give broadcaster time to unregister.
	time.Sleep(10 * time.Millisecond)

	if b.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", b.ClientCount())
	}
}

// TestBroadcasterMultipleClients tests broadcasting to multiple clients.
func TestBroadcasterMultipleClients(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	// Register 3 clients.
	clients := make([]*Client, 3)
	sendChans := make([]chan Message, 3)
	for i := 0; i < 3; i++ {
		sendChans[i] = make(chan Message, 10)
		clients[i] = b.RegisterClient(sendChans[i], nil)
	}

	// Run broadcaster.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go b.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	if b.ClientCount() != 3 {
		t.Fatalf("expected 3 clients, got %d", b.ClientCount())
	}

	// Send a track.
	b.trackChan <- TrackBatch{Tracks: []cat062.DecodedTrack{{TrackNum: 1}}}

	// All clients should receive it.
	for i := 0; i < 3; i++ {
		select {
		case msg := <-sendChans[i]:
			if len(msg.Tracks) != 1 {
				t.Errorf("client %d: expected 1 track, got %d", i, len(msg.Tracks))
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("client %d: timeout waiting for broadcast", i)
		}
	}
}

// TestBroadcasterSend tests the Send() method.
func TestBroadcasterSend(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	sendChan := make(chan Message, 10)
	b.RegisterClient(sendChan, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go b.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	// Send a message directly.
	msg := Message{Tracks: []TrackMessage{{TrackNum: 99}}}
	if err := b.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Receive it.
	select {
	case received := <-sendChan:
		if len(received.Tracks) != 1 || received.Tracks[0].TrackNum != 99 {
			t.Errorf("message mismatch")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout")
	}
}

// TestTracksToMessage tests track conversion to message format.
func TestTracksToMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	track := cat062.DecodedTrack{
		TrackNum:  42,
		Source:    cat062.DataSourceID{SAC: 0x19, SIC: 0x02},
		WGS84:     cat062.WGS84Position{Latitude: 45.0, Longitude: 11.25},
		Velocity:  cat062.Velocity{Vx: 100.0, Vy: -50.0},
		Cartesian: cat062.CartesianPosition{X: 1000.0, Y: -500.0},
	}

	msg := b.tracksToMessage(TrackBatch{FeedID: 7, Tracks: []cat062.DecodedTrack{track}})

	if len(msg.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(msg.Tracks))
	}

	tm := msg.Tracks[0]
	if tm.FeedID != 7 {
		t.Errorf("FeedID: got %d, want 7 (batch feed stamped onto track)", tm.FeedID)
	}
	if tm.TrackNum != 42 {
		t.Errorf("TrackNum: got %d, want 42", tm.TrackNum)
	}
	if tm.SAC != 0x19 {
		t.Errorf("SAC: got 0x%02X, want 0x19", tm.SAC)
	}
	if tm.SIC != 0x02 {
		t.Errorf("SIC: got 0x%02X, want 0x02", tm.SIC)
	}
	if tm.Latitude < 44.999 || tm.Latitude > 45.001 {
		t.Errorf("Latitude: got %f, want ~45.0", tm.Latitude)
	}
	if tm.CartX < 999.9 || tm.CartX > 1000.1 {
		t.Errorf("CartX: got %f, want ~1000.0", tm.CartX)
	}
}

// TestTracksToMessageMapsAdsbAge verifies the I062/290 ES age (ADS-B, ICD
// 2.4.0) is carried through to the wire as adsb_age_s, and that a radar-only
// track leaves it nil (so the frontend shows no ADS-B badge). AP9.9.
func TestTracksToMessageMapsAdsbAge(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	esAge := 3.0
	adsbTrack := cat062.DecodedTrack{TrackNum: 1, UpdateAge: cat062.UpdateAge{PSRAge: 2.0, ESAge: &esAge}}
	radarTrack := cat062.DecodedTrack{TrackNum: 2, UpdateAge: cat062.UpdateAge{PSRAge: 2.0}}

	msg := b.tracksToMessage(TrackBatch{Tracks: []cat062.DecodedTrack{adsbTrack, radarTrack}})
	if len(msg.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(msg.Tracks))
	}

	if msg.Tracks[0].AdsbAgeS == nil {
		t.Fatalf("ADS-B track: AdsbAgeS got nil, want ~3.0")
	}
	if *msg.Tracks[0].AdsbAgeS < 2.99 || *msg.Tracks[0].AdsbAgeS > 3.01 {
		t.Errorf("ADS-B track: AdsbAgeS got %f, want ~3.0", *msg.Tracks[0].AdsbAgeS)
	}
	if msg.Tracks[1].AdsbAgeS != nil {
		t.Errorf("radar-only track: AdsbAgeS got %v, want nil", *msg.Tracks[1].AdsbAgeS)
	}
}

// TestScopeAllowsFeed covers the feed-level scope predicate: a nil scope sees
// everything (single-tenant), a built scope sees only its feeds, and an empty
// scope sees nothing (fail-closed).
func TestScopeAllowsFeed(t *testing.T) {
	var unscoped *Scope // nil
	if !unscoped.AllowsFeed(1) || !unscoped.AllowsFeed(999) {
		t.Error("nil scope must allow every feed (single-tenant passthrough)")
	}

	s := NewScope([]int64{1, 3})
	if !s.AllowsFeed(1) || !s.AllowsFeed(3) {
		t.Error("scope must allow its own feeds")
	}
	if s.AllowsFeed(2) {
		t.Error("scope must reject a feed it was not granted")
	}

	if NewScope(nil).AllowsFeed(1) {
		t.Error("empty scope must allow nothing (fail-closed)")
	}
}

// TestBroadcastFeedIsolation is the mandatory cross-tenant negative test (WF2-21,
// NFR-SEC-003): a client scoped to feed 1 must NEVER receive a track from feed 2,
// and vice versa. Two clients with disjoint scopes prove the boundary.
func TestBroadcastFeedIsolation(t *testing.T) {
	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go b.Run(ctx)

	chanA := make(chan Message, 10)
	chanB := make(chan Message, 10)
	b.RegisterClient(chanA, NewScope([]int64{1})) // tenant A: only feed 1
	b.RegisterClient(chanB, NewScope([]int64{2})) // tenant B: only feed 2
	for i := 0; i < 100 && b.ClientCount() != 2; i++ {
		time.Sleep(time.Millisecond)
	}

	// A track on feed 1 → only A.
	b.trackChan <- TrackBatch{FeedID: 1, Tracks: []cat062.DecodedTrack{{TrackNum: 11}}}
	select {
	case msg := <-chanA:
		if len(msg.Tracks) != 1 || msg.Tracks[0].FeedID != 1 {
			t.Fatalf("A: got %+v, want one feed-1 track", msg.Tracks)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("A: timeout — should have received its own feed's track")
	}
	select {
	case msg := <-chanB:
		t.Fatalf("ISOLATION BREACH: B received a feed-1 track: %+v", msg.Tracks)
	case <-time.After(100 * time.Millisecond):
		// expected: B sees nothing from feed 1
	}

	// A track on feed 2 → only B.
	b.trackChan <- TrackBatch{FeedID: 2, Tracks: []cat062.DecodedTrack{{TrackNum: 22}}}
	select {
	case msg := <-chanB:
		if len(msg.Tracks) != 1 || msg.Tracks[0].FeedID != 2 {
			t.Fatalf("B: got %+v, want one feed-2 track", msg.Tracks)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("B: timeout — should have received its own feed's track")
	}
	select {
	case msg := <-chanA:
		t.Fatalf("ISOLATION BREACH: A received a feed-2 track: %+v", msg.Tracks)
	case <-time.After(100 * time.Millisecond):
		// expected: A sees nothing from feed 2
	}
}

// TestBroadcastEvictsClientWithFullSendChannel verifies that a client whose
// send channel is full (i.e., not being drained) is evicted instead of
// blocking the broadcaster.
func TestBroadcastEvictsClientWithFullSendChannel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := New(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go b.Run(ctx)

	// Unbuffered channel that nobody reads from, so the first broadcast fills it.
	sendChan := make(chan Message)
	b.RegisterClient(sendChan, nil)

	// Wait for registration to be processed.
	for i := 0; i < 100 && b.ClientCount() != 1; i++ {
		time.Sleep(time.Millisecond)
	}
	if b.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", b.ClientCount())
	}

	if err := b.Send(Message{Tracks: []TrackMessage{{TrackNum: 1}}}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Wait for the broadcaster to evict the unresponsive client.
	for i := 0; i < 100 && b.ClientCount() != 0; i++ {
		time.Sleep(time.Millisecond)
	}
	if b.ClientCount() != 0 {
		t.Errorf("expected client to be evicted, got %d clients", b.ClientCount())
	}
	if got := b.EvictedCount(); got != 1 {
		t.Errorf("EvictedCount: got %d, want 1", got)
	}
}
