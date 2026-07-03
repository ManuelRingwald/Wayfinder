package feature

import (
	"sort"
	"testing"
)

func TestIsKnown(t *testing.T) {
	for _, k := range []Key{STCA, MultiFeed, PremiumLayers, Airspaces, RangeRings, HistoryDots, VorNdb, Waypoints, WeatherRadar, QNH} {
		if !IsKnown(k) {
			t.Errorf("IsKnown(%q) = false, want true", k)
		}
	}
	for _, k := range []Key{"", "STCA", "bogus", "stca "} {
		if IsKnown(k) {
			t.Errorf("IsKnown(%q) = true, want false", k)
		}
	}
}

func TestAllSortedAndComplete(t *testing.T) {
	all := All()
	if len(all) != 10 {
		t.Fatalf("All() len = %d, want 10", len(all))
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i] < all[j] }) {
		t.Errorf("All() not sorted: %v", all)
	}
	seen := map[Key]bool{}
	for _, k := range all {
		seen[k] = true
	}
	for _, want := range []Key{STCA, MultiFeed, PremiumLayers, Airspaces, RangeRings, HistoryDots, VorNdb, Waypoints, WeatherRadar, QNH} {
		if !seen[want] {
			t.Errorf("All() missing %q", want)
		}
	}
}

func TestDescribe(t *testing.T) {
	if Describe(STCA) == "" {
		t.Error("Describe(STCA) = empty, want non-empty")
	}
	if got := Describe("bogus"); got != "" {
		t.Errorf("Describe(bogus) = %q, want empty", got)
	}
}
