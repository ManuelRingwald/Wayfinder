package basemapsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
)

// ---- unit pieces ------------------------------------------------------------

func TestTileXYKnownPoint(t *testing.T) {
	// Frankfurt (50.0379N, 8.5622E) at z14 — reference 8581/5553, verified
	// against an independent implementation of the standard slippy-map formula.
	x, y := tileXY(50.0379, 8.5622, 14)
	if x != 8581 || y != 5553 {
		t.Errorf("tileXY(Frankfurt, z14) = %d/%d, want 8581/5553", x, y)
	}
}

func TestTilesForBBoxClampsAroundCentre(t *testing.T) {
	// A huge box (all of Germany-ish) at z14 vastly exceeds a 64-tile cap.
	b := BBox{MinLat: 47.3, MinLon: 5.9, MaxLat: 55.1, MaxLon: 15.0}
	r := tilesForBBox(b, 14, 64)
	if !r.clamped {
		t.Fatalf("expected clamping")
	}
	if r.count() > 64 {
		t.Errorf("count %d exceeds cap", r.count())
	}
	if r.requestedTileCount <= 64 {
		t.Errorf("requestedTileCount %d should reflect the pre-clamp size", r.requestedTileCount)
	}
	// The clamped window must stay inside the original range (centre-preserving).
	full := tilesForBBox(b, 14, 1<<30)
	if r.minX < full.minX || r.maxX > full.maxX || r.minY < full.minY || r.maxY > full.maxY {
		t.Errorf("clamped range %+v escapes original %+v", r, full)
	}
}

func TestNormalizeName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Friedrichstraße", "friedrichstr"},
		{"Friedrichstrasse", "friedrichstr"},
		{"Friedrichstr.", "friedrichstr"},
		{"  Große   Bäckergasse ", "grosse baeckergasse"},
		{"Überseering", "ueberseering"},
	} {
		if got := normalizeName(tc.in); got != tc.want {
			t.Errorf("normalizeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAddClusteredMergesNearbySameName(t *testing.T) {
	clusters := map[string][]Entry{}
	a := Entry{Name: "Friedrichstraße", Lat: 50.00, Lon: 8.50, norm: "friedrichstr"}
	near := Entry{Name: "Friedrichstraße", Lat: 50.01, Lon: 8.51, norm: "friedrichstr"} // ~1.3 km
	far := Entry{Name: "Friedrichstraße", Lat: 50.30, Lon: 8.50, norm: "friedrichstr"}  // ~33 km
	if !addClustered(clusters, a) {
		t.Fatal("first entry must add")
	}
	if addClustered(clusters, near) {
		t.Error("nearby duplicate must merge")
	}
	if !addClustered(clusters, far) {
		t.Error("distant same-name street must stay separate")
	}
	if len(clusters["friedrichstr"]) != 2 {
		t.Errorf("clusters = %d, want 2", len(clusters["friedrichstr"]))
	}
}

func TestMatchRanking(t *testing.T) {
	entries := []Entry{
		{Name: "Am Friedrichshof", norm: "am friedrichshof"},
		{Name: "Friedrichstraße", norm: "friedrichstr"},
		{Name: "Friedrichsberg", norm: "friedrichsberg"},
	}
	hits := match(entries, "Friedrich")
	if len(hits) != 3 {
		t.Fatalf("hits = %d, want 3", len(hits))
	}
	// Prefix matches first (shorter first), the infix match last.
	if hits[0].Name != "Friedrichstraße" || hits[2].Name != "Am Friedrichshof" {
		t.Errorf("ranking wrong: %v", []string{hits[0].Name, hits[1].Name, hits[2].Name})
	}
	if got := match(entries, "F"); len(got) != 0 {
		t.Errorf("single-char query must return empty, got %d", len(got))
	}
}

// ---- end-to-end against an MVT-serving upstream -----------------------------

type stubStyles struct{ style []byte }

func (s stubStyles) StyleJSON(context.Context) ([]byte, error) { return s.style, nil }

// smallBBox is a tiny AOI (a few z14 tiles) around a reference point.
var smallBBox = BBox{MinLat: 50.03, MinLon: 8.55, MaxLat: 50.05, MaxLon: 8.58}

// newTileUpstream serves MVT tiles that contain a street (LineString, exact
// "name" key), a place with a schema-drifted name key ("objektname") and an
// unnamed feature — encoded with orb, the same wire format basemap.de serves.
func newTileUpstream(t *testing.T, gzipped bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/tiles/"), ".pbf"), "/")
		if len(parts) != 3 {
			http.NotFound(w, r)
			return
		}
		z, _ := strconv.Atoi(parts[0])
		x, _ := strconv.Atoi(parts[1])
		y, _ := strconv.Atoi(parts[2])

		street := geojson.NewFeature(orb.LineString{{8.560, 50.040}, {8.565, 50.041}})
		street.Properties = geojson.Properties{"name": "Friedrichstraße", "klasse": "Gemeindestraße"}
		place := geojson.NewFeature(orb.Point{8.562, 50.042})
		place.Properties = geojson.Properties{"objektname": "Rathausplatz"}
		unnamed := geojson.NewFeature(orb.Point{8.561, 50.043})
		unnamed.Properties = geojson.Properties{"klasse": "Weg"}

		fcStreets := geojson.NewFeatureCollection()
		fcStreets.Append(street)
		fcStreets.Append(unnamed)
		fcPlaces := geojson.NewFeatureCollection()
		fcPlaces.Append(place)

		layers := mvt.NewLayers(map[string]*geojson.FeatureCollection{
			"verkehrslinie": fcStreets,
			"siedlung":      fcPlaces,
		})
		layers.ProjectToTile(maptile.New(uint32(x), uint32(y), maptile.Zoom(z)))
		var data []byte
		var err error
		if gzipped {
			data, err = mvt.MarshalGzipped(layers)
		} else {
			data, err = mvt.Marshal(layers)
		}
		if err != nil {
			t.Errorf("marshal tile: %v", err)
			http.Error(w, "encode", 500)
			return
		}
		_, _ = w.Write(data)
	}))
}

