//go:build ignore

// Command genrunways turns the upstream OurAirports runways.csv into the compact,
// embedded runways.tsv that pkg/airport ships (#192, the runway overlay). Like the
// airports generator it is run by hand to refresh the data, not at build time —
// the generated runways.tsv is committed so the server needs no network and no
// external dependency (air-gapped ATC console).
//
// Source: https://github.com/davidmegginson/ourairports-data (runways.csv,
// public domain / CC0). Refresh:
//
//	curl -sSL -o runways.csv \
//	  https://raw.githubusercontent.com/davidmegginson/ourairports-data/main/runways.csv
//	go run ./pkg/airport/gen/runways.go runways.csv > pkg/airport/runways.tsv
//
// Filtering: keep runways that are NOT closed, carry both LE and HE threshold
// coordinates, and belong to an airport with a real 4-letter ICAO ident (so a
// runway aligns with the ICAO airport directory and the size stays bounded —
// US local-code fields like "00A" are excluded, matching airports.tsv). Output is
// one tab-separated record per line:
//
//	ICAO<TAB>RWY_IDENT<TAB>LE_LAT<TAB>LE_LON<TAB>HE_LAT<TAB>HE_LON
//
// sorted by ICAO then runway ident. Tab format keeps the runtime loader a trivial
// strings.Split (no CSV parser).
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
		fmt.Fprintln(os.Stderr, "usage: go run ./pkg/airport/gen/runways.go <runways.csv> > runways.tsv")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fatal(err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		fatal(err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	need := []string{
		"airport_ident", "closed", "le_ident", "he_ident",
		"le_latitude_deg", "le_longitude_deg", "he_latitude_deg", "he_longitude_deg",
	}
	for _, n := range need {
		if _, ok := col[n]; !ok {
			fatal(fmt.Errorf("input missing column %q", n))
		}
	}

	type rec struct{ icao, ident, leLat, leLon, heLat, heLon string }
	var out []rec
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fatal(err)
		}
		if field(row, col["closed"]) == "1" {
			continue
		}
		icao := strings.ToUpper(strings.TrimSpace(field(row, col["airport_ident"])))
		if !isICAO(icao) {
			continue // not an ICAO-coded aerodrome → skip (bounds size, aligns with airports.tsv)
		}
		leLat, e1 := strconv.ParseFloat(strings.TrimSpace(field(row, col["le_latitude_deg"])), 64)
		leLon, e2 := strconv.ParseFloat(strings.TrimSpace(field(row, col["le_longitude_deg"])), 64)
		heLat, e3 := strconv.ParseFloat(strings.TrimSpace(field(row, col["he_latitude_deg"])), 64)
		heLon, e4 := strconv.ParseFloat(strings.TrimSpace(field(row, col["he_longitude_deg"])), 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			continue // need both thresholds to draw the centreline
		}
		le := sanitize(field(row, col["le_ident"]))
		he := sanitize(field(row, col["he_ident"]))
		ident := strings.TrimSpace(strings.Trim(le+"/"+he, "/"))
		out = append(out, rec{
			icao:  icao,
			ident: ident,
			leLat: strconv.FormatFloat(leLat, 'f', 6, 64),
			leLon: strconv.FormatFloat(leLon, 'f', 6, 64),
			heLat: strconv.FormatFloat(heLat, 'f', 6, 64),
			heLon: strconv.FormatFloat(heLon, 'f', 6, 64),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].icao != out[j].icao {
			return out[i].icao < out[j].icao
		}
		return out[i].ident < out[j].ident
	})

	w := os.Stdout
	for _, a := range out {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", a.icao, a.ident, a.leLat, a.leLon, a.heLat, a.heLon); err != nil {
			fatal(err)
		}
	}
	fmt.Fprintf(os.Stderr, "wrote %d runways\n", len(out))
}

func field(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
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
	fmt.Fprintln(os.Stderr, "genrunways:", err)
	os.Exit(1)
}
