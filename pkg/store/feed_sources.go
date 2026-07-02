package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/sensorclass"
)

// SourceType is a generic, Firefly-agnostic class of live input a feed's tracker
// instance opens (ADR 0012 §2). The catalogue is closed — like the sensor-mix
// vocabulary (pkg/sensorclass) — so the orchestrator never has to translate a
// typo'd kind into a Firefly launch, and the admin UI can offer a fixed builder.
// New kinds are added here additively; the JSONB column needs no migration.
type SourceType string

const (
	// SourceADSBOpenSky polls the OpenSky REST API (states/all) inside a query
	// bbox — a crowdsourced internet source, area-bounded, optional credentials.
	SourceADSBOpenSky SourceType = "adsb_opensky"
	// SourceFLARMAPRS ingests OGN/APRS-IS (GA/gliders) inside a query bbox.
	SourceFLARMAPRS SourceType = "flarm_aprs"
	// SourceRadarASTERIX ingests a real surveillance sensor (CAT048/CAT001,
	// Firefly SDPS-001); the sensor is identified by its SAC/SIC, not a bbox.
	SourceRadarASTERIX SourceType = "radar_asterix"
)

// knownSourceTypes is the closed catalogue; isAreaBounded marks the kinds whose
// coverage is a geographic query window (vs. a physical sensor with a SAC/SIC).
var knownSourceTypes = map[SourceType]bool{
	SourceADSBOpenSky:  true,
	SourceFLARMAPRS:    true,
	SourceRadarASTERIX: true,
}

func (t SourceType) isAreaBounded() bool {
	return t == SourceADSBOpenSky || t == SourceFLARMAPRS
}

// sensorClassBySourceType maps each live source kind to the surveillance sensor
// class it contributes, so a feed's sensor mix (feed metadata, pkg/sensorclass)
// can be DERIVED from its configured sources instead of hand-maintained
// (Issue #102). radar_asterix is treated as secondary surveillance (SSR/Mode S):
// CAT048 target reports carry a SAC/SIC identity. The mapping is intentionally
// coarse — the sensor mix is informational metadata, not a rendering input.
var sensorClassBySourceType = map[SourceType]sensorclass.Class{
	SourceADSBOpenSky:  sensorclass.ADSB,
	SourceFLARMAPRS:    sensorclass.FLARM,
	SourceRadarASTERIX: sensorclass.SSR,
}

// DerivedSensorMix returns the feed's sensor mix implied by its configured source
// types — deduplicated and sorted for a stable value. Once a feed has sources it
// is the single source of truth for the feed's sensor metadata (Issue #102),
// replacing the manually entered mix. An empty source list yields an empty mix.
func (c SourceConfig) DerivedSensorMix() []string {
	seen := map[sensorclass.Class]bool{}
	for _, s := range c {
		if cl, ok := sensorClassBySourceType[s.Type]; ok {
			seen[cl] = true
		}
	}
	out := make([]string, 0, len(seen))
	for cl := range seen {
		out = append(out, string(cl))
	}
	sort.Strings(out)
	return out
}

// Source is one configured live input for a feed's tracker. The fields that
// apply depend on Type: area-bounded internet sources carry a BBox (and may
// carry a CredRef to a per-feed secret); a real radar carries a SAC/SIC sensor
// identity. CredRef is only a *reference* (e.g. "secret/speyer-opensky"), never
// the secret itself — the secret value is held separately and handed to the
// InstanceBackend at launch (ADR 0012 §6, ORCH-2), never returned to the browser.
type Source struct {
	Type    SourceType `json:"type"`
	BBox    *BBox      `json:"bbox,omitempty"`
	SAC     *int       `json:"sac,omitempty"`
	SIC     *int       `json:"sic,omitempty"`
	CredRef *string    `json:"cred_ref,omitempty"`
	// Radar location (radar_asterix only, Firefly contract v1.3.0 / #91): CAT048
	// is polar *relative to the radar* and does not carry the site, so Firefly
	// needs Lat/Lon (WGS84 degrees, required) to lift polar plots into the
	// tracking frame. HeightM (metres above the WGS84 ellipsoid, default 0) and
	// Listen (UDP endpoint "group:port" for the ASTERIX input) are optional.
	Lat     *float64 `json:"lat,omitempty"`
	Lon     *float64 `json:"lon,omitempty"`
	HeightM *float64 `json:"height_m,omitempty"`
	Listen  string   `json:"listen,omitempty"`
}

// SourceConfig is the ordered list of a feed's live inputs.
type SourceConfig []Source

