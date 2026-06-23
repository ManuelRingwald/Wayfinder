package sensorclass

import (
	"errors"
	"reflect"
	"sort"
	"testing"
)

func TestParseLegacySpellings(t *testing.T) {
	cases := map[string]Class{
		// PSR
		"PSR": PSR, "psr": PSR, "Primary": PSR, "primary radar": PSR,
		// SSR / Mode A/C
		"SSR": SSR, "secondary": SSR, "Mode A/C": SSR, "mode-c": SSR,
		// Mode S
		"MODE_S": ModeS, "Mode S": ModeS, "mode-s": ModeS, "modes": ModeS,
		// ADS-B and its many spellings
		"ADS-B": ADSB, "ADSB": ADSB, "ads_b": ADSB, "ads b": ADSB, "1090ES": ADSB,
		// MLAT
		"MLAT": MLAT, "multilateration": MLAT, "WAM": MLAT,
		// FLARM
		"FLARM": FLARM, "flarm": FLARM,
	}
	for in, want := range cases {
		got, ok := Parse(in)
		if !ok || got != want {
			t.Errorf("Parse(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}
}

func TestParseUnknown(t *testing.T) {
	for _, in := range []string{"", "  ", "radar", "ADS", "xyz", "mode"} {
		if got, ok := Parse(in); ok {
			t.Errorf("Parse(%q) = (%q, true), want not-ok", in, got)
		}
	}
}

func TestCanonicalValuesRoundTrip(t *testing.T) {
	for _, c := range All() {
		got, ok := Parse(string(c))
		if !ok || got != c {
			t.Errorf("round-trip Parse(%q) = (%q, %v), want (%q, true)", c, got, ok, c)
		}
	}
}

func TestCanonicalizeNormalizesAndDedups(t *testing.T) {
	got, err := Canonicalize([]string{"psr", "ADS-B", "adsb", " ", "ssr", "ADSB"})
	if err != nil {
		t.Fatalf("Canonicalize err = %v", err)
	}
	// First-seen order preserved; "ADS-B"/"adsb"/"ADSB" collapse to one ADS-B.
	want := []string{"PSR", "ADS-B", "SSR"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Canonicalize = %v, want %v", got, want)
	}
}

func TestCanonicalizeRejectsUnknown(t *testing.T) {
	_, err := Canonicalize([]string{"PSR", "bogus", "SSR"})
	var uce *UnknownClassError
	if !errors.As(err, &uce) {
		t.Fatalf("Canonicalize err = %v, want *UnknownClassError", err)
	}
	if uce.Token != "bogus" {
		t.Errorf("UnknownClassError.Token = %q, want %q", uce.Token, "bogus")
	}
}

func TestCanonicalizeEmptyIsNonNil(t *testing.T) {
	for _, in := range [][]string{nil, {}, {"", "   "}} {
		got, err := Canonicalize(in)
		if err != nil {
			t.Fatalf("Canonicalize(%v) err = %v", in, err)
		}
		if got == nil {
			t.Errorf("Canonicalize(%v) = nil, want non-nil empty slice", in)
		}
		if len(got) != 0 {
			t.Errorf("Canonicalize(%v) = %v, want empty", in, got)
		}
	}
}

func TestAllSortedAndComplete(t *testing.T) {
	all := All()
	if len(all) != 6 {
		t.Fatalf("All() len = %d, want 6", len(all))
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i] < all[j] }) {
		t.Errorf("All() not sorted: %v", all)
	}
	for _, c := range all {
		if !IsKnown(c) || Describe(c) == "" {
			t.Errorf("catalogue entry %q incomplete (known=%v, desc=%q)", c, IsKnown(c), Describe(c))
		}
	}
	if IsKnown("bogus") || Describe("bogus") != "" {
		t.Error("unknown class reported as known")
	}
}
