// Package orchestrator is the control-plane glue that drives the auto-spawning of
// per-feed Firefly instances (ORCH-2c, ADR 0012). It connects the catalogue (the
// desired state — which feeds should run, with what config) to the reconciler and
// instance.Backend that converge the actual running set toward it.
//
// Security (ADR 0012 §6): this control-plane code is meant to run as a separate,
// least-privilege component — never in the browser-facing server, which only
// writes the desired state (feeds + source config) to the database. This first
// slice provides the store-backed DesiredState adapter; the separate process,
// secret resolution and change trigger follow in subsequent ORCH-2c steps.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/manuelringwald/wayfinder/pkg/instance"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// SubscribedFeedLister yields the feeds that should currently run an instance —
// those with at least one active subscription (satisfied by *store.SubscriptionRepo).
type SubscribedFeedLister interface {
	ListSubscribedFeeds(ctx context.Context) ([]store.Feed, error)
}

// SourceConfigReader reads a feed's generic source configuration and derived
// coverage (satisfied by *store.FeedRepo).
type SourceConfigReader interface {
	GetSourceConfig(ctx context.Context, feedID int64) (store.SourceConfig, *store.BBox, error)
}

// FeedSecretResolver turns a feed's cred_ref handle into its plaintext credential
// (satisfied by *SecretResolver). It is optional: when configured (a deployment
// secret key is present, ORCH-5b) StoreDesiredState resolves each spec's secret
// references into Spec.ResolvedSecrets so the values reach the spawned container's
// env. The plaintext lives only in this least-privilege control-plane process and
// is never logged.
type FeedSecretResolver interface {
	Resolve(ctx context.Context, feedID int64, credRef string) (string, error)
}

// StoreDesiredState implements reconciler.DesiredState from the catalogue: the
// desired set is every subscribed feed, each turned into an instance.Spec from
// its source configuration. It translates catalogue rows into launch specs; when a
// secret resolver is wired (WithSecretResolver, ORCH-5b) it additionally resolves
// each feed's secret references into plaintext values for the launch (ADR 0012 §6)
// — best-effort, so a missing key or secret degrades a source to anonymous rather
// than failing the whole reconcile pass.
type StoreDesiredState struct {
	feeds   SubscribedFeedLister
	sources SourceConfigReader
	secrets FeedSecretResolver // nil when no deployment key is configured
	logger  *slog.Logger
}

// NewStoreDesiredState wires the adapter over the subscription and feed repos.
// Secret resolution is off by default (the Spec then carries only references); use
// WithSecretResolver to enable it.
func NewStoreDesiredState(feeds SubscribedFeedLister, sources SourceConfigReader) *StoreDesiredState {
	return &StoreDesiredState{feeds: feeds, sources: sources, logger: slog.Default()}
}

// WithSecretResolver enables credential resolution at desired-state computation
// (ORCH-5b): each spec's secret references are decrypted into Spec.ResolvedSecrets
// so the launch backend can inject the values into the tracker container. The
// resolver lives only in this control-plane process (it holds the deployment key).
// A nil logger keeps the default. Returns the same value for fluent wiring.
func (d *StoreDesiredState) WithSecretResolver(r FeedSecretResolver, logger *slog.Logger) *StoreDesiredState {
	d.secrets = r
	if logger != nil {
		d.logger = logger
	}
	return d
}

// DesiredSpecs returns one Spec per subscribed feed. A failure to read any feed's
// source config aborts the whole computation: a partial desired set would make the
// reconciler tear down instances for feeds it merely failed to read, so it is
// safer to fail the pass and retry on the next tick (the reconciler treats a
// DesiredState error as "do nothing this cycle").
func (d *StoreDesiredState) DesiredSpecs(ctx context.Context) ([]instance.Spec, error) {
	feeds, err := d.feeds.ListSubscribedFeeds(ctx)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: list subscribed feeds: %w", err)
	}
	specs := make([]instance.Spec, 0, len(feeds))
	for _, f := range feeds {
		sources, coverage, err := d.sources.GetSourceConfig(ctx, f.ID)
		if err != nil {
			return nil, fmt.Errorf("orchestrator: source config for feed %d: %w", f.ID, err)
		}
		spec := instance.SpecFromFeed(instance.Feed{
			ID:    f.ID,
			Name:  f.Name,
			Group: f.MulticastGroup,
			Port:  f.Port,
		}, sources, coverage)
		d.resolveSecrets(ctx, &spec)
		specs = append(specs, spec)
	}
	return specs, nil
}

// resolveSecrets fills spec.ResolvedSecrets from the spec's secret references when a
// resolver is configured. It is deliberately best-effort: a reference that cannot be
// resolved (no secret stored, wrong key, tampered blob) is logged at WARN and left
// out, so the dependent source is rendered anonymously (fireflySourcesEnv omits its
// cred_env) rather than failing the launch or aborting the reconcile pass. The
// resolved plaintext is never logged — only the reference and the error reason.
func (d *StoreDesiredState) resolveSecrets(ctx context.Context, spec *instance.Spec) {
	if d.secrets == nil || len(spec.SecretRefs) == 0 {
		return
	}
	resolved := make(map[string]string, len(spec.SecretRefs))
	for _, ref := range spec.SecretRefs {
		v, err := d.secrets.Resolve(ctx, spec.FeedID, ref)
		if err != nil {
			d.logger.Warn("secret unresolved — source runs anonymously",
				slog.Int64("feed_id", spec.FeedID),
				slog.String("cred_ref", ref),
				slog.String("error", err.Error()))
			continue
		}
		if v != "" {
			resolved[ref] = v
		}
	}
	if len(resolved) > 0 {
		spec.ResolvedSecrets = resolved
	}
}
