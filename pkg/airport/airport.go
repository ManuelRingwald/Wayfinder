// Package airport is an offline ICAO aerodrome directory for the admin
// view-config form: the operator types an ICAO code (or a name fragment) when
// setting a tenant's default map centre, and the matching airport's coordinates
// are filled in — no hand-copied lat/lon, no "0/0" typos.
//
// The data is embedded (airports.tsv, generated from the public-domain
// OurAirports dataset — see gen/gen.go), so the lookup needs no network, no API
// key and no external service. That is deliberate: it fits the air-gapped ATC
// console posture (like the self-hosted fonts/glyphs) and keeps this admin
// convenience available even when OpenAIP/METAR egress is not. Airports do not
// move, so a static snapshot is authoritative; refresh it with gen/ when the
// upstream data updates.
package airport

import (
	_ "embed"
	"sort"
	"strconv"
	"strings"
)

//go:embed airports.tsv
var raw string

// minQueryLen guards against pathological queries: a single letter would match
// thousands of ICAO prefixes. Two characters keeps result sets sane while still
// allowing name fragments like "JFK".
const minQueryLen = 2

// Airport is one aerodrome returned by the search. JSON tags match the admin API
// DTO the frontend consumes.
type Airport struct {
	ICAO string  `json:"icao"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

// entry pairs an airport with its lower-cased name so name-substring matching
// does not re-lower the whole directory on every query.
type entry struct {
	a         Airport
	nameLower string
}

// Index is a searchable, in-memory airport directory.
type Index struct {
	entries []entry
	byICAO  map[string]Airport
}

// defaultIndex is parsed once from the embedded data at package init. Parsing is
// tolerant: a malformed line is skipped, never fatal — the directory is a
// best-effort convenience, not a safety-critical path.
var defaultIndex = parse(raw)

// Search runs against the embedded directory. See (*Index).Search.
func Search(q string, limit int) []Airport { return defaultIndex.Search(q, limit) }

// Lookup returns the airport for an exact ICAO code (case-insensitive), or
// false if unknown.
func Lookup(icao string) (Airport, bool) { return defaultIndex.Lookup(icao) }

// Count reports how many airports are loaded (for startup logging / sanity).
func Count() int { return len(defaultIndex.entries) }

// parse builds an Index from tab-separated ICAO<TAB>NAME<TAB>LAT<TAB>LON lines.
func parse(data string) *Index {
	ix := &Index{byICAO: map[string]Airport{}}
	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			continue
		}
		lat, err1 := strconv.ParseFloat(f[2], 64)
		lon, err2 := strconv.ParseFloat(f[3], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		a := Airport{ICAO: f[0], Name: f[1], Lat: lat, Lon: lon}
		if _, dup := ix.byICAO[a.ICAO]; dup {
			continue
		}
		ix.byICAO[a.ICAO] = a
		ix.entries = append(ix.entries, entry{a: a, nameLower: strings.ToLower(a.Name)})
	}
	return ix
}

// Lookup returns the airport for an exact ICAO code (case-insensitive).
func (ix *Index) Lookup(icao string) (Airport, bool) {
	a, ok := ix.byICAO[strings.ToUpper(strings.TrimSpace(icao))]
	return a, ok
}

// Search returns up to limit airports matching q, best match first. Matching is
// tiered so an exact ICAO always wins:
//
//	rank 0 — exact ICAO code
//	rank 1 — ICAO prefix (e.g. "EDD" → EDDH, EDDF, …)
//	rank 2 — name substring (e.g. "hamburg" → EDDH, …)
//
// Within a rank, results are ordered by ICAO for a stable, predictable list.
// A query shorter than minQueryLen (after trimming) returns nothing.
func (ix *Index) Search(q string, limit int) []Airport {
	q = strings.TrimSpace(q)
	if len(q) < minQueryLen {
		return nil
	}
	if limit <= 0 {
		limit = 10
	}
	qUpper := strings.ToUpper(q)
	qLower := strings.ToLower(q)

	type hit struct {
		a    Airport
		rank int
	}
	var hits []hit
	for _, e := range ix.entries {
		rank := -1
		switch {
		case e.a.ICAO == qUpper:
			rank = 0
		case strings.HasPrefix(e.a.ICAO, qUpper):
			rank = 1
		case strings.Contains(e.nameLower, qLower):
			rank = 2
		}
		if rank >= 0 {
			hits = append(hits, hit{a: e.a, rank: rank})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		return hits[i].a.ICAO < hits[j].a.ICAO
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Airport, len(hits))
	for i, h := range hits {
		out[i] = h.a
	}
	return out
}