func styleFor(srv *httptest.Server) []byte {
	return []byte(fmt.Sprintf(`{"version":8,"sources":{"basemap":{"type":"vector","tiles":["%s/tiles/{z}/{x}/{y}.pbf"]}}}`, srv.URL))
}

// waitReady polls Search until the async build finishes.
func waitReady(t *testing.T, svc *Service, bbox BBox, q string) Result {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res := svc.Search(bbox, q)
		if res.Status == "ready" {
			return res
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("index build did not become ready")
	return Result{}
}

func TestBuildAndSearchEndToEnd(t *testing.T) {
	for _, gz := range []bool{false, true} {
		t.Run(fmt.Sprintf("gzipped=%v", gz), func(t *testing.T) {
			srv := newTileUpstream(t, gz)
			defer srv.Close()
			svc := NewService(stubStyles{styleFor(srv)}, srv.Client(), Config{MaxTiles: 32}, nil)

			if res := svc.Search(smallBBox, "friedrich"); res.Status != "building" {
				t.Fatalf("first search: status %q, want building", res.Status)
			}
			res := waitReady(t, svc, smallBBox, "friedrichstr")
			if len(res.Results) == 0 {
				t.Fatalf("no hits for friedrichstr")
			}
			hit := res.Results[0]
			if hit.Name != "Friedrichstraße" || hit.Category != "verkehrslinie" {
				t.Errorf("hit = %+v", hit)
			}
			if hit.Lat < 50.0 || hit.Lat > 50.1 || hit.Lon < 8.5 || hit.Lon > 8.6 {
				t.Errorf("hit coordinates off: %+v", hit)
			}

			// Schema tolerance: the "objektname" key counts as a name.
			res = svc.Search(smallBBox, "rathaus")
			if len(res.Results) == 0 || res.Results[0].Name != "Rathausplatz" {
				t.Errorf("schema-tolerant name extraction failed: %+v", res.Results)
			}

			// Unnamed features stay out; a nonsense query yields empty-but-ready.
			res = svc.Search(smallBBox, "zzzzzz")
			if res.Status != "ready" || len(res.Results) != 0 {
				t.Errorf("nonsense query: %+v", res)
			}
		})
	}
}

// TestBuildResolvesTileJSONSource pins the fix for the operator-found first-run
// failure (2026-07-19): the real basemap.de/basemap.world styles declare their
// vector source via a TileJSON reference (`url: ".../tilejson.json"`), not an
// inline `tiles` array — the inline-only reader failed every build with
// "style has no vector tile source".
func TestBuildResolvesTileJSONSource(t *testing.T) {
	tiles := newTileUpstream(t, false)
	defer tiles.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/tilejson.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"tilejson":"3.0.0","tiles":["%s/tiles/{z}/{x}/{y}.pbf"]}`, tiles.URL)
	})
	meta := httptest.NewServer(mux)
	defer meta.Close()

	style := []byte(fmt.Sprintf(`{"version":8,"sources":{"basemap":{"type":"vector","url":"%s/tilejson.json"}}}`, meta.URL))
	svc := NewService(stubStyles{style}, tiles.Client(), Config{MaxTiles: 32}, nil)

	if res := svc.Search(smallBBox, "friedrich"); res.Status != "building" {
		t.Fatalf("first search: status %q, want building", res.Status)
	}
	res := waitReady(t, svc, smallBBox, "friedrichstr")
	if len(res.Results) == 0 || res.Results[0].Name != "Friedrichstraße" {
		t.Fatalf("TileJSON-resolved build found no hits: %+v", res.Results)
	}
}

// TestSearchReportsErrorStatus: a failing FIRST build must surface as "error"
// (not a perpetual "building" spinner) and stay stable across the background
// retries the service keeps launching.
func TestSearchReportsErrorStatus(t *testing.T) {
	// A style whose vector source resolves nowhere → every build fails fast.
	svc := NewService(stubStyles{[]byte(`{"version":8,"sources":{}}`)}, http.DefaultClient, Config{MaxTiles: 4}, nil)

	if res := svc.Search(smallBBox, "x"); res.Status != "building" {
		t.Fatalf("first search: status %q, want building", res.Status)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		res := svc.Search(smallBBox, "x")
		if res.Status == "error" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("failed build never surfaced as error (last status %q)", res.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The error must be sticky: repeated polls (which kick retries) keep
	// reporting error, never flap back to a bare "building".
	for i := 0; i < 5; i++ {
		if res := svc.Search(smallBBox, "x"); res.Status == "ready" {
			t.Fatalf("impossible ready from a failing build")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if svc.BuildFailureCount() == 0 {
		t.Error("failure metric not incremented")
	}
}

func TestDistanceAndBearing(t *testing.T) {
	// Due north: one degree of latitude ≈ 60 NM, bearing 0°.
	if d := distanceNM(50, 8, 51, 8); d < 59 || d > 61 {
		t.Errorf("distanceNM north = %.1f, want ~60", d)
	}
	if b := bearingDeg(50, 8, 51, 8); b != 0 {
		t.Errorf("bearing due north = %d, want 0", b)
	}
	// Due east at 50°N: bearing ≈ 90°, distance ≈ 60·cos(50°) ≈ 38.6 NM.
	if b := bearingDeg(50, 8, 50, 9); b < 88 || b > 92 {
		t.Errorf("bearing due east = %d, want ~90", b)
	}
	if d := distanceNM(50, 8, 50, 9); d < 37 || d > 40 {
		t.Errorf("distanceNM east = %.1f, want ~38.6", d)
	}
}

func TestIsPlaceCategory(t *testing.T) {
	for cat, want := range map[string]bool{
		"siedlung": true, "siedlungsflaeche": true, "ortslage": true,
		"ort": true, "gemeinde_label": true, "wohnplatz": true,
		"verkehrslinie": false, "gewaesserlinie": false,
		"vegetationsflaeche": false, "": false,
	} {
		if got := isPlaceCategory(cat); got != want {
			t.Errorf("isPlaceCategory(%q) = %v, want %v", cat, got, want)
		}
	}
}

func TestEnrichHitsAddsRadialAndNearestPlace(t *testing.T) {
	bbox := BBox{MinLat: 50.0, MinLon: 8.0, MaxLat: 50.2, MaxLon: 8.2} // centre 50.1/8.1
	places := []Entry{{Name: "Wegberg", norm: "wegberg", Lat: 50.155, Lon: 8.152}}
	hits := []Entry{{Name: "Forststraße", norm: "forststr", Lat: 50.15, Lon: 8.15}}
	enrichHits(hits, bbox, places)
	if hits[0].Near != "Wegberg" {
		t.Errorf("Near = %q, want Wegberg", hits[0].Near)
	}
	if hits[0].DistNM <= 0 {
		t.Errorf("DistNM = %v, want > 0", hits[0].DistNM)
	}
	if hits[0].Bearing < 0 || hits[0].Bearing >= 360 {
		t.Errorf("Bearing = %d out of range", hits[0].Bearing)
	}
	// A place beyond maxNearKM must not attach — the row then shows the radial
	// alone (graceful degradation the operator chose).
	far := []Entry{{Name: "Forststraße", norm: "forststr", Lat: 50.15, Lon: 8.15}}
	enrichHits(far, bbox, []Entry{{Name: "Fernort", norm: "fernort", Lat: 51.0, Lon: 8.15}})
	if far[0].Near != "" {
		t.Errorf("far place should not attach: %q", far[0].Near)
	}
}

// TestRadialAlwaysMarshalled guards the fix for the omitempty pitfall: a valid
// due-north bearing of 0° (and a 0 NM distance) must still appear on the wire,
// or the frontend would drop the whole radial for the northern arc.
func TestRadialAlwaysMarshalled(t *testing.T) {
	// A hit exactly due north of the sector centre → bearing rounds to 0.
	bbox := BBox{MinLat: 50.0, MinLon: 8.0, MaxLat: 50.4, MaxLon: 8.0} // centre 50.2/8.0
	hits := []Entry{{Name: "Nordstraße", norm: "nordstr", Lat: 50.3, Lon: 8.0}}
	enrichHits(hits, bbox, nil)
	if hits[0].Bearing != 0 {
		t.Fatalf("bearing due north = %d, want 0", hits[0].Bearing)
	}
	raw, err := json.Marshal(hits[0])
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"bearing_deg":0`) {
		t.Errorf("bearing_deg dropped from JSON for due-north hit: %s", s)
	}
	if !strings.Contains(s, `"dist_nm":`) {
		t.Errorf("dist_nm missing from JSON: %s", s)
	}
	// near is optional and absent here → must be omitted.
	if strings.Contains(s, `"near"`) {
		t.Errorf("near should be omitted when empty: %s", s)
	}
}

