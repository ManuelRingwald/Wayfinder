package dockerbackend

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

func ptrInt(v int) *int           { return &v }
func ptrStr(v string) *string     { return &v }
func ptrFloat(v float64) *float64 { return &v }

// An empty source list renders nothing (ok=false) so the caller can fall back to
// the placeholder scene.
func TestFireflySourcesEnvEmpty(t *testing.T) {
	if _, _, ok := fireflySourcesEnv(nil, nil); ok {
		t.Fatal("empty sources should render ok=false")
	}
	if _, _, ok := fireflySourcesEnv(store.SourceConfig{}, nil); ok {
		t.Fatal("empty sources should render ok=false")
	}
}

// An adsb_opensky source with a bbox and a *resolved* credential renders the
// contract shape (Firefly ADR 0023): type, bbox with min_lat… field names, a
// cred_env *name*, and a matching FIREFLY_SOURCE_0_SECRET=<value> env. The value
// never appears in the JSON blob.
func TestFireflySourcesEnvAdsbWithResolvedCredential(t *testing.T) {
	sources := store.SourceConfig{
		{
			Type:    store.SourceADSBOpenSky,
			BBox:    &store.BBox{MinLat: 48, MinLon: 7, MaxLat: 50, MaxLon: 9},
			CredRef: ptrStr("secret/opensky"),
		},
	}
	resolved := map[string]string{"secret/opensky": "alice:s3cr3t"}
	js, credEnvs, ok := fireflySourcesEnv(sources, resolved)
	if !ok {
		t.Fatal("expected ok=true")
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(js), &got); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, js)
	}
	if got[0]["type"] != "adsb_opensky" || got[0]["cred_env"] != "FIREFLY_SOURCE_0_SECRET" {
		t.Errorf("entry = %v", got[0])
	}
	// The credential value must never appear in the JSON.
	if strings.Contains(js, "s3cr3t") {
		t.Errorf("JSON leaked the credential value: %s", js)
	}
	// The value is emitted as a separate env.
	if len(credEnvs) != 1 || credEnvs[0] != "FIREFLY_SOURCE_0_SECRET=alice:s3cr3t" {
		t.Errorf("credEnvs = %v, want [FIREFLY_SOURCE_0_SECRET=alice:s3cr3t]", credEnvs)
	}
}

// A credentialled source whose secret is *unresolved* (no key / not set) is
// rendered without cred_env, so Firefly runs it anonymously instead of failing on
// a missing env. No value env is emitted.
func TestFireflySourcesEnvUnresolvedCredentialIsAnonymous(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}, CredRef: ptrStr("secret/missing")},
	}
	js, credEnvs, _ := fireflySourcesEnv(sources, nil) // nothing resolved
	var got []map[string]any
	_ = json.Unmarshal([]byte(js), &got)
	if _, has := got[0]["cred_env"]; has {
		t.Error("unresolved credential must omit cred_env (anonymous fallback)")
	}
	if len(credEnvs) != 0 {
		t.Errorf("no value env expected, got %v", credEnvs)
	}
}

// A radar source carries sac/sic + location (lat/lon/listen, contract v1.3.0 /
// #91) and no bbox/cred_env; an anonymous source omits cred_env entirely.
func TestFireflySourcesEnvRadarAndAnonymous(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}},
		{Type: store.SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4), Lat: ptrFloat(50.03), Lon: ptrFloat(8.57), Listen: "239.255.0.48:8048"},
	}
	js, credEnvs, _ := fireflySourcesEnv(sources, nil)
	var got []map[string]any
	if err := json.Unmarshal([]byte(js), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, has := got[0]["cred_env"]; has {
		t.Error("anonymous source must omit cred_env")
	}
	if got[1]["type"] != "radar_asterix" || got[1]["sac"] != 1.0 || got[1]["sic"] != 4.0 {
		t.Errorf("radar entry = %v", got[1])
	}
	// #91: radar site is serialized so Firefly can geolocate CAT048 polar plots.
	if got[1]["lat"] != 50.03 || got[1]["lon"] != 8.57 || got[1]["listen"] != "239.255.0.48:8048" {
		t.Errorf("radar location = lat:%v lon:%v listen:%v", got[1]["lat"], got[1]["lon"], got[1]["listen"])
	}
	if _, has := got[1]["bbox"]; has {
		t.Error("radar source must omit bbox")
	}
	// An area source (no location) omits the radar fields entirely.
	if _, has := got[0]["lat"]; has {
		t.Error("area source must omit lat")
	}
	if len(credEnvs) != 0 {
		t.Errorf("no creds expected, got %v", credEnvs)
	}
}

