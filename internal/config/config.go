// Package config reads Wayfinder's runtime configuration from environment
// variables (12-factor app style, see CLAUDE.md section 8).
package config

import (
	"os"
	"strconv"
)

// Config holds Wayfinder's runtime configuration.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// LogFormat selects the structured log encoding: "text" or "json".
	LogFormat string
}

// Defaults used when the corresponding environment variable is unset or
// cannot be parsed.
const (
	DefaultPort      = 8080
	DefaultLogFormat = "text"
)

// Load reads the configuration from the process environment.
func Load() Config {
	return FromLookup(os.LookupEnv)
}

// FromLookup builds a [Config] using the given lookup function, so tests can
// supply environment values without touching the real process environment.
func FromLookup(lookup func(string) (string, bool)) Config {
	cfg := Config{
		Port:      DefaultPort,
		LogFormat: DefaultLogFormat,
	}

	if v, ok := lookup("WAYFINDER_PORT"); ok {
		if port, err := strconv.Atoi(v); err == nil && port > 0 && port <= 65535 {
			cfg.Port = port
		}
	}

	if v, ok := lookup("WAYFINDER_LOG_FORMAT"); ok {
		if v == "json" || v == "text" {
			cfg.LogFormat = v
		}
	}

	return cfg
}
