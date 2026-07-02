package receiver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/cat065"
)

// TestReceiverLoopback tests the receiver with a loopback UDP sender.
func TestReceiverLoopback(t *testing.T) {
	// Track decoded blocks.
	var decodedBlocks int
	var decodedTracks int

	recv, err := New(Config{
		Group:  "127.0.0.1", // Loopback
		Port:   0,           // Ephemeral port
		Logger: slog.New(slog.NewTextHandler(nil, nil)),
		Handler: func(_ int64, tracks []cat062.DecodedTrack) error {
			decodedBlocks++
			decodedTracks += len(tracks)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = recv.Close() }()

	// Listen on the ephemeral port.
	if err := recv.Listen(); err != nil {
		t.Skipf("Listen failed: %v", err)
	}

	// Full loopback test requires exposing the port or mock transport.
	t.Skip("full loopback test pending: requires exposing receiver port or mock transport")
}

// TestReceiverConfigDefaults verifies configuration defaults.
func TestReceiverConfigDefaults(t *testing.T) {
	recv, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = recv.Close() }()

	if recv.group.String() != "239.255.0.62" {
		t.Errorf("group: got %s, want 239.255.0.62", recv.group.String())
	}
	if recv.port != 8600 {
		t.Errorf("port: got %d, want 8600", recv.port)
	}
}

// TestReceiverDecodeErrorCountStartsAtZero verifies that a fresh receiver
// reports no decode errors yet (REQ NFR-OBS-002, exposed via /metrics).
func TestReceiverDecodeErrorCountStartsAtZero(t *testing.T) {
	recv, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = recv.Close() }()

	if got := recv.DecodeErrorCount(); got != 0 {
		t.Errorf("DecodeErrorCount: got %d, want 0", got)
	}
}

// TestDispatchRoutesByCategory verifies that the receiver routes a CAT062 block
// to the track handler, a CAT065 block to the status handler, and an unknown
// category to the decode-error counter (Firefly ADR 0018, shared multicast feed).
func TestDispatchRoutesByCategory(t *testing.T) {
	var gotTracks int
	var gotStatus int
	recv, err := New(Config{
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Handler:       func(_ int64, tracks []cat062.DecodedTrack) error { gotTracks += len(tracks); return nil },
		StatusHandler: func(cat065.ServiceStatus) error { gotStatus++; return nil },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	remote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}

	// A CAT065 heartbeat (Firefly's byte-exact reference dump) → status handler.
	heartbeat := []byte{0x41, 0x00, 0x0C, 0xF4, 0x19, 0x02, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00}
	recv.dispatch(heartbeat, remote)
	if gotStatus != 1 {
		t.Errorf("CAT065: status handler called %d times, want 1", gotStatus)
	}
	if recv.DecodeErrorCount() != 0 {
		t.Errorf("CAT065: unexpected decode errors %d", recv.DecodeErrorCount())
	}

	// An unknown category → decode error, no handler.
	recv.dispatch([]byte{0x22, 0x00, 0x03}, remote)
	if recv.DecodeErrorCount() != 1 {
		t.Errorf("unknown CAT: decode errors %d, want 1", recv.DecodeErrorCount())
	}
	if gotTracks != 0 || gotStatus != 1 {
		t.Errorf("unknown CAT must not call handlers (tracks=%d status=%d)", gotTracks, gotStatus)
	}
}

// TestHandleTracksStampsFeedID verifies the receiver's configured FeedID is
// passed to the track handler for every decoded block, so downstream the track
// can be attributed to its feed (WF2-20).
func TestHandleTracksStampsFeedID(t *testing.T) {
	var gotFeedID int64 = -1
	var gotTracks int
	recv, err := New(Config{
		FeedID: 42,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Handler: func(feedID int64, tracks []cat062.DecodedTrack) error {
			gotFeedID = feedID
			gotTracks = len(tracks)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Minimal valid CAT062 block: CAT + LEN + FSPEC(FRN1) + I062/010 → 1 track.
	block := []byte{0x3E, 0x00, 0x06, 0x80, 0x19, 0x02}
	recv.dispatch(block, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234})

	if gotFeedID != 42 {
		t.Errorf("handler feed_id = %d, want 42", gotFeedID)
	}
	if gotTracks != 1 {
		t.Errorf("handler tracks = %d, want 1", gotTracks)
	}
}

// TestReceiverInvalidGroup verifies error on invalid multicast group.
func TestReceiverInvalidGroup(t *testing.T) {
	_, err := New(Config{
		Group: "not-an-ip",
	})
	if err == nil {
		t.Errorf("New with invalid group: expected error, got nil")
	}
}

// TestReceiverRunWithoutListen verifies that Run() fails if Listen() wasn't called.
func TestReceiverRunWithoutListen(t *testing.T) {
	recv, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = recv.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = recv.Run(ctx)
	if err == nil {
		t.Errorf("Run without Listen: expected error, got nil")
	}
}

// TestReceiverListenAndClose verifies basic listen/close cycle.
func TestReceiverListenAndClose(t *testing.T) {
	recv, err := New(Config{
		Group: "239.255.0.62",
		Port:  0, // Ephemeral, but multicast group binding may fail on some systems
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = recv.Close() }()

	// Listen may fail on systems without multicast support.
	if err := recv.Listen(); err != nil {
		t.Skipf("Listen failed (multicast not available on this system): %v", err)
	}

	// Should close without error.
	if err := recv.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestReceiverContextCancellation verifies that Run() respects context cancellation.
func TestReceiverContextCancellation(t *testing.T) {
	recv, err := New(Config{
		Group: "127.0.0.1",
		Port:  0,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = recv.Close() }()

	if err := recv.Listen(); err != nil {
		t.Skipf("Listen failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run should exit due to context cancellation.
	err = recv.Run(ctx)
	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Errorf("Run with cancelled context: got %v, want context.DeadlineExceeded or context.Canceled", err)
	}
}
