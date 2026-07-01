// Package dockerbackend implements instance.Backend by running each feed's
// Firefly tracker as a Docker container (ORCH-2b, ADR 0012).
//
// It is the concrete runner the separate control-plane process (ORCH-2c) drives
// — the privileged component that actually starts/stops containers, kept out of
// the browser-facing server (ADR 0012 §6). The container lifecycle logic lives
// here behind a narrow ContainerClient interface so it is fully unit-testable with
// a fake (no Docker daemon needed); the single concrete implementation over the
// Docker SDK lives in client.go and is the only place that imports the heavy
// docker dependency.
//
// Instance identity is the feed id, carried as a container label
// (wayfinder.feed_id). A spec hash label (wayfinder.spec_hash) lets Start detect
// drift cheaply: a container whose hash matches the desired spec is left running
// (idempotent no-op); a changed spec triggers a replace. The reconciler (ORCH-3)
// re-applies every desired spec each cycle, so Start must stay cheap on no-ops.
package dockerbackend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/instance"
)

// Label keys stamped on every managed container.
const (
	labelManaged  = "wayfinder.managed"   // "true" on every container we own
	labelFeedID   = "wayfinder.feed_id"   // the feed id (instance identity)
	labelFeedName = "wayfinder.feed_name" // human-readable, for operators
	labelSpecHash = "wayfinder.spec_hash" // drift detection
)

// ContainerInfo is the backend's view of one managed container. The concrete
// client maps the raw Docker state into Running/Failed so the lifecycle logic
// here stays trivial and daemon-free.
type ContainerInfo struct {
	ID       string
	FeedID   int64
	Running  bool
	Failed   bool // exited non-zero / dead
	SpecHash string
}

// CreateOptions describes a container to create (not yet started).
type CreateOptions struct {
	Name        string
	Image       string
	Env         []string
	Labels      map[string]string
	NetworkMode string // e.g. "host"; multicast typically needs host networking
}

