// Package basemapsearch builds a per-sector search index from the official
// base map's vector tiles (#277, ADR 0028) — the operator's "Kandidat D".
//
// Why not a geocoder: the controller does not need a Germany-wide address
// search, only "find the Friedrichstraße in MY sector, fast" (e.g. a drone
// launching from a named street). So the server downloads the DETAIL tiles
// (street names live only at high zoom) of the tenant's bounded AOI ONCE,
// decodes the MVT protobufs and extracts every named feature into an
// in-memory index — no licence question (the tiles are the same free
// basemap.de data the map renders), air-gap capable (works against the H3
// mirror unchanged), and honest about its limits: street RUNS, not house
// numbers, and simple substring matching, not typo-tolerant geocoding.
//
// Bounds by design (operator decision): a hard tile cap clamps oversized AOIs
// around their centre (tiles.go), indexes are built lazily per AOI with an
// LRU cap, and every upstream read is size- and time-limited (defensive
// consumer, CLAUDE.md §7).
package basemapsearch

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/maptile"
)

// StyleProvider yields the (rewritten, URL-absolutised) base-map style JSON —
// implemented by pkg/basemap.Service, so the index always reads the SAME
// source the map renders (online BKG or the operator's H3 mirror).
type StyleProvider interface {
	StyleJSON(ctx context.Context) ([]byte, error)
}

// Config bounds the index. Zero values apply the documented defaults.
type Config struct {
	// IndexZoom is the tile zoom the index reads. Street names appear in the
	// basemap.de detail tiles; 14 is the established level.
	IndexZoom int
	// MaxTiles caps one index build (W2: ≈4096 ≈ a 50 NM radius at German
	// latitudes). Larger AOIs are clamped around their centre.
	MaxTiles int
	// MaxIndexes caps concurrently cached per-AOI indexes (LRU eviction).
	MaxIndexes int
	// MaxEntries caps one index's extracted features (runaway guard).
	MaxEntries int
	// TTL is how long a built index is served before a background rebuild
	// (tiles update monthly; daily is generous).
	TTL time.Duration
	// Concurrency is the parallel tile-fetch fan-out per build.
	Concurrency int
}

const (
	defaultIndexZoom   = 14
	defaultMaxTiles    = 4096
	defaultMaxIndexes  = 8
	defaultMaxEntries  = 250_000
	defaultTTL         = 24 * time.Hour
	defaultConcurrency = 8
	maxTileBytes       = 4 << 20
	maxTileJSONBytes   = 1 << 20
	buildTimeout       = 5 * time.Minute
	// clusterKM merges same-named features closer than this (a street crosses
	// many tiles; the list should show one entry per distinct location, not
	// one per tile).
	clusterKM = 3.0
)

// Entry is one searchable named feature.
type Entry struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	norm     string
}

// Result is the search response payload.
type Result struct {
	Status  string  `json:"status"` // "ready" | "building" | "error"
	Results []Entry `json:"results,omitempty"`
}

type index struct {
	bbox     BBox
	entries  []Entry
	builtAt  time.Time
	err      error
	building bool
	lastUsed time.Time
}

// Service builds and serves per-AOI search indexes.
type Service struct {
	styles     StyleProvider
	httpClient *http.Client
	cfg        Config
	logger     *slog.Logger
	now        func() time.Time

	mu      sync.Mutex
	indexes map[string]*index

	buildSuccess atomic.Int64
	buildFailure atomic.Int64
	searchCount  atomic.Int64
}

// NewService builds a search Service. A nil httpClient falls back to
// http.DefaultClient — production injects a timed client.
func NewService(styles StyleProvider, httpClient *http.Client, cfg Config, logger *slog.Logger) *Service {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.IndexZoom <= 0 {
		cfg.IndexZoom = defaultIndexZoom
	}
	if cfg.MaxTiles <= 0 {
		cfg.MaxTiles = defaultMaxTiles
	}
	if cfg.MaxIndexes <= 0 {
		cfg.MaxIndexes = defaultMaxIndexes
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = defaultMaxEntries
	}
	if cfg.TTL <= 0 {
		cfg.TTL = defaultTTL
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = defaultConcurrency
	}
	return &Service{
		styles:     styles,
		httpClient: httpClient,
		cfg:        cfg,
		logger:     logger,
		now:        time.Now,
		indexes:    make(map[string]*index),
	}
}

