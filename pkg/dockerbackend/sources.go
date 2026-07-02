package dockerbackend

import (
	"encoding/json"
	"fmt"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// Translation of a feed's generic source list into Firefly's env-driven
// source-input contract (ORCH-5; Firefly ADR 0023, docs/source-input-contract.md).
// This is the Wayfinder side of the cross-project wire: a Firefly instance reads
// FIREFLY_SOURCES (a JSON array) plus, per credentialled source, a separate named
// credential env. The credential *value* is emitted only when the control plane
// has resolved it (ORCH-5b); it is never inlined into the JSON blob, which carries
// only the cred_env *name*.

// fireflySource is one entry of the FIREFLY_SOURCES JSON array. The field shapes
// mirror Firefly's contract: type, an optional bbox (store.BBox already carries
// the min_lat/min_lon/max_lat/max_lon JSON tags the contract expects), optional
// SAC/SIC for a radar, and cred_env — the *name* of the env carrying the secret.
type fireflySource struct {
	Type string      `json:"type"`
	BBox *store.BBox `json:"bbox,omitempty"`
	SAC  *int        `json:"sac,omitempty"`
	SIC  *int        `json:"sic,omitempty"`
	// Radar location (radar_asterix, contract v1.3.0 / #91): pass-through of the
	// stored site so Firefly can lift CAT048 polar plots into the tracking frame.
	Lat     *float64 `json:"lat,omitempty"`
	Lon     *float64 `json:"lon,omitempty"`
	HeightM *float64 `json:"height_m,omitempty"`
	Listen  string   `json:"listen,omitempty"`
	CredEnv string   `json:"cred_env,omitempty"`
}

// credEnvName is the deterministic env name carrying the resolved credential for
// the source at list position i (referenced from the JSON by cred_env).
func credEnvName(i int) string {
	return fmt.Sprintf("FIREFLY_SOURCE_%d_SECRET", i)
}

// fireflySourcesEnv renders a feed's source list into the FIREFLY_SOURCES JSON
// array and the matching credential value envs. It returns ok=false for an empty
// list (no sources to emit). resolved maps a cred_ref to its plaintext value
// (ORCH-5b); a source's cred_env is set — and a FIREFLY_SOURCE_<i>_SECRET=<value>
// env emitted — **only** when its cred_ref resolved to a non-empty value. A source
// whose credential is unresolved (no key / secret not set) is rendered without
// cred_env, so Firefly runs it anonymously rather than failing on a missing env.
//
// The output is deterministic (fixed field order, source order preserved, value
// envs in source order) so the spec hash stays stable across reconciles — and
// changes when a secret rotates, which is what triggers a restart with the new
// value.
func fireflySourcesEnv(sources store.SourceConfig, resolved map[string]string) (sourcesJSON string, credEnvs []string, ok bool) {
	if len(sources) == 0 {
		return "", nil, false
	}
	out := make([]fireflySource, 0, len(sources))
	for i, s := range sources {
		fs := fireflySource{
			Type:    string(s.Type),
			BBox:    s.BBox,
			SAC:     s.SAC,
			SIC:     s.SIC,
			Lat:     s.Lat,
			Lon:     s.Lon,
			HeightM: s.HeightM,
			Listen:  s.Listen,
		}
		if s.CredRef != nil {
			if v, found := resolved[*s.CredRef]; found && v != "" {
				name := credEnvName(i)
				fs.CredEnv = name
				credEnvs = append(credEnvs, name+"="+v)
			}
		}
		out = append(out, fs)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", nil, false
	}
	return string(b), credEnvs, true
}
