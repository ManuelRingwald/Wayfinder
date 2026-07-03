package receiver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/cat063"
	"github.com/manuelringwald/wayfinder/pkg/cat065"
)

// Receiver listens on a UDP-Multicast socket for CAT062 data blocks.
// Each datagram = one complete CAT062 block (CAT + LEN + Records).
type Receiver struct {
	feedID int64
	group  net.IP
	port   int
	conn   *net.UDPConn
	// pc wraps conn to read the per-datagram destination address (control message),
	// so a socket that is wildcard-bound (all groups on the port) can drop datagrams
	// addressed to another feed's group — see Listen/Run and acceptsGroup.
	pc                  *ipv4.PacketConn
	logger              *slog.Logger
	handler             func(feedID int64, tracks []cat062.DecodedTrack) error
	statusHandler       func(status cat065.ServiceStatus) error
	sensorStatusHandler func(statuses []cat063.SensorStatus) error
	onDecodeError       func()

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
	// SensorStatusHandler receives decoded CAT063 per-sensor status records
	// (Firefly ICD 2.5.0, ADR 0022). Optional; nil means sensor status blocks
	// are decoded and logged but otherwise ignored.
	SensorStatusHandler func(statuses []cat063.SensorStatus) error
	// OnDecodeError, if set, is called once per datagram that fails to decode or
	// carries an unknown ASTERIX category. It lets the caller keep a process-wide,
	// churn-stable decode-error counter (ONB-5): receivers come and go as feeds are
	// added/removed, so summing each receiver's own count would make the metric
	// non-monotonic. The receiver's DecodeErrorCount() is unaffected.
	OnDecodeError func()
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
	if cfg.SensorStatusHandler == nil {
		cfg.SensorStatusHandler = func(_ []cat063.SensorStatus) error { return nil }
	}

	group := net.ParseIP(cfg.Group)
	if group == nil {
		return nil, fmt.Errorf("invalid multicast group: %s", cfg.Group)
	}

	return &Receiver{
		feedID:              cfg.FeedID,
		group:               group,
		port:                cfg.Port,
		logger:              cfg.Logger,
		handler:             cfg.Handler,
		statusHandler:       cfg.StatusHandler,
		sensorStatusHandler: cfg.SensorStatusHandler,
		onDecodeError:       cfg.OnDecodeError,
	}, nil
}

// countDecodeError increments the receiver's own decode-error counter and, if
// configured, notifies the process-wide decode-error hook (ONB-5).
func (r *Receiver) countDecodeError() {
	r.decodeErrors.Add(1)
	if r.onDecodeError != nil {
		r.onDecodeError()
	}
}

// Listen opens the UDP-Multicast socket and joins the multicast group.
// It does NOT start the receive loop; call Run() for that.
//
// Per-feed isolation (security-critical). `net.ListenMulticastUDP` binds the
// socket to the wildcard address (0.0.0.0:port), not to the group — the group is
// only joined at the IGMP layer. On a single host several feeds join DISTINCT
// groups on the SAME port (the allocator varies the group, keeps the port; see
// pkg/store/feed_alloc.go), so once the host is a member of both groups, EVERY
// wildcard-bound socket on that port receives BOTH groups' datagrams — a receiver
// would otherwise ingest another feed's tracks (cross-tenant leak). We therefore
// enable the destination-address control message and drop, in Run(), any datagram
// not addressed to this receiver's group (acceptsGroup).
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
	r.pc = ipv4.NewPacketConn(conn)
	// Ask the kernel to attach the datagram's destination address to each read.
	// If the platform cannot (rare — Linux/macOS/BSD support IP_PKTINFO), we log
	// and continue: acceptsGroup then treats a missing destination as "accept",
	// falling back to the prior behaviour rather than blinding the feed.
	if err := r.pc.SetControlMessage(ipv4.FlagDst, true); err != nil {
		r.logger.Warn("per-group datagram filtering unavailable; feeds sharing a port may cross-deliver",
			slog.String("group", r.group.String()), slog.String("error", err.Error()))
	}
	r.logger.Info("listening on multicast",
		slog.String("group", r.group.String()),
		slog.Int("port", r.port))
	return nil
}

// acceptsGroup reports whether a datagram whose IPv4 destination address is dst
// belongs to this receiver's feed group. It is the per-feed isolation guard: the
// wildcard-bound socket may receive other feeds' groups on the shared port, and
// each receiver must accept ONLY its own group's datagrams. A nil dst (control
// message unavailable) is accepted — we cannot prove it foreign, and dropping it
// would blind the feed on a platform without IP_PKTINFO.
func (r *Receiver) acceptsGroup(dst net.IP) bool {
	return dst == nil || dst.Equal(r.group)
}

