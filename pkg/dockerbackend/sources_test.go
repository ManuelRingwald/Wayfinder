package dockerbackend

import (
	"encoding/json"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

func ptrInt(v int) *int       { return &v }
func ptrStr(v string) *string { return &v }

// An empty source list renders nothing (ok=false) so the caller can fall back to
// the placeholder scene.
func TestFireflySourcesJSONEmpty(t *testing.T) {
	if _, ok := fireflySourcesJSON(nil); ok {
		t.Fatal("empty sources should render ok=false")
	}
	if _, ok := fireflySourcesJSON(store.SourceConfig{}); ok {
		t.Fatal("empty sources should render ok=false")
	}
}

// An adsb_opensky source with a bbox and a credential renders the contract shape
// (Firefly ADR 0023): type, bbox with min_lat… field names, and a cred_env *name*
// (never the value).
func TestFireflySourcesJSONAdsbWithCredential(t *testing.T) {
	sources := store.SourceConfig{
		{
			Type:    store.SourceADSBOpenSky,
			BBox:    &store.BBox{MinLat: 48, MinLon: 7, MaxLat: 50, MaxLon: 9},
			CredRef: ptrStr("secret/opensky"),
		},
	}
	js, ok := fireflySourcesJSON(sources)
	if !ok {
		t.Fatal("expected ok=true")
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(js), &got); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, js)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	e := got[0]
	if e["type"] != "adsb_opensky" {
		t.Errorf("type = %v, want adsb_opensky", e["type"])
	}
	bbox, _ := e["bbox"].(map[string]any)
	if bbox["min_lat"] != 48.0 || bbox["max_lon"] != 9.0 {
		t.Errorf("bbox = %v, want min_lat 48 / max_lon 9", bbox)
	}
	if e["cred_env"] != "FIREFLY_SOURCE_0_SECRET" {
		t.Errorf("cred_env = %v, want FIREFLY_SOURCE_0_SECRET", e["cred_env"])
	}
	// The credential value must never appear in the rendered JSON.
	if _, hasRef := e["cred_ref"]; hasRef {
		t.Error("rendered JSON must not carry cred_ref")
	}
}

// A radar source carries sac/sic and no bbox/cred_env; a source without a
// credential omits cred_env entirely.
func TestFireflySourcesJSONRadarAndAnonymous(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}},
		{Type: store.SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)},
	}
	js, _ := fireflySourcesJSON(sources)
	var got []map[string]any
	if err := json.Unmarshal([]byte(js), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	// Anonymous adsb: no cred_env.
	if _, has := got[0]["cred_env"]; has {
		t.Error("anonymous source must omit cred_env")
	}
	// Radar: sac/sic present, bbox absent.
	if got[1]["type"] != "radar_asterix" || got[1]["sac"] != 1.0 || got[1]["sic"] != 4.0 {
		t.Errorf("radar entry = %v", got[1])
	}
	if _, has := got[1]["bbox"]; has {
		t.Error("radar source must omit bbox")
	}
}

// cred_env names are assigned by list position, so a credential on the second
// source maps to FIREFLY_SOURCE_1_SECRET.
func TestFireflySourcesJSONCredEnvByIndex(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}},
		{Type: store.SourceFLARMAPRS, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}, CredRef: ptrStr("secret/flarm")},
	}
	js, _ := fireflySourcesJSON(sources)
	var got []map[string]any
	_ = json.Unmarshal([]byte(js), &got)
	if got[1]["cred_env"] != "FIREFLY_SOURCE_1_SECRET" {
		t.Errorf("cred_env = %v, want FIREFLY_SOURCE_1_SECRET", got[1]["cred_env"])
	}
}

// Rendering is deterministic so the spec hash is stable across reconciles.
func TestFireflySourcesJSONDeterministic(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}, CredRef: ptrStr("secret/x")},
	}
	a, _ := fireflySourcesJSON(sources)
	b, _ := fireflySourcesJSON(sources)
	if a != b {
		t.Fatalf("non-deterministic render:\n%s\n%s", a, b)
	}
}
