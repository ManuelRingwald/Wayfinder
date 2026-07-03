package airac

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestCycleAtEpoch(t *testing.T) {
	// The anchor: 2020-01-02 is cycle "2001".
	got := Cycle(d(2020, 1, 2))
	if got.Ident != "2001" {
		t.Errorf("ident = %q, want 2001", got.Ident)
	}
	if !got.Effective.Equal(d(2020, 1, 2)) {
		t.Errorf("effective = %v, want 2020-01-02", got.Effective)
	}
	if !got.NextEffective.Equal(d(2020, 1, 30)) || got.NextIdent != "2002" {
		t.Errorf("next = %v/%q, want 2020-01-30/2002", got.NextEffective, got.NextIdent)
	}
}

func TestCycleMidCycleUsesCurrentEffective(t *testing.T) {
	// Any day within [effective, effective+28) maps to the same cycle.
	got := Cycle(d(2020, 1, 15)) // 13 days into 2001
	if got.Ident != "2001" || !got.Effective.Equal(d(2020, 1, 2)) {
		t.Errorf("mid-cycle = %q/%v, want 2001/2020-01-02", got.Ident, got.Effective)
	}
	if got.DaysUntilNext != 15 { // 2020-01-30 minus 2020-01-15
		t.Errorf("days until next = %d, want 15", got.DaysUntilNext)
	}
}

func TestDaysUntilNextOnEffectiveDay(t *testing.T) {
	// On a cycle's own effective day you are at the START of that cycle, so the
	// FOLLOWING cycle is a full 28 days away.
	got := Cycle(d(2020, 1, 30))
	if got.DaysUntilNext != 28 {
		t.Errorf("on the effective day, days until next = %d, want 28", got.DaysUntilNext)
	}
	if got.Ident != "2002" {
		t.Errorf("ident = %q, want 2002", got.Ident)
	}
	// The day before the next cycle, DaysUntilNext is 1.
	if before := Cycle(d(2020, 2, 26)).DaysUntilNext; before != 1 {
		t.Errorf("day before next cycle, days until next = %d, want 1", before)
	}
}

func TestIdentIncrementsAndResetsPerYear(t *testing.T) {
	// Sequence increments by one every 28 days from the anchor.
	for i, want := range []string{"2001", "2002", "2003", "2004", "2005"} {
		eff := d(2020, 1, 2).AddDate(0, 0, i*28)
		if got := Cycle(eff).Ident; got != want {
			t.Errorf("cycle %d ident = %q, want %q", i, got, want)
		}
	}
	// The cycle effective 2020-12-31 spans into 2021; the FIRST cycle whose effective
	// date lands in 2021 is 2021-01-28 (2020-12-31 + 28) and resets to "2101".
	first2021 := Cycle(d(2021, 1, 28))
	if first2021.Effective.Year() != 2021 {
		t.Fatalf("expected an effective date in 2021, got %v", first2021.Effective)
	}
	if first2021.Ident != "2101" {
		t.Errorf("first 2021 cycle ident = %q, want 2101", first2021.Ident)
	}
	// The last cycle effective in 2020 (2020-12-31) must be "20xx", not "21xx".
	prev := Cycle(first2021.Effective.AddDate(0, 0, -1))
	if prev.Effective.Year() != 2020 || prev.Ident[:2] != "20" {
		t.Errorf("cycle before the 2021 reset = %q @ %v, want a 20xx ident in 2020", prev.Ident, prev.Effective)
	}
}

func TestConsecutiveCyclesAre28DaysApart(t *testing.T) {
	c := Cycle(d(2025, 6, 15))
	if diff := c.NextEffective.Sub(c.Effective); diff != 28*24*time.Hour {
		t.Errorf("cycle length = %v, want 28 days", diff)
	}
	// The next cycle's Cycle() must report the same effective date as this one's Next.
	n := Cycle(c.NextEffective)
	if !n.Effective.Equal(c.NextEffective) || n.Ident != c.NextIdent {
		t.Errorf("next-cycle mismatch: %q/%v vs %q/%v", n.Ident, n.Effective, c.NextIdent, c.NextEffective)
	}
}

func TestCycleNormalisesTimeOfDay(t *testing.T) {
	// A timestamp late in the day maps to the same cycle as its midnight.
	withTime := time.Date(2020, 1, 15, 23, 59, 0, 0, time.UTC)
	if Cycle(withTime).Ident != Cycle(d(2020, 1, 15)).Ident {
		t.Error("time-of-day must not change the cycle")
	}
}
