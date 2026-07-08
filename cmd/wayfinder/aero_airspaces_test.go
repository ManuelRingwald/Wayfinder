package main

import (
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/aeronautical"
)

// TestProjectAirspaces pins the AoR-picker projection (ASD-014): id-less features
// are dropped, numeric props are read whether int (fresh fetch) or float64
// (hydrated from the JSON cache), and the list is sorted by name.
func TestProjectAirspaces(t *testing.T) {
	fc := aeronautical.FeatureCollection{Features: []aeronautical.Feature{
		{Properties: map[string]any{"id": "62b2", "name": "HAMBURG TMA", "type": 7}},                                    // fresh: int
		{Properties: map[string]any{"id": "62a1", "name": "HAMBURG CTR", "type": float64(4), "icao_class": float64(3)}}, // hydrated: float64
		{Properties: map[string]any{"name": "NO ID"}},                                                                   // dropped (no id)
		{Properties: map[string]any{"id": ""}},                                                                          // dropped (empty id)
	}}
	got := projectAirspaces(fc)
	if len(got) != 2 {
		t.Fatalf("expected 2 options (id-less dropped), got %d: %+v", len(got), got)
	}
	// Sorted by name → "HAMBURG CTR" before "HAMBURG TMA".
	if got[0].ID != "62a1" || got[0].Name != "HAMBURG CTR" {
		t.Errorf("expected CTR first (sorted by name), got %+v", got[0])
	}
	if got[0].Type == nil || *got[0].Type != 4 || got[0].ICAOClass == nil || *got[0].ICAOClass != 3 {
		t.Errorf("hydrated float64 type/class not read: %+v", got[0])
	}
	if got[1].Type == nil || *got[1].Type != 7 {
		t.Errorf("fresh int type not read: %+v", got[1])
	}
	if got[1].ICAOClass != nil {
		t.Errorf("expected no icao_class on the TMA, got %+v", got[1])
	}
}

func TestPropInt(t *testing.T) {
	cases := []struct {
		in   any
		want int
		ok   bool
	}{
		{5, 5, true},
		{float64(7), 7, true},
		{"nope", 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got, ok := propInt(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("propInt(%v) = %d,%v want %d,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}
