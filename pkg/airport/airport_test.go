package airport

import (
	"math"
	"strings"
	"testing"
)

// fixture builds a small, deterministic index so ranking tests don't depend on
// the (large, evolving) embedded dataset.
func fixture() *Index {
	return parse(strings.Join([]string{
		"EDDH\tHamburg Helmut Schmidt Airport\t53.63040\t9.98823",
		"EDDF\tFrankfurt Main Airport\t50.02671\t8.55835",
		"EDDL\tDusseldorf Airport\t51.28950\t6.76678",
		"EGLL\tLondon Heathrow Airport\t51.47075\t-0.45991",
		"KJFK\tJohn F. Kennedy International Airport\t40.63945\t-73.77932",
	}, "\n"))
}

func icaos(as []Airport) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.ICAO
	}
	return out
}

// #192: InBBox returns only aerodromes inside the WGS84 box, ICAO-sorted.
func TestInBBoxReturnsOnlyInSectorAirportsSorted(t *testing.T) {
	ix := fixture()
	// A box around northern Germany: EDDH (Hamburg) and EDDL (Düsseldorf) in,
	// EDDF (Frankfurt), EGLL (London) and KJFK (New York) out.
	got := icaos(ix.InBBox(51.0, 6.0, 54.0, 11.0, 0))
	want := []string{"EDDH", "EDDL"}
	if len(got) != len(want) {
		t.Fatalf("InBBox: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("InBBox order: got %v, want %v", got, want)
		}
	}
}

func TestInBBoxRespectsLimit(t *testing.T) {
	ix := fixture()
	if n := len(ix.InBBox(-90, -180, 90, 180, 1)); n != 1 {
		t.Fatalf("InBBox limit: got %d, want 1", n)
	}
	// Non-positive limit → unbounded (all five fixture airports).
	if n := len(ix.InBBox(-90, -180, 90, 180, 0)); n != 5 {
		t.Fatalf("InBBox unbounded: got %d, want 5", n)
	}
}

func TestSearchExactICAOWinsAndFillsCoords(t *testing.T) {
	got := fixture().Search("EDDH", 10)
	if len(got) == 0 || got[0].ICAO != "EDDH" {
		t.Fatalf("exact ICAO EDDH not first: %v", icaos(got))
	}
	if got[0].Name != "Hamburg Helmut Schmidt Airport" {
		t.Errorf("name = %q", got[0].Name)
	}
	if math.Abs(got[0].Lat-53.63040) > 1e-5 || math.Abs(got[0].Lon-9.98823) > 1e-5 {
		t.Errorf("coords = %v/%v, want 53.63040/9.98823", got[0].Lat, got[0].Lon)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	if got := fixture().Search("eddh", 10); len(got) == 0 || got[0].ICAO != "EDDH" {
		t.Fatalf("lower-case query failed: %v", icaos(got))
	}
}

func TestSearchPrefixRankedAfterExactAndSortedByICAO(t *testing.T) {
	// "EDD" is a prefix of EDDF, EDDH, EDDL — none is an exact match, so all
	// come back at rank 1, ordered by ICAO.
	got := icaos(fixture().Search("EDD", 10))
	want := []string{"EDDF", "EDDH", "EDDL"}
	if len(got) != len(want) {
		t.Fatalf("EDD prefix: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("EDD prefix order: got %v, want %v", got, want)
		}
	}
}

func TestSearchNameSubstringRankedLast(t *testing.T) {
	// "Airport" appears in several names (rank 2). A pure ICAO prefix match must
	// still outrank a name hit: query "EGLL" is exact, but "London" is name-only.
	got := icaos(fixture().Search("london", 10))
	if len(got) != 1 || got[0] != "EGLL" {
		t.Fatalf("name search 'london': got %v, want [EGLL]", got)
	}
}

func TestSearchExactBeatsNameSubstring(t *testing.T) {
	// A query that is both an exact ICAO (rank 0) and a substring of other names
	// must return the exact match first.
	got := fixture().Search("EDDF", 10)
	if len(got) == 0 || got[0].ICAO != "EDDF" {
		t.Fatalf("EDDF should rank first: %v", icaos(got))
	}
}

func TestSearchMinQueryLen(t *testing.T) {
	for _, q := range []string{"", " ", "E", "  x "} {
		if got := fixture().Search(q, 10); got != nil {
			t.Errorf("Search(%q) = %v, want nil (below min length)", q, icaos(got))
		}
	}
}

func TestSearchLimit(t *testing.T) {
	got := fixture().Search("ED", 2) // EDDF, EDDH, EDDL match; cap at 2
	if len(got) != 2 {
		t.Fatalf("limit 2: got %d results (%v)", len(got), icaos(got))
	}
}

func TestLookup(t *testing.T) {
	ix := fixture()
	if a, ok := ix.Lookup("eddh"); !ok || a.Name != "Hamburg Helmut Schmidt Airport" {
		t.Errorf("Lookup(eddh) = %v, %v", a, ok)
	}
	if _, ok := ix.Lookup("ZZZZ"); ok {
		t.Error("Lookup(ZZZZ) = ok, want not found")
	}
}

// TestEmbeddedData guards the shipped airports.tsv: it must load a substantial
// directory and resolve a well-known station to the expected coordinates.
func TestEmbeddedData(t *testing.T) {
	if Count() < 10000 {
		t.Fatalf("embedded directory too small: %d airports", Count())
	}
	a, ok := Lookup("EDDH")
	if !ok {
		t.Fatal("EDDH missing from embedded data")
	}
	if math.Abs(a.Lat-53.6304) > 1e-3 || math.Abs(a.Lon-9.98823) > 1e-3 {
		t.Errorf("EDDH coords = %v/%v", a.Lat, a.Lon)
	}
	if got := Search("EDDH", 5); len(got) == 0 || got[0].ICAO != "EDDH" {
		t.Errorf("embedded Search(EDDH) first = %v", icaos(got))
	}
}
