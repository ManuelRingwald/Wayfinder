package broadcast

import (
	"context"
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
	client := b.RegisterClient(sendChan)

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

	b.trackChan <- []cat062.DecodedTrack{track}

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
		clients[i] = b.RegisterClient(sendChans[i])
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
	b.trackChan <- []cat062.DecodedTrack{{TrackNum: 1}}

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
	b.RegisterClient(sendChan)

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

	tracks := []cat062.DecodedTrack{track}
	msg := b.tracksToMessage(tracks)

	if len(msg.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(msg.Tracks))
	}

	tm := msg.Tracks[0]
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
