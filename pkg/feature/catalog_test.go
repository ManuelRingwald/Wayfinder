package feature

import (
	"sort"
	"strings"
	"testing"
)

func TestIsKnown(t *testing.T) {
	for _, k := range []Key{STCA, MultiFeed, PremiumLayers, Airspaces, RangeRings, HistoryDots, VorNdb, Waypoints, WeatherRadar, QNH, WeatherWarnings} {
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
	if len(all) != 11 {
		t.Fatalf("All() len = %d, want 11", len(all))
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i] < all[j] }) {
		t.Errorf("All() not sorted: %v", all)
	}
	seen := map[Key]bool{}
	for _, k := range all {
		seen[k] = true
	}
	for _, want := range []Key{STCA, MultiFeed, PremiumLayers, Airspaces, RangeRings, HistoryDots, VorNdb, Waypoints, WeatherRadar, QNH, WeatherWarnings} {
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

func TestLabel(t *testing.T) {
	// Every known key carries a non-empty label + description — the admin panel
	// shows the label as the heading, so a blank one would render a bare key.
	for _, k := range All() {
		if Label(k) == "" {
			t.Errorf("Label(%q) = empty, want a Fachbegriff", k)
		}
		if Describe(k) == "" {
			t.Errorf("Describe(%q) = empty, want a description", k)
		}
	}
	if got := Label("bogus"); got != "" {
		t.Errorf("Label(bogus) = %q, want empty", got)
	}
}

// TestNoInternalDocRefs pins the operator-facing copy free of internal document
// references (requirement IDs, ADR numbers, ASTERIX item codes). These mean
// nothing to an administrator granting entitlements and were removed on request;
// this guard stops them from creeping back into a label or description.
func TestNoInternalDocRefs(t *testing.T) {
	forbidden := []string{"ASD-", "WF2", "WX-", "ADR", "I062", "I065"}
	for _, k := range All() {
		for _, field := range []struct{ name, val string }{
			{"label", Label(k)},
			{"description", Describe(k)},
		} {
			for _, bad := range forbidden {
				if strings.Contains(field.val, bad) {
					t.Errorf("%s of %q contains internal ref %q: %q", field.name, k, bad, field.val)
				}
			}
		}
	}
}
