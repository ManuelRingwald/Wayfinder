// Package airac computes the AIRAC (Aeronautical Information Regulation And
// Control) cycle for a given date (AERO-3). AIRAC effective dates repeat on a
// fixed 28-day grid anchored to a known reference; the whole calendar is therefore
// deterministic and offline-computable — no external data source is needed. It is
// used to tell the operator which cycle is current and when the next one takes
// effect, so an OpenAIP refresh can be scheduled around the AIRAC change.
package airac

import (
	"fmt"
	"time"
)

// epoch is a known AIRAC effective date used as the anchor: 2020-01-02 is cycle
// "2001" (the first cycle of 2020). Every AIRAC effective date is epoch + k·28 days.
var epoch = time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

// cycleDays is the fixed AIRAC period.
const cycleDays = 28

// Info describes the AIRAC cycle a date falls in, plus the next one.
type Info struct {
	// Ident is the cycle identifier "YYNN" (year + sequence within the year), e.g.
	// "2507" for the 7th cycle of 2025.
	Ident string `json:"ident"`
	// Effective is the current cycle's effective date (UTC midnight).
	Effective time.Time `json:"effective"`
	// NextIdent / NextEffective describe the following cycle.
	NextIdent     string    `json:"next_ident"`
	NextEffective time.Time `json:"next_effective"`
	// DaysUntilNext is whole days from the query date to NextEffective. It is 28 on
	// a cycle's own effective day (you are at the start of the cycle) and counts down
	// to 1 on the day before the next cycle.
	DaysUntilNext int `json:"days_until_next"`
}

// dayFloor normalises a time to UTC midnight so the 28-day grid arithmetic is exact
// (UTC has no DST, so day differences are whole multiples of 24h).
func dayFloor(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// daysSinceEpoch is whole days from the epoch to d (both UTC midnight); may be
// negative for dates before the epoch.
func daysSinceEpoch(d time.Time) int {
	return int(d.Sub(epoch).Hours() / 24)
}

// floorDiv is integer division rounding toward negative infinity (correct cycle
// index for dates before the epoch, unlike Go's truncating /).
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// effectiveForIndex returns the effective date of the k-th cycle since the epoch.
func effectiveForIndex(k int) time.Time { return epoch.AddDate(0, 0, k*cycleDays) }

// Cycle returns the AIRAC Info for the date containing t.
func Cycle(t time.Time) Info {
	day := dayFloor(t)
	k := floorDiv(daysSinceEpoch(day), cycleDays)
	eff := effectiveForIndex(k)
	next := effectiveForIndex(k + 1)
	return Info{
		Ident:         identFor(eff),
		Effective:     eff,
		NextIdent:     identFor(next),
		NextEffective: next,
		DaysUntilNext: int(next.Sub(day).Hours() / 24),
	}
}

// identFor builds the "YYNN" identifier for a cycle effective date: the year's last
// two digits plus the 1-based sequence of the cycle within that calendar year.
func identFor(eff time.Time) string {
	year := eff.Year()
	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	// The first AIRAC effective date on/after Jan 1 of this year.
	k0 := ceilDiv(daysSinceEpoch(jan1), cycleDays)
	first := effectiveForIndex(k0)
	seq := int(eff.Sub(first).Hours()/24)/cycleDays + 1
	return fmt.Sprintf("%02d%02d", year%100, seq)
}

// ceilDiv is integer division rounding toward positive infinity.
func ceilDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) == (b < 0)) {
		q++
	}
	return q
}
