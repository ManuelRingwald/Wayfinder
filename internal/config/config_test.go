package config

import "testing"

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

// An empty environment yields the documented defaults. REQ: FR-CFG-001
func TestDefaults(t *testing.T) {
	cfg := FromLookup(lookupFrom(nil))

	if cfg.Port != DefaultPort {
		t.Errorf("expected default port %d, got %d", DefaultPort, cfg.Port)
	}
	if cfg.LogFormat != DefaultLogFormat {
		t.Errorf("expected default log format %q, got %q", DefaultLogFormat, cfg.LogFormat)
	}
}

// Valid environment variables override the defaults. REQ: FR-CFG-001
func TestValidOverrides(t *testing.T) {
	cfg := FromLookup(lookupFrom(map[string]string{
		"WAYFINDER_PORT":       "9090",
		"WAYFINDER_LOG_FORMAT": "json",
	}))

	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("expected log format json, got %q", cfg.LogFormat)
	}
}

// Invalid values fall back to defaults rather than crashing.
// REQ: FR-CFG-002
func TestInvalidValuesFallBackToDefaults(t *testing.T) {
	cfg := FromLookup(lookupFrom(map[string]string{
		"WAYFINDER_PORT":       "not-a-number",
		"WAYFINDER_LOG_FORMAT": "xml",
	}))

	if cfg.Port != DefaultPort {
		t.Errorf("expected default port %d for invalid input, got %d", DefaultPort, cfg.Port)
	}
	if cfg.LogFormat != DefaultLogFormat {
		t.Errorf("expected default log format %q for invalid input, got %q", DefaultLogFormat, cfg.LogFormat)
	}
}