// Run starts the receive loop. It blocks until context is cancelled or an error occurs.
// Malformed blocks are logged and skipped; errors from the handler propagate.
//
// Prompt cancellation (ONB-5, ADR 0011): a blocked ReadFromUDP does not observe a
// context cancellation on its own, so the live feed manager could not promptly
// leave the multicast group of a *dead* feed (no datagram to unblock the read).
// A watchdog goroutine sets a past read deadline the moment ctx is done, which
// makes the in-flight read return immediately with a deadline error; we then see
// ctx.Err() and stop cleanly. This is the idiomatic way to interrupt a blocked
// UDP read and guarantees the socket (and its IGMP group membership) is released
// at once.
func (r *Receiver) Run(ctx context.Context) error {
	if r.conn == nil {
		return fmt.Errorf("not listening; call Listen() first")
	}

	defer func() { _ = r.conn.Close() }()

	// Watchdog: unblock a read in progress as soon as the context is cancelled.
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		select {
		case <-ctx.Done():
			// A deadline in the past makes the current/next read return at once
			// with os.ErrDeadlineExceeded; the loop then observes ctx.Done().
			_ = r.pc.SetReadDeadline(time.Now())
		case <-stopWatch:
		}
	}()

	buffer := make([]byte, 65535) // Max UDP datagram size

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read via the ipv4.PacketConn so the control message carries the datagram's
		// destination group (see Listen): a wildcard-bound socket receives every
		// group joined on this port, and we must keep only our own feed's group.
		n, cm, src, err := r.pc.ReadFrom(buffer)
		if err != nil {
			// A read interrupted by the cancellation watchdog (deadline exceeded)
			// is a clean stop, not a feed error — report the context cause.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, os.ErrDeadlineExceeded) {
				// Defensive: a deadline fired without cancellation (should not
				// happen, as we only ever set it from the watchdog). Clear it and
				// continue rather than treating it as a fatal feed error.
				_ = r.pc.SetReadDeadline(time.Time{})
				continue
			}
			return fmt.Errorf("read multicast: %w", err)
		}
		if n == 0 {
			continue
		}
		// Drop datagrams addressed to another feed's group that reached this
		// wildcard-bound socket on the shared port (feed isolation).
		var dst net.IP
		if cm != nil {
			dst = cm.Dst
		}
		if !r.acceptsGroup(dst) {
			continue
		}
		remoteAddr, _ := src.(*net.UDPAddr)
		r.dispatch(buffer[:n], remoteAddr)
	}
}

// dispatch routes one datagram by its leading ASTERIX CAT octet. The multicast
// stream carries three categories on one group (Firefly ICD 2.5.0): 0x3E is a
// CAT062 track block, 0x41 a CAT065 SDPS-status heartbeat, 0x3F a CAT063
// per-sensor status block. Unknown categories are counted as decode errors and
// skipped (robust decoder, CLAUDE.md §7).
func (r *Receiver) dispatch(data []byte, remote *net.UDPAddr) {
	switch data[0] {
	case 0x3E: // CAT062 — system tracks
		r.handleTracks(data, remote)
	case cat065.Category: // 0x41 CAT065 — SDPS service status (heartbeat)
		r.handleStatus(data, remote)
	case cat063.Category: // 0x3F CAT063 — per-sensor status (ADR 0022)
		r.handleSensorStatus(data, remote)
	default:
		r.countDecodeError()
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
		r.countDecodeError()
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
		r.countDecodeError()
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

// handleSensorStatus decodes a CAT063 per-sensor status block and forwards
// the statuses to the sensor status handler.
func (r *Receiver) handleSensorStatus(data []byte, remote *net.UDPAddr) {
	statuses, err := cat063.DecodeSensorBlock(data)
	if err != nil {
		r.countDecodeError()
		r.logger.Error("decode CAT063 block",
			slog.String("remote", remote.String()),
			slog.Int("bytes", len(data)),
			slog.String("error", err.Error()))
		return
	}
	if len(statuses) == 0 {
		r.logger.Debug("empty CAT063 block", slog.String("remote", remote.String()))
		return
	}
	active := 0
	for _, s := range statuses {
		if s.Operational {
			active++
		}
	}
	r.logger.Debug("decoded CAT063 block",
		slog.String("remote", remote.String()),
		slog.Int64("feed_id", r.feedID),
		slog.Int("sensors_total", len(statuses)),
		slog.Int("sensors_active", active))
	if err := r.sensorStatusHandler(statuses); err != nil {
		r.logger.Error("sensor status handler error", slog.String("error", err.Error()))
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
