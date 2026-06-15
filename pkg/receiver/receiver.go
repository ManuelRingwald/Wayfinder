package receiver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// Receiver listens on a UDP-Multicast socket for CAT062 data blocks.
// Each datagram = one complete CAT062 block (CAT + LEN + Records).
type Receiver struct {
	group   net.IP
	port    int
	conn    *net.UDPConn
	logger  *slog.Logger
	handler func(tracks []cat062.DecodedTrack) error

	decodeErrors atomic.Int64
}

// Config holds receiver configuration.
type Config struct {
	Group   string // Multicast group, default "239.255.0.62"
	Port    int    // Port, default 8600
	Logger  *slog.Logger
	Handler func(tracks []cat062.DecodedTrack) error
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
		cfg.Handler = func(_ []cat062.DecodedTrack) error { return nil }
	}

	group := net.ParseIP(cfg.Group)
	if group == nil {
		return nil, fmt.Errorf("invalid multicast group: %s", cfg.Group)
	}

	return &Receiver{
		group:   group,
		port:    cfg.Port,
		logger:  cfg.Logger,
		handler: cfg.Handler,
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

		// Decode the CAT062 block.
		tracks, err := cat062.DecodeDataBlock(buffer[:n])
		if err != nil {
			r.decodeErrors.Add(1)
			r.logger.Error("decode CAT062 block",
				slog.String("remote", remoteAddr.String()),
				slog.Int("bytes", n),
				slog.String("error", err.Error()))
			continue
		}

		if len(tracks) == 0 {
			r.logger.Debug("empty CAT062 block", slog.String("remote", remoteAddr.String()))
			continue
		}

		r.logger.Debug("decoded CAT062 block",
			slog.String("remote", remoteAddr.String()),
			slog.Int("tracks", len(tracks)))

		// Pass tracks to the handler.
		if err := r.handler(tracks); err != nil {
			r.logger.Error("handler error",
				slog.Int("tracks", len(tracks)),
				slog.String("error", err.Error()))
			// Don't return; keep listening for more blocks.
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
