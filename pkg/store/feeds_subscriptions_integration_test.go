package store

import (
	"context"
	"errors"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/sensorclass"
)

func TestIntegrationFeedRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewFeedRepo(pool)

	region := "EDGG"
	f, err := repo.Create(ctx, "FFM-Approach", "239.255.0.62", 8600, &region, []string{"ADS-B", "PSR"})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if f.ID == 0 || f.Port != 8600 || f.Region == nil || *f.Region != "EDGG" {
		t.Fatalf("unexpected feed: %+v", f)
	}
	if len(f.SensorMix) != 2 || f.SensorMix[0] != "ADS-B" || f.SensorMix[1] != "PSR" {
		t.Fatalf("sensor_mix did not round-trip: %+v", f.SensorMix)
	}

	got, err := repo.GetByID(ctx, f.ID)
	if err != nil || got.Name != "FFM-Approach" || len(got.SensorMix) != 2 {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}

	// Nil sensorMix and nil region are stored cleanly (empty array / NULL).
	f2, err := repo.Create(ctx, "Bare", "239.255.0.63", 8601, nil, nil)
	if err != nil || f2.Region != nil || len(f2.SensorMix) != 0 {
		t.Fatalf("bare feed = %+v, %v", f2, err)
	}

	if _, err := repo.GetByID(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID(missing) = %v, want ErrNotFound", err)
	}

	list, err := repo.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("List len = %d, %v", len(list), err)
	}

	// WF2-41: an unknown sensor class is rejected before any row is written.
	_, err = repo.Create(ctx, "Bad", "239.255.0.64", 8602, nil, []string{"PSR", "bogus"})
	var uce *sensorclass.UnknownClassError
	if !errors.As(err, &uce) || uce.Token != "bogus" {
		t.Fatalf("create with unknown class err = %v, want *UnknownClassError{bogus}", err)
	}

	// Legacy spellings are normalised + deduped to canonical classes on write.
	f3, err := repo.Create(ctx, "Legacy", "239.255.0.65", 8603, nil, []string{"ads-b", "Mode S", "ADSB"})
	if err != nil {
		t.Fatalf("create legacy feed: %v", err)
	}
	if len(f3.SensorMix) != 2 || f3.SensorMix[0] != "ADS-B" || f3.SensorMix[1] != "MODE_S" {
		t.Fatalf("legacy mix not canonicalised/deduped: %+v", f3.SensorMix)
	}
}

func TestIntegrationFeedSourceConfig(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewFeedRepo(pool)

	f, err := repo.Create(ctx, "Speyer", "239.255.0.70", 8700, nil, []string{"ADS-B"})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	// A fresh feed defaults to an empty source config and no coverage (migration
	// 00010 default '[]' / NULL).
	sources, coverage, err := repo.GetSourceConfig(ctx, f.ID)
	if err != nil {
		t.Fatalf("get default source config: %v", err)
	}
	if len(sources) != 0 || coverage != nil {
		t.Fatalf("default config = %+v / %+v, want empty / nil", sources, coverage)
	}

	// Round-trip a mixed config with a derived coverage bbox.
	cfg := SourceConfig{
		{Type: SourceADSBOpenSky, BBox: bbox(48, 7, 50, 9), CredRef: ptrStr("secret/speyer-opensky")},
		{Type: SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)},
	}
	cov := cfg.CoverageBBox(50)
	if err := repo.SetSourceConfig(ctx, f.ID, cfg, cov); err != nil {
		t.Fatalf("set source config: %v", err)
	}
	gotSources, gotCov, err := repo.GetSourceConfig(ctx, f.ID)
	if err != nil {
		t.Fatalf("get source config: %v", err)
	}
	if len(gotSources) != 2 || gotSources[0].Type != SourceADSBOpenSky || gotSources[1].Type != SourceRadarASTERIX {
		t.Fatalf("sources did not round-trip: %+v", gotSources)
	}
	if gotSources[0].CredRef == nil || *gotSources[0].CredRef != "secret/speyer-opensky" {
		t.Fatalf("cred_ref did not round-trip: %+v", gotSources[0])
	}
	if gotSources[1].SAC == nil || *gotSources[1].SAC != 1 || gotSources[1].SIC == nil || *gotSources[1].SIC != 4 {
		t.Fatalf("sac/sic did not round-trip: %+v", gotSources[1])
	}
	if gotCov == nil || *gotCov != *cov {
		t.Fatalf("coverage = %+v, want %+v", gotCov, cov)
	}

	// An invalid config is rejected before any write (no partial update).
	bad := SourceConfig{{Type: SourceADSBOpenSky}} // missing bbox
	var ise *InvalidSourceError
	if err := repo.SetSourceConfig(ctx, f.ID, bad, nil); !errors.As(err, &ise) {
		t.Fatalf("set invalid config = %v, want *InvalidSourceError", err)
	}
	// The previous good config is untouched.
	stillSources, _, err := repo.GetSourceConfig(ctx, f.ID)
	if err != nil || len(stillSources) != 2 {
		t.Fatalf("config after rejected write = %+v, %v, want 2 sources preserved", stillSources, err)
	}

	// Setting an empty config clears sources and coverage.
	if err := repo.SetSourceConfig(ctx, f.ID, nil, nil); err != nil {
		t.Fatalf("clear source config: %v", err)
	}
	clearedSources, clearedCov, err := repo.GetSourceConfig(ctx, f.ID)
	if err != nil || len(clearedSources) != 0 || clearedCov != nil {
		t.Fatalf("cleared config = %+v / %+v, %v", clearedSources, clearedCov, err)
	}

	// A missing feed yields ErrNotFound for both accessors.
	if _, _, err := repo.GetSourceConfig(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSourceConfig(missing) = %v, want ErrNotFound", err)
	}
	if err := repo.SetSourceConfig(ctx, 999999, nil, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetSourceConfig(missing) = %v, want ErrNotFound", err)
	}
}