// InvalidSourceError reports a source entry that fails validation. It carries the
// entry index so the admin API (ORCH-1b) can point at the offending row.
type InvalidSourceError struct {
	Index  int
	Reason string
}

func (e *InvalidSourceError) Error() string {
	return fmt.Sprintf("store: source[%d]: %s", e.Index, e.Reason)
}

// Validate checks every source against the closed catalogue and its per-kind
// rules, so the catalogue never stores a configuration the orchestrator cannot
// launch (defense at the write boundary, like the sensor-mix check, WF2-41). A
// nil/empty config is valid: it describes a feed with no live source yet (a
// scenario/placeholder tracker). Errors are *InvalidSourceError (errors.As-able).
func (sc SourceConfig) Validate() error {
	for i, s := range sc {
		if err := s.validate(i); err != nil {
			return err
		}
	}
	return nil
}

func (s Source) validate(idx int) error {
	if !knownSourceTypes[s.Type] {
		return &InvalidSourceError{Index: idx, Reason: fmt.Sprintf("unknown source type %q", s.Type)}
	}
	if s.BBox != nil {
		if err := s.BBox.validate(); err != nil {
			return &InvalidSourceError{Index: idx, Reason: err.Error()}
		}
	}
	if s.CredRef != nil {
		ref := strings.TrimSpace(*s.CredRef)
		if ref == "" {
			return &InvalidSourceError{Index: idx, Reason: "cred_ref must not be blank when present"}
		}
		if len(ref) > 200 {
			return &InvalidSourceError{Index: idx, Reason: "cred_ref too long"}
		}
	}

	if s.Type.isAreaBounded() {
		if s.BBox == nil {
			return &InvalidSourceError{Index: idx, Reason: fmt.Sprintf("%s requires a bbox", s.Type)}
		}
		if s.SAC != nil || s.SIC != nil {
			return &InvalidSourceError{Index: idx, Reason: fmt.Sprintf("%s has no sensor identity (sac/sic not allowed)", s.Type)}
		}
		if s.Lat != nil || s.Lon != nil || s.HeightM != nil || s.Listen != "" {
			return &InvalidSourceError{Index: idx, Reason: fmt.Sprintf("%s has no radar location (lat/lon/height_m/listen not allowed)", s.Type)}
		}
		return nil
	}

	// radar_asterix: a physical sensor, identified by SAC/SIC; bbox optional.
	if s.SAC == nil || s.SIC == nil {
		return &InvalidSourceError{Index: idx, Reason: "radar_asterix requires sac and sic"}
	}
	if *s.SAC < 0 || *s.SAC > 255 || *s.SIC < 0 || *s.SIC > 255 {
		return &InvalidSourceError{Index: idx, Reason: "sac/sic must be in 0..255"}
	}
	// Radar location is required (contract v1.3.0, #91): CAT048 is polar and does
	// not carry the site, so without it Firefly cannot geolocate the plots and
	// the spawned instance would abort at startup — reject at the write boundary.
	if s.Lat == nil || s.Lon == nil {
		return &InvalidSourceError{Index: idx, Reason: "radar_asterix requires lat and lon (radar site)"}
	}
	if *s.Lat < -90 || *s.Lat > 90 {
		return &InvalidSourceError{Index: idx, Reason: "radar lat out of range [-90,90]"}
	}
	if *s.Lon < -180 || *s.Lon > 180 {
		return &InvalidSourceError{Index: idx, Reason: "radar lon out of range [-180,180]"}
	}
	return nil
}

// validate checks a bbox is well-formed in WGS84 (lat/lon in range, min ≤ max).
func (b BBox) validate() error {
	if b.MinLat < -90 || b.MinLat > 90 || b.MaxLat < -90 || b.MaxLat > 90 {
		return fmt.Errorf("bbox latitude out of range [-90,90]")
	}
	if b.MinLon < -180 || b.MinLon > 180 || b.MaxLon < -180 || b.MaxLon > 180 {
		return fmt.Errorf("bbox longitude out of range [-180,180]")
	}
	if b.MinLat > b.MaxLat || b.MinLon > b.MaxLon {
		return fmt.Errorf("bbox min must not exceed max")
	}
	return nil
}

