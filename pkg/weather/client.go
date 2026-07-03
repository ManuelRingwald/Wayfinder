// Package weather is a best-effort QNH source for the ASD (WX-B, ADR 0016). QNH
// (altimeter setting, hPa) is NOT part of the CAT062 wire contract and is NOT
// available cleanly in open DWD data — the DWD open products carry only
// mean-sea-level pressure (QFF/PMSL), a different physical quantity. The practical
// open source of real QNH is the NOAA/NWS Aviation Weather Center METAR feed
// (public domain), whose "altim" field is the altimeter setting in hPa. This
// package polls it server-side (one auditable egress, ADR 0016), caches the last
// good reading per station, and serves it to the header infobox. It never touches
// the CAT062 track path and, per "keine Fake-UI", never invents a value.
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// defaultMetarURL is the NOAA/NWS Aviation Weather Center METAR data API. It is
// public and needs no key; a distinctive User-Agent is required or requests may
// be filtered (403). Overridable via WAYFINDER_METAR_URL.
const defaultMetarURL = "https://aviationweather.gov/api/data/metar"

// defaultUserAgent identifies Wayfinder to the AWC. A blank/default UA gets
// filtered, so this is always sent. Overridable via WAYFINDER_METAR_USER_AGENT.
const defaultUserAgent = "Wayfinder-ASD/1.0 (+https://github.com/manuelringwald/wayfinder)"

// maxResponseBytes caps a METAR response read into memory (defensive consumer,
// ADR 0016). A handful of stations is a few KiB; 4 MiB is a generous ceiling.
const maxResponseBytes = 4 << 20

// Plausible QNH range (hPa). Values outside are rejected as decode noise rather
// than shown on a safety display.
const (
	minQNHHpa = 850.0
	maxQNHHpa = 1100.0
)

// inHgToHpa converts inches of mercury to hectopascals (1 inHg = 33.8639 hPa).
const inHgToHpa = 33.8639

// Report is one station's parsed QNH observation.
type Report struct {
	ICAO        string
	QNHHpa      float64
	ObsTimeUnix int64
	Raw         string
}

// Client fetches METAR reports from the AWC data API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// NewClient builds a METAR client. A nil httpClient falls back to
// http.DefaultClient (no timeout) — production injects a timed client. Empty
// baseURL/userAgent fall back to the documented defaults.
func NewClient(httpClient *http.Client, baseURL, userAgent string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultMetarURL
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = defaultUserAgent
	}
	return &Client{httpClient: httpClient, baseURL: baseURL, userAgent: userAgent}
}

// metarJSON is the tolerant view of one AWC METAR record. Unknown fields are
// ignored; missing fields decode to their zero value / nil.
type metarJSON struct {
	ICAOID  string   `json:"icaoId"`
	Altim   *float64 `json:"altim"`   // altimeter setting (QNH) in hPa
	ObsTime *int64   `json:"obsTime"` // observation time, epoch seconds
	RawOb   string   `json:"rawOb"`   // full raw METAR (carries the Q/A group)
}

// FetchMETAR retrieves METAR reports for the given ICAO ids and returns one
// Report per station that yields a plausible QNH. It is defensive: it caps the
// response size, requires a 200, decodes tolerantly, and skips any record without
// a usable QNH rather than failing the whole fetch (ADR 0016).
func (c *Client) FetchMETAR(ctx context.Context, ids []string) ([]Report, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("weather: bad METAR URL: %w", err)
	}
	q := u.Query()
	q.Set("ids", strings.Join(ids, ","))
	q.Set("format", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather: fetch METAR: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather: fetch METAR: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("weather: read METAR: %w", err)
	}

	var parsed []metarJSON
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("weather: decode METAR: %w", err)
	}

	reports := make([]Report, 0, len(parsed))
	for _, m := range parsed {
		icao := strings.ToUpper(strings.TrimSpace(m.ICAOID))
		if icao == "" {
			continue
		}
		qnh, ok := qnhFrom(m)
		if !ok {
			continue // no usable QNH — skip rather than show a wrong/absent value
		}
		var obs int64
		if m.ObsTime != nil {
			obs = *m.ObsTime
		}
		reports = append(reports, Report{ICAO: icao, QNHHpa: qnh, ObsTimeUnix: obs, Raw: m.RawOb})
	}
	return reports, nil
}

// qnhFrom extracts a plausible QNH (hPa) from a METAR record: the parsed altim
// field when present and sane, else the Q/A group parsed out of the raw report.
func qnhFrom(m metarJSON) (float64, bool) {
	if m.Altim != nil && plausibleQNH(*m.Altim) {
		return *m.Altim, true
	}
	return qnhFromRaw(m.RawOb)
}

// qGroup matches the METAR pressure group: Qxxxx (hPa, Europe) or Axxxx (inHg,
// hundredths, US). German aerodromes report Q; A is handled for robustness.
var qGroup = regexp.MustCompile(`\b([QA])(\d{4})\b`)

// qnhFromRaw parses the QNH out of a raw METAR string, handling both the hPa
// "Q" group and the inHg "A" group (converted to hPa).
func qnhFromRaw(raw string) (float64, bool) {
	mm := qGroup.FindStringSubmatch(strings.ToUpper(raw))
	if mm == nil {
		return 0, false
	}
	n, err := strconv.Atoi(mm[2])
	if err != nil {
		return 0, false
	}
	var hpa float64
	if mm[1] == "Q" {
		hpa = float64(n) // whole hPa
	} else {
		hpa = (float64(n) / 100.0) * inHgToHpa // A2992 = 29.92 inHg
	}
	if !plausibleQNH(hpa) {
		return 0, false
	}
	return hpa, true
}

func plausibleQNH(v float64) bool { return v >= minQNHHpa && v <= maxQNHHpa }
