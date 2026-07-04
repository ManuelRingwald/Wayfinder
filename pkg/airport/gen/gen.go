//go:build ignore

// Command gen turns the upstream OurAirports airports.csv into the compact,
// embedded airports.tsv that pkg/airport ships (ICAO search for the admin
// view-config form). It is run by hand to refresh the data, not at build time —
// the generated airports.tsv is committed so the server needs no network and no
// external dependency (air-gapped ATC console; consistent with self-hosted
// fonts/glyphs).
//
// Source: https://github.com/davidmegginson/ourairports-data (airports.csv,
// public domain / CC0). Refresh:
//
//	curl -sSL -o airports.csv \
//	  https://raw.githubusercontent.com/davidmegginson/ourairports-data/main/airports.csv
//	go run ./pkg/airport/gen airports.csv > pkg/airport/airports.tsv
//
// Filtering: keep rows that carry a real ICAO code, valid coordinates and are
// not marked closed. Output is one tab-separated record per line —
// ICAO<TAB>NAME<TAB>LAT<TAB>LON — sorted by ICAO, deduplicated, uppercase ICAO.
// Tab format keeps the runtime loader a trivial strings.Split (no CSV parser).
package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./pkg/airport/gen <airports.csv> > airports.tsv")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fatal(err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // OurAirports rows vary; index by header instead
	header, err := r.Read()
	if err != nil {
		fatal(err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	need := []string{"type", "name", "latitude_deg", "longitude_deg", "icao_code", "ident", "gps_code"}
	for _, n := range need {
		if _, ok := col[n]; !ok {
			fatal(fmt.Errorf("input missing column %q", n))
		}
	}

	type rec struct{ icao, name, lat, lon string }
	seen := map[string]bool{}
	var out []rec
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fatal(err)
		}
		// Prefer the dedicated icao_code column; fall back to ident/gps_code,
		// which is where many smaller aerodromes carry their ICAO indicator.
		// Every candidate must be exactly four ASCII letters — a real ICAO
		// location indicator — which excludes US FAA local codes ("00A", "1B9").
		icao := pickICAO(field(row, col["icao_code"]), field(row, col["ident"]), field(row, col["gps_code"]))
		if icao == "" {
			continue // no ICAO code → not searchable by ICAO
		}
		if strings.EqualFold(strings.TrimSpace(field(row, col["type"])), "closed") {
			continue
		}
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(field(row, col["latitude_deg"])), 64)
		lon, err2 := strconv.ParseFloat(strings.TrimSpace(field(row, col["longitude_deg"])), 64)
		if err1 != nil || err2 != nil {
			continue
		}
		name := sanitize(field(row, col["name"]))
		if name == "" {
			name = icao
		}
		if seen[icao] {
			continue
		}
		seen[icao] = true
		out = append(out, rec{
			icao: icao,
			name: name,
			lat:  strconv.FormatFloat(lat, 'f', 5, 64),
			lon:  strconv.FormatFloat(lon, 'f', 5, 64),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].icao < out[j].icao })

	w := os.Stdout
	for _, a := range out {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.icao, a.name, a.lat, a.lon); err != nil {
			fatal(err)
		}
	}
	fmt.Fprintf(os.Stderr, "wrote %d airports\n", len(out))
}

func field(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

// pickICAO returns the first candidate that is a valid ICAO location indicator
// (exactly four ASCII letters, uppercased), or "" if none qualifies.
func pickICAO(candidates ...string) string {
	for _, c := range candidates {
		c = strings.ToUpper(strings.TrimSpace(c))
		if isICAO(c) {
			return c
		}
	}
	return ""
}

func isICAO(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// sanitize collapses tabs/newlines (the TSV separators) into spaces and trims,
// so a stray control character in an upstream name can never break a record.
func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen:", err)
	os.Exit(1)
}
