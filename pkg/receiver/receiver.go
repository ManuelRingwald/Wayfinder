package receiver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/cat065"
)

// Receiver listens on a UDP-Multicast socket for CAT062 data blocks.
// Each datagram = one complete CAT062 block (CAT + LEN + Records).
type Receiver struct {
	feedID        int64
	group         net.IP
	port          int
	conn          *net.UDPConn
	logger        *slog.Logger
	handler       func(feedID int64, tracks []cat062.DecodedTrack) error
	statusHandler func(status cat065.ServiceStatus) error

	decodeErrors atomic.Int64
}

// Config holds receiver configuration.
type Config struct {
	// FeedID identifies which feed this receiver consumes (the feeds.id of the
	// catalogue entry, WF2-20). It is stamped onto every decoded track so the
	// scoped fan-out (WF2-21) can filter per tenant subscription. Zero in the
	// single-tenant / single-feed fallback.
	FeedID int64
	Group  string // Multicast group, default "239.255.0.62"
	Port   int    // Port, default 8600
	Logger *slog.Logger
	// Handler receives decoded CAT062 track blocks, attributed to FeedID.
	Handler func(feedID int64, tracks []cat062.DecodedTrack) error
	// StatusHandler receives decoded CAT065 SDPS heartbeats (Firefly ADR 0018).
	// Optional; a nil handler means heartbeats are decoded and logged but
	// otherwise ignored.
	StatusHandler func(status cat065.ServiceStatus) error
}

// New creates a new Receiver with the given configuration.
func New(cfg Config) (*Receiver, error) {
	if cfg.Group == "" {
		cfg.Group = "239.255.0.62"
	}
	if cfg.Port == 0 {
		cfg.Port = 8600
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Handler == nil {
		cfg.Handler = func(_ int64, _ []cat062.DecodedTrack) error { return nil }
	}
	if cfg.StatusHandler == nil {
		cfg.StatusHandler = func(_ cat065.ServiceStatus) error { return nil }
	}

	group := net.ParseIP(cfg.Group)
	if group == nil {
		return nil, fmt.Errorf("invalid multicast group: %s", cfg.Group)
	}

	return &Receiver{
		feedID:        cfg.FeedID,
		group:         group,
		port:          cfg.Port,
		logger:        cfg.Logger,
		handler:       cfg.Handler,
		statusHandler: cfg.StatusHandler,
	}, nil
}

// Listen opens the UDP-Multicast socket and joins the multicast group.
// It does NOT start the receive loop; call Run() for that.
func (r *Receiver) Listen() error {
	groupAddr := &net.UDPAddr{
		Port: r.port,
		IP:   r.group,
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, groupAddr)
	if err != nil {
		return fmt.Errorf("listen multicast: %w", err)
	}

	r.conn = conn
	r.logger.Info("listening on multicast",
		slog.String("group", r.group.String()),
		slog.Int("port", r.port))
	return nil
}

// Run starts the receive loop. It blocks until context is cancelled or an error occurs.
// Malformed blocks are logged and skipped; errors from the handler propagate.
func (r *Receiver) Run(ctx context.Context) error {
	if r.conn == nil {
		return fmt.Errorf("not listening; call Listen() first")
	}

	defer r.conn.Close()

	buffer := make([]byte, 65535) // Max UDP datagram size

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, remoteAddr, err := r.conn.ReadFromUDP(buffer)
		if err != nil {
			return fmt.Errorf("read multicast: %w", err)
		}
		if n == 0 {
			continue
		}
		r.dispatch(buffer[:n], remoteAddr)
	}
}

// dispatch routes one datagram by its leading ASTERIX CAT octet. The multicast
// stream carries multiple categories on one group (Firefly ADR 0018): 0x3E is a
// CAT062 track block, 0x41 a CAT065 SDPS-status heartbeat. Unknown categories
// are counted as decode errors and skipped.
func (r *Receiver) dispatch(data []byte, remote *net.UDPAddr) {
	switch data[0] {
	case 0x3E: // CAT062 — system tracks
		r.handleTracks(data, remote)
	case cat065.Category: // CAT065 — SDPS service status (heartbeat)
		r.handleStatus(data, remote)
	default:
		r.decodeErrors.Add(1)
		r.logger.Error("unknown ASTERIX category",
			slog.String("remote", remote.String()),
			slog.Int("cat", int(data[0])),
			slog.Int("bytes", len(data)))
	}
}

// handleTracks decodes a CAT062 block and forwards the tracks to the handler.
func (r *Receiver) handleTracks(data []byte, remote *net.UDPAddr) {
	tracks, err := cat062.DecodeDataBlock(data)
	if err != nil {
		r.decodeErrors.Add(1)
		r.logger.Error("decode CAT062 block",
			slog.String("remote", remote.String()),
			slog.Int("bytes", len(data)),
			slog.String("error", err.Error()))
		return
	}
	if len(tracks) == 0 {
		r.logger.Debug("empty CAT062 block", slog.String("remote", remote.String()))
		return
	}
	r.logger.Debug("decoded CAT062 block",
		slog.String("remote", remote.String()),
		slog.Int64("feed_id", r.feedID),
		slog.Int("tracks", len(tracks)))
	if err := r.handler(r.feedID, tracks); err != nil {
		r.logger.Error("handler error",
			slog.Int("tracks", len(tracks)),
			slog.String("error", err.Error()))
	}
}

// handleStatus decodes a CAT065 heartbeat block and forwards each report to the
// status handler.
func (r *Receiver) handleStatus(data []byte, remote *net.UDPAddr) {
	reports, err := cat065.DecodeStatusBlock(data)
	if err != nil {
		r.decodeErrors.Add(1)
		r.logger.Error("decode CAT065 block",
			slog.String("remote", remote.String()),
			slog.Int("bytes", len(data)),
			slog.String("error", err.Error()))
		return
	}
	for _, status := range reports {
		r.logger.Debug("decoded CAT065 heartbeat",
			slog.String("remote", remote.String()),
			slog.Int("service_id", int(status.ServiceID)),
			slog.Bool("operational", status.Operational))
		if err := r.statusHandler(status); err != nil {
			r.logger.Error("status handler error", slog.String("error", err.Error()))
		}
	}
}

// DecodeErrorCount returns the total number of CAT062 blocks that failed to
// decode so far (REQ NFR-OBS-002, exposed via /metrics).
func (r *Receiver) DecodeErrorCount() int64 {
	return r.decodeErrors.Load()
}

// Close closes the UDP connection.
func (r *Receiver) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