func TestHandlerGatesAndStatuses(t *testing.T) {
	srv := newTileUpstream(t, false)
	defer srv.Close()
	svc := NewService(stubStyles{styleFor(srv)}, srv.Client(), Config{MaxTiles: 32}, nil)

	denied := svc.Handler(func(*http.Request) bool { return false }, func(*http.Request) (BBox, bool) { return smallBBox, true })
	rec := httptest.NewRecorder()
	denied(rec, httptest.NewRequest(http.MethodGet, "/api/basemap/search?q=x", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("unentitled: %d, want 403", rec.Code)
	}

	noArea := svc.Handler(func(*http.Request) bool { return true }, func(*http.Request) (BBox, bool) { return BBox{}, false })
	rec = httptest.NewRecorder()
	noArea(rec, httptest.NewRequest(http.MethodGet, "/api/basemap/search?q=x", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("no area: %d, want 503", rec.Code)
	}

	h := svc.Handler(func(*http.Request) bool { return true }, func(*http.Request) (BBox, bool) { return smallBBox, true })
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/basemap/search?q=friedrich", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("first call: %d, want 202 (building)", rec.Code)
	}
	waitReady(t, svc, smallBBox, "friedrich")
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/basemap/search?q=friedrich", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ready call: %d, want 200", rec.Code)
	}
	var res Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != "ready" || len(res.Results) == 0 {
		t.Errorf("ready payload: %+v", res)
	}
}

func TestLRUEviction(t *testing.T) {
	srv := newTileUpstream(t, false)
	defer srv.Close()
	svc := NewService(stubStyles{styleFor(srv)}, srv.Client(), Config{MaxTiles: 4, MaxIndexes: 2}, nil)

	for i := 0; i < 4; i++ {
		b := smallBBox
		b.MinLat += float64(i) * 0.001 // distinct AOIs → distinct indexes
		svc.Search(b, "x")
	}
	svc.mu.Lock()
	n := len(svc.indexes)
	svc.mu.Unlock()
	if n > 2 {
		t.Errorf("indexes = %d, want ≤ MaxIndexes 2", n)
	}
}