// BuildSuccessCount, BuildFailureCount and SearchCount expose the metrics trio.
func (s *Service) BuildSuccessCount() int64 { return s.buildSuccess.Load() }

// BuildFailureCount returns the total number of failed index builds.
func (s *Service) BuildFailureCount() int64 { return s.buildFailure.Load() }

// SearchCount returns the total number of executed searches.
func (s *Service) SearchCount() int64 { return s.searchCount.Load() }

// Search answers a query against the index for bbox. A missing index kicks off
// an asynchronous build and reports "building" immediately (single-flight per
// AOI) — the UI polls; a request must never hang for a minute-long build. A
// stale index serves its last state while a background rebuild replaces it.
func (s *Service) Search(bbox BBox, q string) Result {
	s.searchCount.Add(1)
	key := bboxKey(bbox)

	s.mu.Lock()
	idx, ok := s.indexes[key]
	if !ok {
		idx = &index{bbox: bbox, building: true, lastUsed: s.now()}
		s.indexes[key] = idx
		s.evictLocked()
		go s.build(key, bbox)
		s.mu.Unlock()
		return Result{Status: "building"}
	}
	idx.lastUsed = s.now()
	if idx.builtAt.IsZero() {
		// Never built successfully. A failed first build is reported HONESTLY as
		// "error" (operator finding 2026-07-19: reporting "building" here hid an
		// endless fail-retry loop behind a perpetual spinner). The error sticks
		// across background retries and only a successful build clears it, so
		// the status is stable instead of flapping error↔building.
		if idx.err != nil {
			if !idx.building {
				idx.building = true
				go s.build(key, bbox)
			}
			s.mu.Unlock()
			return Result{Status: "error"}
		}
		s.mu.Unlock()
		return Result{Status: "building"}
	}
	if !idx.building && (idx.err != nil || s.now().Sub(idx.builtAt) > s.cfg.TTL) {
		idx.building = true // serve stale below, refresh in the background
		go s.build(key, bbox)
	}
	entries := idx.entries
	s.mu.Unlock()

	return Result{Status: "ready", Results: match(entries, q)}
}

// match filters + ranks: prefix hits first, then shorter names, then
// alphabetically; capped at 20 rows for the dropdown.
func match(entries []Entry, q string) []Entry {
	nq := normalizeName(q)
	if len(nq) < 2 {
		return []Entry{}
	}
	var hits []Entry
	for _, e := range entries {
		if strings.Contains(e.norm, nq) {
			hits = append(hits, e)
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		pi, pj := strings.HasPrefix(hits[i].norm, nq), strings.HasPrefix(hits[j].norm, nq)
		if pi != pj {
			return pi
		}
		if len(hits[i].norm) != len(hits[j].norm) {
			return len(hits[i].norm) < len(hits[j].norm)
		}
		return hits[i].Name < hits[j].Name
	})
	if len(hits) > 20 {
		hits = hits[:20]
	}
	if hits == nil {
		hits = []Entry{}
	}
	return hits
}

// build downloads + decodes the AOI's tiles and swaps the finished index in.
// Runs detached from any request (its own timeout): index building is server
// work, not request work.
func (s *Service) build(key string, bbox BBox) {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	entries, err := s.buildEntries(ctx, bbox)

	s.mu.Lock()
	idx, ok := s.indexes[key]
	if !ok { // evicted while building — drop the result
		s.mu.Unlock()
		return
	}
	idx.building = false
	if err != nil {
		idx.err = err
		s.buildFailure.Add(1)
		s.mu.Unlock()
		s.logger.Warn("basemap search index build failed", slog.String("error", err.Error()))
		return
	}
	idx.entries = entries
	idx.builtAt = s.now()
	idx.err = nil
	s.buildSuccess.Add(1)
	s.mu.Unlock()
	s.logger.Info("basemap search index built",
		slog.Int("entries", len(entries)), slog.String("aoi", key))
}

