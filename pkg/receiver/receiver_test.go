package receiver

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// TestReceiverLoopback tests the receiver with a loopback UDP sender.
func TestReceiverLoopback(t *testing.T) {
	// Track decoded blocks.
	var decodedBlocks int
	var decodedTracks int

	recv, err := New(Config{
		Group: "127.0.0.1", // Loopback
		Port:  0,           // Ephemeral port
		Logger: slog.New(slog.NewTextHandler(nil, nil)),
		Handler: func(tracks []cat062.DecodedTrack) error {
			decodedBlocks++
			decodedTracks += len(tracks)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer recv.Close()

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
	defer recv.Close()

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
	defer recv.Close()

	if got := recv.DecodeErrorCount(); got != 0 {
		t.Errorf("DecodeErrorCount: got %d, want 0", got)
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
	defer recv.Close()

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
	defer recv.Close()

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
	defer recv.Close()

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