// ContainerClient is the narrow slice of container-runtime operations the backend
// needs. Implemented over the Docker SDK in client.go; faked in tests. List
// returns every wayfinder-managed container (running or stopped) so orphan
// cleanup can remove exited ones too.
type ContainerClient interface {
	List(ctx context.Context) ([]ContainerInfo, error)
	Create(ctx context.Context, opts CreateOptions) (id string, err error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
}

// Backend runs Firefly instances as Docker containers. Safe for concurrent use to
// the extent the underlying ContainerClient is (the Docker SDK client is).
type Backend struct {
	client       ContainerClient
	image        string // Firefly image to run
	networkMode  string // applied to every container (default "host")
	sceneDefault string // placeholder FIREFLY_SCENE until real source ingestion (ORCH-5)
	logger       *slog.Logger
}

// New builds a Backend. image is the Firefly container image; networkMode is the
// Docker network mode for spawned containers (multicast needs "host" or an
// equivalent); sceneDefault, when set, is passed as FIREFLY_SCENE so a spawned
// instance produces tracks before real live-source ingestion exists (ORCH-5).
func New(client ContainerClient, image, networkMode, sceneDefault string, logger *slog.Logger) *Backend {
	if logger == nil {
		logger = slog.Default()
	}
	if networkMode == "" {
		networkMode = "host"
	}
	return &Backend{
		client:       client,
		image:        image,
		networkMode:  networkMode,
		sceneDefault: sceneDefault,
		logger:       logger,
	}
}

var _ instance.Backend = (*Backend)(nil)

// Start ensures a container matching spec is running for the feed. Idempotent: an
// existing container with the same spec hash is left running (started if stopped);
// a drifted container is removed and recreated; otherwise a new one is created and
// started.
func (b *Backend) Start(ctx context.Context, spec instance.Spec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	existing, err := b.find(ctx, spec.FeedID)
	if err != nil {
		return err
	}
	env := b.fireflyEnv(spec)
	hash := specHash(b.image, b.networkMode, env)

	if existing != nil {
		if existing.SpecHash == hash {
			if existing.Running {
				return nil // already converged
			}
			return b.client.Start(ctx, existing.ID) // present but stopped → start
		}
		// Drift: tear the old container down before recreating with the new config.
		_ = b.client.Stop(ctx, existing.ID)
		if err := b.client.Remove(ctx, existing.ID); err != nil {
			return fmt.Errorf("dockerbackend: remove drifted container for feed %d: %w", spec.FeedID, err)
		}
		b.logger.Info("replacing drifted instance", slog.Int64("feed_id", spec.FeedID))
	}

	id, err := b.client.Create(ctx, CreateOptions{
		Name:        containerName(spec.FeedID),
		Image:       b.image,
		Env:         env,
		NetworkMode: b.networkMode,
		Labels: map[string]string{
			labelManaged:  "true",
			labelFeedID:   strconv.FormatInt(spec.FeedID, 10),
			labelFeedName: spec.FeedName,
			labelSpecHash: hash,
		},
	})
	if err != nil {
		return fmt.Errorf("dockerbackend: create container for feed %d: %w", spec.FeedID, err)
	}
	if err := b.client.Start(ctx, id); err != nil {
		return fmt.Errorf("dockerbackend: start container for feed %d: %w", spec.FeedID, err)
	}
	b.logger.Info("started instance",
		slog.Int64("feed_id", spec.FeedID), slog.String("name", spec.FeedName))
	return nil
}

// Stop tears down the container for feedID (stop + remove). Unknown feed is a
// no-op (idempotent).
func (b *Backend) Stop(ctx context.Context, feedID int64) error {
	existing, err := b.find(ctx, feedID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	_ = b.client.Stop(ctx, existing.ID)
	if err := b.client.Remove(ctx, existing.ID); err != nil {
		return fmt.Errorf("dockerbackend: remove container for feed %d: %w", feedID, err)
	}
	b.logger.Info("stopped instance", slog.Int64("feed_id", feedID))
	return nil
}

// Status maps the container state to an instance.Status: running → Running, an
// exited/dead container → Failed, no container → Stopped.
func (b *Backend) Status(ctx context.Context, feedID int64) (instance.Status, error) {
	existing, err := b.find(ctx, feedID)
	if err != nil {
		return "", err
	}
	switch {
	case existing == nil:
		return instance.StatusStopped, nil
	case existing.Running:
		return instance.StatusRunning, nil
	case existing.Failed:
		return instance.StatusFailed, nil
	default:
		return instance.StatusStopped, nil
	}
}

// RunningFeeds returns the ids of all managed feeds (running or stopped) so the
// reconciler can tear down orphans regardless of their container state.
func (b *Backend) RunningFeeds(ctx context.Context) ([]int64, error) {
	infos, err := b.client.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(infos))
	for _, c := range infos {
		ids = append(ids, c.FeedID)
	}
	return ids, nil
}

// find returns the managed container for feedID, or nil if none.
func (b *Backend) find(ctx context.Context, feedID int64) (*ContainerInfo, error) {
	infos, err := b.client.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range infos {
		if infos[i].FeedID == feedID {
			return &infos[i], nil
		}
	}
	return nil, nil
}

// fireflyEnv translates a Spec into the Firefly container environment: the
// ratified output config (multicast group/port), the coarse coverage bound, and —
// when the feed has live sources — Firefly's live mode driven by the FIREFLY_SOURCES
// input contract (ORCH-5; Firefly ADR 0023). Order is deterministic so the spec
// hash is stable.
//
// FIREFLY_SOURCES carries the source *structure* and the cred_env *names*; the
// resolved credential *values* (spec.ResolvedSecrets, filled by the control plane,
// ORCH-5b) are emitted as separate FIREFLY_SOURCE_<i>_SECRET envs, never inlined
// into the JSON blob. A source whose credential is unresolved is rendered without
// cred_env (Firefly then runs it anonymously). A feed without sources falls back
// to the optional placeholder scene.
func (b *Backend) fireflyEnv(spec instance.Spec) []string {
	env := []string{
		"FIREFLY_CAT062_GROUP=" + spec.Group,
		"FIREFLY_CAT062_PORT=" + strconv.Itoa(spec.Port),
		// The orchestrator exists to EMIT this feed, so the CAT062 multicast sender
		// must be on. Firefly's own default is off (a demo must not blast UDP onto
		// the network unasked) — without this the spawned tracker runs but stays
		// silent (no CAT062, no CAT065 heartbeat), and the ASD only ever sees a
		// "feed unknown" state.
		"FIREFLY_CAT062_ENABLED=true",
		// Firefly always binds an HTTP server on FIREFLY_PORT (default 8080). The
		// spawned tracker shares the HOST network namespace (multicast egress needs
		// it), where 8080 is already held by the Wayfinder probe server — so an
		// unset port makes Firefly fail to bind and crash-loop, and multiple feeds
		// would collide with each other. We give every feed a stable, distinct port
		// clear of Wayfinder (8081 UI / 8080 probe) and Postgres (5432). Firefly's
		// HTTP/WS is unused in this topology (Wayfinder consumes the multicast); the
		// port only has to bind successfully.
		"FIREFLY_PORT=" + strconv.Itoa(fireflyHTTPPort(spec.FeedID)),
	}
	if spec.Coverage != nil {
		c := spec.Coverage
		env = append(env, fmt.Sprintf("FIREFLY_COVERAGE_BBOX=%g,%g,%g,%g",
			c.MinLat, c.MinLon, c.MaxLat, c.MaxLon))
	}
	if sourcesJSON, credEnvs, ok := fireflySourcesEnv(spec.Sources, spec.ResolvedSecrets); ok {
		env = append(env, "FIREFLY_MODE=live", "FIREFLY_SOURCES="+sourcesJSON)
		env = append(env, credEnvs...)
	} else if b.sceneDefault != "" {
		env = append(env, "FIREFLY_SCENE="+b.sceneDefault)
	}
	return env
}

// containerName is the deterministic, injection-safe container name for a feed.
// It uses only the numeric feed id (never the operator-supplied feed name), so no
// sanitisation is needed and names never collide.
func containerName(feedID int64) string {
	return "wayfinder-firefly-feed-" + strconv.FormatInt(feedID, 10)
}

// fireflyHTTPPortBase is the start of the port window for the spawned Firefly
// instances' (otherwise unused) HTTP servers. It sits well clear of Wayfinder's
// own ports (8081 UI / 8080 probe) and Postgres (5432), so a host-networked
// tracker can always bind — see fireflyEnv.
const fireflyHTTPPortBase = 18080

// fireflyHTTPPort maps a feed id to a stable, collision-free HTTP port for that
// feed's spawned Firefly instance. Host networking makes the port process-global,
// so every feed needs a distinct one; the (unique) feed id provides that. The id
// is wrapped into a bounded window so a large id can never exceed the valid port
// range; the window (~40k ports) far exceeds any realistic feed count on a host.
func fireflyHTTPPort(feedID int64) int {
	const window = 40000
	off := feedID % window
	if off < 0 {
		off += window
	}
	return fireflyHTTPPortBase + int(off)
}

// specHash is a stable fingerprint of the container-defining inputs (image,
// network, env). Start compares it against a running container's stored hash to
// decide no-op vs replace. Deterministic: env order is fixed by fireflyEnv.
func specHash(image, networkMode string, env []string) string {
	sorted := append([]string(nil), env...)
	sort.Strings(sorted)
	h := sha256.New()
	h.Write([]byte(image))
	h.Write([]byte{0})
	h.Write([]byte(networkMode))
	h.Write([]byte{0})
	h.Write([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(h.Sum(nil))
}