func (s *Service) buildEntries(ctx context.Context, bbox BBox) ([]Entry, error) {
	tmpl, err := s.tilesTemplate(ctx)
	if err != nil {
		return nil, err
	}
	r := tilesForBBox(bbox, s.cfg.IndexZoom, s.cfg.MaxTiles)
	if r.clamped {
		s.logger.Warn("basemap search AOI exceeds tile cap; index clamped around AOI centre",
			slog.Int("requested_tiles", r.requestedTileCount), slog.Int("cap", s.cfg.MaxTiles))
	}

	type tileJob struct{ x, y int }
	jobs := make(chan tileJob)
	var wg sync.WaitGroup
	var cmu sync.Mutex
	clusters := map[string][]Entry{} // normalized name → merged locations
	total := 0

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			ents := s.fetchTileEntries(ctx, tmpl, j.x, j.y, r.zoom)
			if len(ents) == 0 {
				continue
			}
			cmu.Lock()
			for _, e := range ents {
				if total >= s.cfg.MaxEntries {
					break
				}
				if addClustered(clusters, e) {
					total++
				}
			}
			cmu.Unlock()
		}
	}
	wg.Add(s.cfg.Concurrency)
	for i := 0; i < s.cfg.Concurrency; i++ {
		go worker()
	}
	for x := r.minX; x <= r.maxX; x++ {
		for y := r.minY; y <= r.maxY; y++ {
			select {
			case jobs <- tileJob{x, y}:
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				return nil, ctx.Err()
			}
		}
	}
	close(jobs)
	wg.Wait()

	if total >= s.cfg.MaxEntries {
		s.logger.Warn("basemap search index hit the entry cap; results may be incomplete",
			slog.Int("cap", s.cfg.MaxEntries))
	}
	var entries []Entry
	for _, pts := range clusters {
		entries = append(entries, pts...)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("basemapsearch: no named features extracted (%d tiles)", r.count())
	}
	return entries, nil
}

// fetchTileEntries downloads one tile and extracts its named features.
// Per-tile failures are logged-and-skipped: one missing tile must not sink a
// 4000-tile build.
func (s *Service) fetchTileEntries(ctx context.Context, tmpl string, x, y, z int) []Entry {
	u := strings.NewReplacer(
		"{z}", fmt.Sprint(z), "{x}", fmt.Sprint(x), "{y}", fmt.Sprint(y),
	).Replace(tmpl)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Debug("basemap search tile fetch failed", slog.String("url", u), slog.String("error", err.Error()))
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil // empty tile — normal over water/abroad
	}
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTileBytes))
	if err != nil || len(data) == 0 {
		return nil
	}
	// The H3 mirror may serve pre-gzipped .pbf without the transfer header the
	// Go client would transparently decode — sniff the gzip magic and unwrap.
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil
		}
		data, err = io.ReadAll(io.LimitReader(zr, maxTileBytes))
		if err != nil {
			return nil
		}
	}
	layers, err := mvt.Unmarshal(data)
	if err != nil {
		s.logger.Debug("basemap search tile decode failed", slog.String("url", u), slog.String("error", err.Error()))
		return nil
	}
	layers.ProjectToWGS84(maptile.New(uint32(x), uint32(y), maptile.Zoom(z)))

	var out []Entry
	for _, layer := range layers {
		for _, f := range layer.Features {
			name := featureName(f.Properties)
			if name == "" {
				continue
			}
			c := f.Geometry.Bound().Center()
			out = append(out, Entry{
				Name: name, Category: layer.Name,
				Lon: c[0], Lat: c[1],
				norm: normalizeName(name),
			})
		}
	}
	return out
}

