package store

import (
	"encoding/json"
	"testing"
)

func TestNormalizeSettings(t *testing.T) {
	if got := string(normalizeSettings(nil)); got != "{}" {
		t.Errorf("nil -> %q, want {}", got)
	}
	if got := string(normalizeSettings(json.RawMessage{})); got != "{}" {
		t.Errorf("empty -> %q, want {}", got)
	}
	in := json.RawMessage(`{"rangeRings":true}`)
	if got := string(normalizeSettings(in)); got != `{"rangeRings":true}` {
		t.Errorf("passthrough -> %q, want it unchanged", got)
	}
}