// #239/#240: an ASTERIX-over-UDP source (adsb_asterix / mlat_asterix) serializes
// its listen endpoint + optional sac/sic + optional sensor_id, and no
// bbox/location/cred_env — it is auth-free and the ground system computes the
// position. An all-default source puts only its type on the wire.
func TestFireflySourcesEnvAsterixUDP(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBASTERIX, SAC: ptrInt(0), SIC: ptrInt(1), SensorID: ptrInt(230), Listen: "239.255.0.21:8021"},
		{Type: store.SourceMLATASTERIX},
	}
	js, credEnvs, ok := fireflySourcesEnv(sources, nil)
	if !ok {
		t.Fatal("expected ok for a non-empty config")
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(js), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got[0]["type"] != "adsb_asterix" || got[0]["listen"] != "239.255.0.21:8021" || got[0]["sensor_id"] != 230.0 {
		t.Errorf("adsb_asterix entry = %v", got[0])
	}
	if got[0]["sac"] != 0.0 || got[0]["sic"] != 1.0 {
		t.Errorf("adsb_asterix sac/sic = %v/%v", got[0]["sac"], got[0]["sic"])
	}
	for _, k := range []string{"bbox", "lat", "lon", "cred_env"} {
		if _, has := got[0][k]; has {
			t.Errorf("adsb_asterix must omit %q", k)
		}
	}
	if got[1]["type"] != "mlat_asterix" {
		t.Errorf("mlat entry type = %v", got[1]["type"])
	}
	for _, k := range []string{"listen", "sensor_id", "sac", "sic"} {
		if _, has := got[1][k]; has {
			t.Errorf("all-default mlat_asterix must omit %q", k)
		}
	}
	if len(credEnvs) != 0 {
		t.Errorf("no creds expected, got %v", credEnvs)
	}
}

// poll_interval_secs is passed through into the FIREFLY_SOURCES JSON for an
// adsb_opensky source (contract v1.4.0 / ADR 0029); a source without it omits the
// field so Firefly keeps its default.
func TestFireflySourcesEnvPollIntervalPassthrough(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}, PollIntervalSecs: ptrInt(30)},
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}},
	}
	js, _, _ := fireflySourcesEnv(sources, nil)
	var got []map[string]any
	if err := json.Unmarshal([]byte(js), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got[0]["poll_interval_secs"] != 30.0 {
		t.Errorf("poll_interval_secs = %v, want 30", got[0]["poll_interval_secs"])
	}
	if _, has := got[1]["poll_interval_secs"]; has {
		t.Error("source without poll interval must omit the field (Firefly default)")
	}
}

// provider is passed through into the FIREFLY_SOURCES JSON for an
// adsb_aggregator source (contract v1.5.0 / ADR 0031); a source without it
// omits the field so Firefly keeps its default (adsb_lol). The type is
// auth-free, so no cred_env is emitted even when a resolved map is present.
func TestFireflySourcesEnvAggregatorProviderPassthrough(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBAggregator, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}, Provider: "adsb_fi", PollIntervalSecs: ptrInt(15)},
		{Type: store.SourceADSBAggregator, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}},
	}
	js, credEnvs, ok := fireflySourcesEnv(sources, map[string]string{"secret/unrelated": "v"})
	if !ok {
		t.Fatal("expected ok")
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(js), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got[0]["type"] != "adsb_aggregator" || got[0]["provider"] != "adsb_fi" {
		t.Errorf("first source = %v, want adsb_aggregator with provider adsb_fi", got[0])
	}
	if got[0]["poll_interval_secs"] != 15.0 {
		t.Errorf("poll_interval_secs = %v, want 15", got[0]["poll_interval_secs"])
	}
	if _, has := got[1]["provider"]; has {
		t.Error("source without provider must omit the field (Firefly default adsb_lol)")
	}
	if _, has := got[0]["cred_env"]; has {
		t.Error("aggregator source must not carry a cred_env (auth-free)")
	}
	if len(credEnvs) != 0 {
		t.Errorf("credEnvs = %v, want none", credEnvs)
	}
}

// cred_env names and value envs are assigned by list position.
func TestFireflySourcesEnvCredByIndex(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}},
		{Type: store.SourceFLARMAPRS, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}, CredRef: ptrStr("secret/flarm")},
	}
	js, credEnvs, _ := fireflySourcesEnv(sources, map[string]string{"secret/flarm": "u:p"})
	var got []map[string]any
	_ = json.Unmarshal([]byte(js), &got)
	if got[1]["cred_env"] != "FIREFLY_SOURCE_1_SECRET" {
		t.Errorf("cred_env = %v, want FIREFLY_SOURCE_1_SECRET", got[1]["cred_env"])
	}
	if len(credEnvs) != 1 || credEnvs[0] != "FIREFLY_SOURCE_1_SECRET=u:p" {
		t.Errorf("credEnvs = %v", credEnvs)
	}
}

// Rendering is deterministic so the spec hash is stable across reconciles.
func TestFireflySourcesEnvDeterministic(t *testing.T) {
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: &store.BBox{MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}, CredRef: ptrStr("secret/x")},
	}
	resolved := map[string]string{"secret/x": "u:p"}
	jsA, envA, _ := fireflySourcesEnv(sources, resolved)
	jsB, envB, _ := fireflySourcesEnv(sources, resolved)
	if jsA != jsB || strings.Join(envA, ",") != strings.Join(envB, ",") {
		t.Fatalf("non-deterministic render")
	}
}