// featureName extracts a display name schema-tolerantly: the exact "name"
// property wins; otherwise the first string property whose key contains
// "name" (case-insensitive). Robust against BKG schema drift by design — we
// deliberately do NOT hard-code the tile schema (#277).
func featureName(props map[string]interface{}) string {
	if v, ok := props["name"].(string); ok && nameOK(v) {
		return strings.TrimSpace(v)
	}
	for k, raw := range props {
		if !strings.Contains(strings.ToLower(k), "name") {
			continue
		}
		if v, ok := raw.(string); ok && nameOK(v) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func nameOK(v string) bool {
	v = strings.TrimSpace(v)
	return len(v) >= 2 && len(v) <= 80
}

// addClustered merges e into the per-name clusters; entries of the same name
// closer than clusterKM collapse into the first-seen location. Reports whether
// a NEW entry was added.
func addClustered(clusters map[string][]Entry, e Entry) bool {
	pts := clusters[e.norm]
	for _, p := range pts {
		dLat := (e.Lat - p.Lat) * 111
		dLon := (e.Lon - p.Lon) * 111 * 0.64 // cos(50°) ≈ .64 — German latitudes
		if dLat*dLat+dLon*dLon < clusterKM*clusterKM {
			return false
		}
	}
	clusters[e.norm] = append(pts, e)
	return true
}

// tilesTemplate extracts the vector source's tile URL template from the
// style — absolute already (pkg/basemap rewrites it), so mirror deployments
// work unchanged. A MapLibre style declares tiles in one of TWO forms: inline
// (`tiles: [...]`) or as a TileJSON indirection (`url: ".../tilejson.json"`
// whose document carries the tiles array). The real basemap.de/basemap.world
// styles use the TileJSON form (operator finding 2026-07-19 — the inline-only
// reader failed every build with "style has no vector tile source"), so both
// forms are resolved here.
func (s *Service) tilesTemplate(ctx context.Context) (string, error) {
	raw, err := s.styles.StyleJSON(ctx)
	if err != nil {
		return "", fmt.Errorf("basemapsearch: style unavailable: %w", err)
	}
	var style struct {
		Sources map[string]struct {
			Type  string   `json:"type"`
			Tiles []string `json:"tiles"`
			URL   string   `json:"url"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(raw, &style); err != nil {
		return "", fmt.Errorf("basemapsearch: style parse: %w", err)
	}
	// Deterministic pick across map order: sort source names.
	names := make([]string, 0, len(style.Sources))
	for n := range style.Sources {
		names = append(names, n)
	}
	sort.Strings(names)
	var lastErr error
	for _, n := range names {
		src := style.Sources[n]
		if src.Type != "vector" {
			continue
		}
		if len(src.Tiles) > 0 {
			return src.Tiles[0], nil
		}
		if src.URL != "" {
			tmpl, err := s.tilesFromTileJSON(ctx, src.URL)
			if err == nil {
				return tmpl, nil
			}
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("basemapsearch: style has no vector tile source")
}

// tilesFromTileJSON follows a source's TileJSON reference and returns its tile
// URL template. Defensive consumer like every upstream read: context-bound,
// size-limited, status-checked.
func (s *Service) tilesFromTileJSON(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("basemapsearch: tilejson request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("basemapsearch: tilejson fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("basemapsearch: tilejson fetch: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTileJSONBytes))
	if err != nil {
		return "", fmt.Errorf("basemapsearch: tilejson read: %w", err)
	}
	var tj struct {
		Tiles []string `json:"tiles"`
	}
	if err := json.Unmarshal(data, &tj); err != nil {
		return "", fmt.Errorf("basemapsearch: tilejson parse: %w", err)
	}
	if len(tj.Tiles) == 0 || tj.Tiles[0] == "" {
		return "", fmt.Errorf("basemapsearch: tilejson has no tiles template")
	}
	return tj.Tiles[0], nil
}

// evictLocked drops the least-recently-used indexes beyond the cap. Caller
// holds s.mu.
func (s *Service) evictLocked() {
	for len(s.indexes) > s.cfg.MaxIndexes {
		var oldestKey string
		var oldest time.Time
		for k, ix := range s.indexes {
			if oldestKey == "" || ix.lastUsed.Before(oldest) {
				oldestKey, oldest = k, ix.lastUsed
			}
		}
		delete(s.indexes, oldestKey)
	}
}

func bboxKey(b BBox) string {
	return fmt.Sprintf("%.4f,%.4f,%.4f,%.4f", b.MinLat, b.MinLon, b.MaxLat, b.MaxLon)
}