func TestIntegrationSubscriptionRepoIsolation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	feeds := NewFeedRepo(pool)
	subs := NewSubscriptionRepo(pool)

	ffm, _ := tenants.Create(ctx, "frankfurt", "Frankfurt")
	stg, _ := tenants.Create(ctx, "stuttgart", "Stuttgart")
	feed1, _ := feeds.Create(ctx, "feed-ffm", "239.255.0.62", 8600, nil, nil)
	feed2, _ := feeds.Create(ctx, "feed-stg", "239.255.0.64", 8600, nil, nil)

	if err := subs.Subscribe(ctx, ffm.ID, feed1.ID); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	// Idempotent: subscribing again is a no-op.
	if err := subs.Subscribe(ctx, ffm.ID, feed1.ID); err != nil {
		t.Fatalf("re-subscribe should be a no-op: %v", err)
	}
	if err := subs.Subscribe(ctx, stg.ID, feed2.ID); err != nil {
		t.Fatalf("subscribe stg: %v", err)
	}

	// Isolation: Frankfurt sees only its own feed, never Stuttgart's.
	ffmFeeds, err := subs.ListFeedsByTenant(ctx, ffm.ID)
	if err != nil {
		t.Fatalf("list feeds: %v", err)
	}
	if len(ffmFeeds) != 1 || ffmFeeds[0].ID != feed1.ID {
		t.Fatalf("Frankfurt feeds = %+v, want only feed1", ffmFeeds)
	}

	if ok, _ := subs.IsSubscribed(ctx, ffm.ID, feed2.ID); ok {
		t.Fatal("Frankfurt must not be subscribed to Stuttgart's feed")
	}
	if ok, _ := subs.IsSubscribed(ctx, ffm.ID, feed1.ID); !ok {
		t.Fatal("Frankfurt should be subscribed to its own feed")
	}

	ids, err := subs.ListFeedIDsByTenant(ctx, ffm.ID)
	if err != nil || len(ids) != 1 || ids[0] != feed1.ID {
		t.Fatalf("feed ids = %v, %v", ids, err)
	}

	// ListSubscribedFeeds returns the distinct feeds with ≥1 subscription
	// (ORCH-3 desired-instance input). Both feeds have a subscriber here.
	subscribed, err := subs.ListSubscribedFeeds(ctx)
	if err != nil {
		t.Fatalf("list subscribed feeds: %v", err)
	}
	if len(subscribed) != 2 {
		t.Fatalf("subscribed feeds = %d, want 2", len(subscribed))
	}
	// A second subscriber on feed1 must not duplicate it (DISTINCT).
	if err := subs.Subscribe(ctx, stg.ID, feed1.ID); err != nil {
		t.Fatalf("second subscribe: %v", err)
	}
	if again, _ := subs.ListSubscribedFeeds(ctx); len(again) != 2 {
		t.Fatalf("subscribed feeds after 2nd subscriber = %d, want 2 (distinct)", len(again))
	}

	// Unsubscribe removes access.
	if err := subs.Unsubscribe(ctx, ffm.ID, feed1.ID); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	if ok, _ := subs.IsSubscribed(ctx, ffm.ID, feed1.ID); ok {
		t.Fatal("Frankfurt should no longer be subscribed after unsubscribe")
	}

	// feed2 lost its only subscriber? No — stg still has feed1; remove all of
	// feed2's subscribers and confirm it drops out of the subscribed set.
	if err := subs.Unsubscribe(ctx, stg.ID, feed2.ID); err != nil {
		t.Fatalf("unsubscribe feed2: %v", err)
	}
	remaining, err := subs.ListSubscribedFeeds(ctx)
	if err != nil {
		t.Fatalf("list subscribed feeds (after): %v", err)
	}
	// feed1 still has stg; feed2 has none → only feed1 remains.
	if len(remaining) != 1 || remaining[0].ID != feed1.ID {
		t.Fatalf("subscribed feeds = %+v, want only feed1", remaining)
	}
}

func TestIntegrationEntitlementRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	ent := NewEntitlementRepo(pool)

	ten, _ := tenants.Create(ctx, "frankfurt", "Frankfurt")

	// Default-deny: an unset feature is not enabled, and that is not an error.
	if on, err := ent.IsEnabled(ctx, ten.ID, "psr_layer"); err != nil || on {
		t.Fatalf("unset feature = %v, %v, want false/nil", on, err)
	}

	if err := ent.Set(ctx, ten.ID, "psr_layer", true); err != nil {
		t.Fatalf("set: %v", err)
	}
	if on, err := ent.IsEnabled(ctx, ten.ID, "psr_layer"); err != nil || !on {
		t.Fatalf("enabled feature = %v, %v, want true", on, err)
	}

	// Upsert flips the value.
	if err := ent.Set(ctx, ten.ID, "psr_layer", false); err != nil {
		t.Fatalf("set false: %v", err)
	}
	if on, _ := ent.IsEnabled(ctx, ten.ID, "psr_layer"); on {
		t.Fatal("feature should be disabled after upsert to false")
	}

	if err := ent.Set(ctx, ten.ID, "history", true); err != nil {
		t.Fatalf("set history: %v", err)
	}
	all, err := ent.ListByTenant(ctx, ten.ID)
	if err != nil || len(all) != 2 || all["psr_layer"] != false || all["history"] != true {
		t.Fatalf("ListByTenant = %v, %v", all, err)
	}
}
