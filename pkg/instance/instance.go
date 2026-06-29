// Package instance is the orchestrator's abstraction over running one Firefly
// tracker instance per feed (ORCH-2, ADR 0012).
//
// ADR 0012 turns "assign a feed to a tenant" into "the matching Firefly instance
// starts automatically". The lifecycle unit is the feed: one feed = one
// multicast group = one dedicated Firefly instance, configured from the feed's
// generic source list (ORCH-1). This package holds the backend-agnostic core:
//
//   - Spec — the generic, Firefly-agnostic launch specification derived from a
//     feed (its multicast endpoint, coarse coverage bound, source list and the
//     *references* to its per-feed secrets — never the secret values).
//   - Backend — the Start/Stop/Status interface a concrete runner implements
//     (a Docker adapter first, ORCH-2b; Kubernetes later, ORCH-6). Mirrors the
//     feedmanager.Manager shape: injected and idempotent, so the control plane
//     and reconciler (ORCH-3) are unit-testable without spawning anything.
//   - MemoryBackend — an in-memory backend used as the test double and the
//     single-host dev placeholder until the Docker adapter exists.
//
// Security (ADR 0012 §6): spawning processes/containers is a privilege jump, so
// the Backend is meant to run in a separate, least-privilege control-plane
// component (ORCH-2c), never in the browser-facing server. The Spec carries only
// secret *references* (cred_ref handles); the actual secret values are resolved
// and handed to the Backend at launch, never stored in the Spec or logged.
package instance

import (
	"context"
	"fmt"
	"sort"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// Status is the lifecycle state of a tracker instance, as reported by a Backend.
type Status string

const (
	// StatusProvisioning: the backend is creating/starting the instance.
	StatusProvisioning Status = "provisioning"
	// StatusRunning: the instance is up.
	StatusRunning Status = "running"
	// StatusFailed: the instance could not be started or crashed.
	StatusFailed Status = "failed"
	// StatusStopped: no instance is running for the feed (never started or torn down).
	StatusStopped Status = "stopped"
)

// Feed is the minimal feed descriptor the orchestrator needs to derive a Spec —
// its catalogue id, name (for logging) and the multicast endpoint the resulting
// Firefly instance must emit on. Mirrors feedmanager.Feed (deliberately a small,
// local type so this package does not couple to the receiver/transport layer);
// the source list and coverage are passed alongside (they live behind dedicated
// store accessors, ORCH-1).
type Feed struct {
	ID    int64
	Name  string
	Group string
	Port  int
}

// Spec is the generic, Firefly-agnostic launch specification for one feed's
// tracker instance. It is the structured hand-off between Wayfinder and a
// Backend: the Backend (ORCH-2b) translates it into a concrete launch (container
// env, Firefly-specific variable names). Keeping the names out of the Spec is
// deliberate — the exact Firefly input-config contract (FIREFLY_SOURCES etc.) is
// cross-project work (ORCH-5) and not yet ratified, so the orchestrator core
// stays in structured terms.
type Spec struct {
	FeedID   int64
	FeedName string
	// Multicast endpoint the Firefly instance must emit CAT062/063/065 on. Owned
	// by Wayfinder (the feed already holds it); the reconciler keeps it
	// collision-free across feeds (ORCH-3).
	Group string
	Port  int
	// Coverage is the coarse outer geographic bound handed to Firefly
	// (FIREFLY_COVERAGE_BBOX); nil when the feed has no bbox-bounded source.
	Coverage *store.BBox
	// Sources is the generic source list the instance should ingest from.
	Sources store.SourceConfig
	// SecretRefs are the distinct cred_ref handles the sources reference, sorted
	// and de-duplicated. They are *references*, never secret values — the
	// control plane resolves them to values at launch and hands them to the
	// Backend out of band (ADR 0012 §6).
	SecretRefs []string
	// ResolvedSecrets maps a cred_ref to its resolved plaintext credential value
	// (ORCH-5b, ADR 0012 §6). It is filled by the control plane (StoreDesiredState
	// via the SecretResolver) — never by SpecFromFeed, which stays pure — and is
	// nil when no key is configured or no source is credentialled. The value lives
	// only in this least-privilege orchestrator process (never browser-facing) and
	// flows into the spawned container's env, so a secret rotation changes the spec
	// hash and the reconciler restarts the instance with the new value.
	ResolvedSecrets map[string]string
}

// SpecFromFeed derives the launch Spec for a feed from its descriptor, source
// configuration and derived coverage (all produced by ORCH-1). It is pure: it
// collects the distinct, sorted secret references from the sources and copies
// the structured config; it performs no I/O and resolves no secrets.
func SpecFromFeed(f Feed, sources store.SourceConfig, coverage *store.BBox) Spec {
	return Spec{
		FeedID:     f.ID,
		FeedName:   f.Name,
		Group:      f.Group,
		Port:       f.Port,
		Coverage:   coverage,
		Sources:    sources,
		SecretRefs: collectSecretRefs(sources),
	}
}

// collectSecretRefs returns the distinct, sorted cred_ref handles referenced by
// the sources. Order is deterministic so two equal source lists yield an equal
// Spec (the reconciler compares specs to detect drift, ORCH-3).
func collectSecretRefs(sources store.SourceConfig) []string {
	seen := make(map[string]struct{})
	for _, s := range sources {
		if s.CredRef != nil && *s.CredRef != "" {
			seen[*s.CredRef] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	refs := make([]string, 0, len(seen))
	for ref := range seen {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}

// Endpoint returns the "group:port" key for the instance's multicast output. The
// reconciler uses it to detect two feeds configured onto the same endpoint
// (which would collide on the wire).
func (s Spec) Endpoint() string {
	return fmt.Sprintf("%s:%d", s.Group, s.Port)
}

// Validate checks the Spec is launchable: a feed id, a name, and a usable
// multicast endpoint. The source list itself is validated upstream at the write
// boundary (store.SourceConfig.Validate, ORCH-1); this guards the orchestration
// fields a Backend relies on.
func (s Spec) Validate() error {
	if s.FeedID == 0 {
		return fmt.Errorf("instance: spec has no feed id")
	}
	if s.Group == "" {
		return fmt.Errorf("instance: spec for feed %d has no multicast group", s.FeedID)
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("instance: spec for feed %d has invalid port %d", s.FeedID, s.Port)
	}
	return nil
}

// Backend runs and tears down tracker instances. Implementations are injected so
// the control plane and reconciler are unit-testable without spawning anything
// (a Docker adapter follows in ORCH-2b, Kubernetes in ORCH-6). Contract:
//
//   - Start is idempotent: starting a feed whose instance is already running with
//     an equal Spec is a no-op; a changed Spec re-applies (the running instance is
//     reconfigured/replaced). Instance identity is the feed id.
//   - Stop tears down the instance for a feed id; stopping an unknown feed is not
//     an error (idempotent, so the reconciler can call it freely).
//   - Status reports the current lifecycle state; an unknown feed is StatusStopped.
//
// Implementations must be safe for concurrent use (the reconciler may act on
// several feeds at once).
type Backend interface {
	Start(ctx context.Context, spec Spec) error
	Stop(ctx context.Context, feedID int64) error
	Status(ctx context.Context, feedID int64) (Status, error)
	// RunningFeeds returns the ids of feeds the backend currently has a live
	// instance for. The reconciler (ORCH-3) compares this actual set against the
	// desired set to tear down orphans — observing real state rather than trusting
	// remembered state is what makes the operator loop crash-safe.
	RunningFeeds(ctx context.Context) ([]int64, error)
}
