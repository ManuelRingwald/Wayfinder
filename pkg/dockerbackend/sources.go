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
// credential env. Here we emit only the *structure* and the credential env
// *names* a source references — the resolved credential *values* are injected by
// the control plane (ORCH-5b), never inlined into the JSON blob.

// fireflySource is one entry of the FIREFLY_SOURCES JSON array. The field shapes
// mirror Firefly's contract: type, an optional bbox (store.BBox already carries
// the min_lat/min_lon/max_lat/max_lon JSON tags the contract expects), optional
// SAC/SIC for a radar, and cred_env — the *name* of the env carrying the secret.
type fireflySource struct {
	Type    string      `json:"type"`
	BBox    *store.BBox `json:"bbox,omitempty"`
	SAC     *int        `json:"sac,omitempty"`
	SIC     *int        `json:"sic,omitempty"`
	CredEnv string      `json:"cred_env,omitempty"`
}

// credEnvName is the deterministic env name carrying the resolved credential for
// the source at list position i (referenced from the JSON by cred_env). The
// control plane sets this env to the resolved "user:pass" value (ORCH-5b).
func credEnvName(i int) string {
	return fmt.Sprintf("FIREFLY_SOURCE_%d_SECRET", i)
}

// fireflySourcesJSON renders a feed's source list as the FIREFLY_SOURCES JSON
// array. It returns ok=false for an empty list (no sources to emit). The output
// is deterministic (fixed struct field order, source order preserved) so the
// spec hash stays stable across reconciles. Credential *values* are not included;
// a source with a cred_ref gets a cred_env *name* the control plane fills.
func fireflySourcesJSON(sources store.SourceConfig) (string, bool) {
	if len(sources) == 0 {
		return "", false
	}
	out := make([]fireflySource, 0, len(sources))
	for i, s := range sources {
		fs := fireflySource{
			Type: string(s.Type),
			BBox: s.BBox,
			SAC:  s.SAC,
			SIC:  s.SIC,
		}
		if s.CredRef != nil {
			fs.CredEnv = credEnvName(i)
		}
		out = append(out, fs)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", false
	}
	return string(b), true
}
