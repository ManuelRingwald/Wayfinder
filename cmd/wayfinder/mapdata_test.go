package main

import (
	"context"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/coverage"
)

// K2 (#310): the map-data config plane resolves theme DB-override ?? env-default
// and validates admin input. (Reuses memSettings from aero_test.go.)
func TestMapDataEffectiveTheme(t *testing.T) {
	ctx := context.Background()
	st := newMemSettings()
	cfg := Config{MapTheme: mapThemeBKGDark, BKGStyleURL: "https://default.example/style.json"}
	md := newMapDataConfig(st, cfg, nil, nil)

	if got := md.effectiveTheme(ctx); got != mapThemeBKGDark {
		t.Fatalf("default theme = %q, want %q", got, mapThemeBKGDark)
	}
	if err := md.theme.Set(ctx, mapThemeBKG); err != nil {
		t.Fatal(err)
	}
	if got := md.effectiveTheme(ctx); got != mapThemeBKG {
		t.Fatalf("override theme = %q, want %q", got, mapThemeBKG)
	}
	// Reset (empty) falls back to the env default.
	if err := md.theme.Set(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if got := md.effectiveTheme(ctx); got != mapThemeBKGDark {
		t.Fatalf("after reset theme = %q, want env default %q", got, mapThemeBKGDark)
	}
}

func TestValidTheme(t *testing.T) {
	for _, ok := range []string{mapThemeBKG, mapThemeBKGDark} {
		if err := validTheme(ok); err != nil {
			t.Errorf("theme %q should be valid: %v", ok, err)
		}
	}
	if validTheme("neon") == nil {
		t.Error("unknown theme must be rejected")
	}
	if validTheme("") == nil {
		t.Error("empty theme must be rejected by the validator")
	}
}

// reloadBasemap with a nil service (custom style bypasses the base-map service)
// is a safe no-op, not a crash.
func TestReloadBasemapNilServiceNoop(t *testing.T) {
	md := newMapDataConfig(newMemSettings(), Config{MapTheme: mapThemeBKGDark}, nil, nil)
	if err := md.reloadBasemap(context.Background()); err != nil {
		t.Fatalf("nil-service reload should be a no-op, got %v", err)
	}
	// applyAtBoot must not panic either.
	md.applyAtBoot(context.Background())
}

// K4 (#312): coverage sensor list + ring colour override, with validation.
func TestValidateSensors(t *testing.T) {
	ok := []coverage.SensorConfig{{Lat: 50, Lon: 8, MinRangeM: 0, MaxRangeM: 120000, Label: "FRA"}}
	if err := validateSensors(ok); err != nil {
		t.Fatalf("valid sensor rejected: %v", err)
	}
	bad := [][]coverage.SensorConfig{
		{{Lat: 91, Lon: 8, MaxRangeM: 1000}},                  // lat out of range
		{{Lat: 50, Lon: 181, MaxRangeM: 1000}},                // lon out of range
		{{Lat: 50, Lon: 8, MaxRangeM: 0}},                     // max range 0
		{{Lat: 50, Lon: 8, MinRangeM: 2000, MaxRangeM: 1000}}, // min ≥ max
	}
	for i, s := range bad {
		if err := validateSensors(s); err == nil {
			t.Errorf("bad sensor set %d should be rejected", i)
		}
	}
}

func TestEffectiveSensors(t *testing.T) {
	ctx := context.Background()
	st := newMemSettings()
	env := []coverage.SensorConfig{{Lat: 50, Lon: 8, MaxRangeM: 120000, Label: "env"}}
	md := newMapDataConfig(st, Config{CoverageSensors: env, CoverageRingColor: "#abcdef"}, nil, nil)

	// No override → env sensors + env colour.
	if got := md.effectiveSensors(ctx); len(got) != 1 || got[0].Label != "env" {
		t.Fatalf("default sensors = %+v", got)
	}
	if c := md.effectiveRingColor(ctx); c != "#abcdef" {
		t.Fatalf("default colour = %q", c)
	}

	// Override with a stored JSON list.
	_ = md.coverageSensors.Set(ctx, `[{"Lat":52,"Lon":9,"MaxRangeM":90000,"Label":"db"}]`)
	if got := md.effectiveSensors(ctx); len(got) != 1 || got[0].Label != "db" {
		t.Fatalf("override sensors = %+v", got)
	}

	// Malformed override degrades to the env sensors (never a broken overlay).
	_ = md.coverageSensors.Set(ctx, `not json`)
	if got := md.effectiveSensors(ctx); len(got) != 1 || got[0].Label != "env" {
		t.Fatalf("malformed override should fall back to env, got %+v", got)
	}
}
