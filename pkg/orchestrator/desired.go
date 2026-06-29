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

// StoreDesiredState implements reconciler.DesiredState from the catalogue: the
// desired set is every subscribed feed, each turned into an instance.Spec from
// its source configuration. It performs no spawning and resolves no secrets — it
// only translates catalogue rows into launch specs (the Spec carries secret
// references, never values, ADR 0012 §6).
type StoreDesiredState struct {
	feeds   SubscribedFeedLister
	sources SourceConfigReader
}

// NewStoreDesiredState wires the adapter over the subscription and feed repos.
func NewStoreDesiredState(feeds SubscribedFeedLister, sources SourceConfigReader) *StoreDesiredState {
	return &StoreDesiredState{feeds: feeds, sources: sources}
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
		specs = append(specs, instance.SpecFromFeed(instance.Feed{
			ID:    f.ID,
			Name:  f.Name,
			Group: f.MulticastGroup,
			Port:  f.Port,
		}, sources, coverage))
	}
	return specs, nil
}
