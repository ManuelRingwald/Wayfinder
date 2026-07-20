package main

import (
	"context"
	"testing"
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
