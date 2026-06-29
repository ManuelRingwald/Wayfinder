package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Multicast endpoint allocation (ORCH-4, ADR 0012). Each feed gets its OWN
// multicast group so the network layer prunes per feed (IGMP joins are per group,
// not per port): a receiver only sees datagrams of the groups it joined, which
// keeps the wire-level isolation tight (defense-in-depth under NFR-SEC-003). The
// allocator therefore varies the group's last octet within a configured /24 and
// keeps the port fixed, assigning the lowest free group to each new feed.

// ErrPoolExhausted is returned when no free multicast group remains in the pool.
var ErrPoolExhausted = errors.New("store: multicast endpoint pool exhausted")

// ErrEndpointTaken is returned when an explicit (group, port) is already in use by
// another feed (the feeds_endpoint_unique constraint, migration 00013). The admin
// API surfaces it as 409.
var ErrEndpointTaken = errors.New("store: multicast endpoint already in use")

// MulticastPool describes the address space the allocator draws feed endpoints
// from: one group per feed, varying the last octet of Base24 within [OctetMin,
// OctetMax] at the fixed Port. Base24 is the first three octets of an IPv4
// multicast /24 (e.g. "239.255.0").
type MulticastPool struct {
	Base24   string // first three octets of the /24, e.g. "239.255.0"
	OctetMin int    // lowest assignable last octet (>= 0)
	OctetMax int    // highest assignable last octet (<= 255)
	Port     int    // fixed UDP port for every allocated group
}

// DefaultMulticastPool is the out-of-the-box pool: 239.255.0.1 .. 239.255.0.254
// on port 8600 (~254 feeds, each on its own group). It deliberately skips .0 and
// .255 by convention.
var DefaultMulticastPool = MulticastPool{Base24: "239.255.0", OctetMin: 1, OctetMax: 254, Port: 8600}

// Validate checks the pool is a usable IPv4 multicast /24 with a sane octet range
// and port, so a misconfiguration fails loudly at wiring time, not on first use.
func (p MulticastPool) Validate() error {
	octets := strings.Split(p.Base24, ".")
	if len(octets) != 3 {
		return fmt.Errorf("multicast pool: Base24 %q must be three octets (e.g. 239.255.0)", p.Base24)
	}
	for _, o := range octets {
		n, err := strconv.Atoi(o)
		if err != nil || n < 0 || n > 255 {
			return fmt.Errorf("multicast pool: Base24 %q has an invalid octet", p.Base24)
		}
	}
	if !p.group(p.OctetMin).IsMulticast() {
		return fmt.Errorf("multicast pool: %s is not in the IPv4 multicast range (224.0.0.0–239.255.255.255)", p.group(p.OctetMin))
	}
	if p.OctetMin < 0 || p.OctetMax > 255 || p.OctetMin > p.OctetMax {
		return fmt.Errorf("multicast pool: octet range [%d,%d] invalid", p.OctetMin, p.OctetMax)
	}
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("multicast pool: port %d out of range", p.Port)
	}
	return nil
}

// group renders the multicast group address for a last octet.
func (p MulticastPool) group(octet int) net.IP {
	return net.ParseIP(fmt.Sprintf("%s.%d", p.Base24, octet))
}

// WithMulticastPool returns a copy of the repo that auto-allocates feed endpoints
// from the given pool (CreateAutoAllocated). An invalid pool falls back to the
// default so the server never wires an unusable allocator silently — callers that
// want strict validation should call pool.Validate() first.
func (r *FeedRepo) WithMulticastPool(p MulticastPool) *FeedRepo {
	if err := p.Validate(); err != nil {
		p = DefaultMulticastPool
	}
	clone := *r
	clone.pool = p
	return &clone
}

// CreateAutoAllocated inserts a feed on the lowest free multicast group in the
// repo's pool (one group per feed, fixed port). It is race-safe: it reads the
// taken octets, picks the lowest free one and inserts; if a concurrent create won
// that endpoint (feeds_endpoint_unique → ErrEndpointTaken) it re-reads and retries
// the next free octet. ErrPoolExhausted is returned when the pool has no free
// group. The pool defaults to DefaultMulticastPool when unset.
func (r *FeedRepo) CreateAutoAllocated(ctx context.Context, name string, region *string, sensorMix []string) (Feed, error) {
	pool := r.pool
	if pool.Base24 == "" {
		pool = DefaultMulticastPool
	}
	// Bound the attempts to the pool size (+1): every iteration either succeeds or
	// eliminates at least one taken octet from contention, so this terminates.
	for attempt := 0; attempt <= pool.OctetMax-pool.OctetMin+1; attempt++ {
		octet, err := r.lowestFreeOctet(ctx, pool)
		if err != nil {
			return Feed{}, err
		}
		f, err := r.Create(ctx, name, pool.group(octet).String(), pool.Port, region, sensorMix)
		if errors.Is(err, ErrEndpointTaken) {
			continue // a concurrent create grabbed this octet; re-read and retry
		}
		if err != nil {
			return Feed{}, err
		}
		return f, nil
	}
	return Feed{}, ErrPoolExhausted
}

// lowestFreeOctet returns the smallest last octet in the pool's range that no feed
// currently uses at the pool's port, or ErrPoolExhausted when the range is full.
func (r *FeedRepo) lowestFreeOctet(ctx context.Context, pool MulticastPool) (int, error) {
	const q = `SELECT multicast_group FROM feeds WHERE port = $1`
	rows, err := r.db.Query(ctx, q, pool.Port)
	if err != nil {
		return 0, wrap("list feed endpoints", err)
	}
	defer rows.Close()

	taken := make(map[int]struct{})
	prefix := pool.Base24 + "."
	for rows.Next() {
		var group string
		if err := rows.Scan(&group); err != nil {
			return 0, wrap("scan feed endpoint", err)
		}
		// Only octets within this pool's /24 occupy a slot; endpoints on other
		// groups (e.g. legacy manual entries) don't shrink the pool.
		if !strings.HasPrefix(group, prefix) {
			continue
		}
		if octet, err := strconv.Atoi(strings.TrimPrefix(group, prefix)); err == nil {
			taken[octet] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, wrap("iterate feed endpoints", err)
	}

	for octet := pool.OctetMin; octet <= pool.OctetMax; octet++ {
		if _, used := taken[octet]; !used {
			return octet, nil
		}
	}
	return 0, ErrPoolExhausted
}

// isEndpointUnique reports whether err is the feeds_endpoint_unique violation
// (Postgres 23505 on that constraint), so callers can map it to ErrEndpointTaken.
func isEndpointUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "feeds_endpoint_unique"
}