// CoverageBBox derives the coarse outer geographic bound for the feed's tracker:
// the union of all source bboxes, expanded by marginKm and clamped to valid
// WGS84 ranges. This is the generic Coverage Wayfinder hands to Firefly
// (FIREFLY_COVERAGE_BBOX) — a deliberately *loose* outer limit, distinct from the
// tenant's precise inner AOI (ADR 0012 §3). It returns nil when no source carries
// a bbox (e.g. a radar-only feed), in which case coverage is left unset.
//
// The longitude margin uses the box edge with the largest |latitude| (where a
// degree of longitude is shortest), so the expansion is at least marginKm
// everywhere in the box — conservative on purpose for a coarse outer bound.
func (sc SourceConfig) CoverageBBox(marginKm float64) *BBox {
	var u *BBox
	for _, s := range sc {
		if s.BBox == nil {
			continue
		}
		if u == nil {
			b := *s.BBox
			u = &b
			continue
		}
		u.MinLat = math.Min(u.MinLat, s.BBox.MinLat)
		u.MinLon = math.Min(u.MinLon, s.BBox.MinLon)
		u.MaxLat = math.Max(u.MaxLat, s.BBox.MaxLat)
		u.MaxLon = math.Max(u.MaxLon, s.BBox.MaxLon)
	}
	if u == nil {
		return nil
	}
	if marginKm > 0 {
		const kmPerDegLat = 111.0
		latMargin := marginKm / kmPerDegLat
		u.MinLat = clamp(u.MinLat-latMargin, -90, 90)
		u.MaxLat = clamp(u.MaxLat+latMargin, -90, 90)

		latForLon := math.Max(math.Abs(u.MinLat), math.Abs(u.MaxLat))
		cosLat := math.Cos(latForLon * math.Pi / 180)
		var lonMargin float64
		if cosLat < 1e-6 {
			lonMargin = 180 // near the poles, longitude collapses; widen fully
		} else {
			lonMargin = marginKm / (kmPerDegLat * cosLat)
		}
		u.MinLon = clamp(u.MinLon-lonMargin, -180, 180)
		u.MaxLon = clamp(u.MaxLon+lonMargin, -180, 180)
	}
	return u
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// GetSourceConfig returns a feed's source configuration and its derived coverage
// bbox (nil when unset). A missing feed yields ErrNotFound. The config is read
// via a dedicated accessor (not folded into the general Feed row) so the lean
// catalogue list/get queries stay free of the JSONB payload.
func (r *FeedRepo) GetSourceConfig(ctx context.Context, feedID int64) (SourceConfig, *BBox, error) {
	const q = `SELECT source_config, coverage_bbox FROM feeds WHERE id = $1`
	var (
		rawSources  []byte
		rawCoverage []byte
	)
	if err := r.db.QueryRow(ctx, q, feedID).Scan(&rawSources, &rawCoverage); err != nil {
		return nil, nil, wrap("get feed source config", err)
	}
	var sources SourceConfig
	if err := fromJSONB(rawSources, &sources); err != nil {
		return nil, nil, wrap("get feed source config: decode sources", err)
	}
	var coverage *BBox
	if err := fromJSONB(rawCoverage, &coverage); err != nil {
		return nil, nil, wrap("get feed source config: decode coverage", err)
	}
	return sources, coverage, nil
}

// SetSourceConfig replaces a feed's source configuration and coverage bbox. The
// config is validated first, so a rejected configuration never reaches the
// database (the error is *InvalidSourceError). A nil coverage stores SQL NULL; a
// nil/empty config stores an empty array. A missing feed yields ErrNotFound.
func (r *FeedRepo) SetSourceConfig(ctx context.Context, feedID int64, sources SourceConfig, coverage *BBox) error {
	if err := sources.Validate(); err != nil {
		return wrap("set feed source config", err)
	}
	if sources == nil {
		sources = SourceConfig{}
	}
	srcJSON, err := toJSONB(sources)
	if err != nil {
		return wrap("set feed source config: marshal sources", err)
	}
	var covParam any
	if coverage != nil {
		if err := coverage.validate(); err != nil {
			return wrap("set feed source config: coverage", err)
		}
		s, e := toJSONB(coverage)
		if e != nil {
			return wrap("set feed source config: marshal coverage", e)
		}
		covParam = s
	}
	// The sensor mix is derived from the source types (Issue #102) and written in
	// the same statement, so a feed's sensor metadata always mirrors its actual
	// sources — no separate, hand-maintained field to drift out of sync.
	mixJSON, err := toJSONB(sources.DerivedSensorMix())
	if err != nil {
		return wrap("set feed source config: marshal sensor mix", err)
	}
	const q = `UPDATE feeds SET source_config = $2::jsonb, coverage_bbox = $3::jsonb, sensor_mix = $4::jsonb WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, feedID, srcJSON, covParam, mixJSON)
	if err != nil {
		return wrap("set feed source config", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
