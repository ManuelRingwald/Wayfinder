package main

import (
	"log/slog"
	"testing"
	"time"
)

// envFunc builds a getenv stub from a map.
func envFunc(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(envFunc(map[string]string{"WAYFINDER_DB_URL": "postgres://x"}), nil)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.dsn != "postgres://x" {
		t.Errorf("dsn = %q", cfg.dsn)
	}
	if cfg.interval != defaultInterval {
		t.Errorf("interval = %v, want default %v", cfg.interval, defaultInterval)
	}
	if cfg.logLevel != slog.LevelInfo {
		t.Errorf("logLevel = %v, want info", cfg.logLevel)
	}
	if cfg.once {
		t.Error("once should default to false")
	}
}

func TestLoadConfigRequiresDSN(t *testing.T) {
	if _, err := loadConfig(envFunc(nil), nil); err == nil {
		t.Fatal("loadConfig should fail without WAYFINDER_DB_URL")
	}
}

func TestLoadConfigOnceFlag(t *testing.T) {
	cfg, err := loadConfig(envFunc(map[string]string{"WAYFINDER_DB_URL": "x"}), []string{"--once"})
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if !cfg.once {
		t.Error("--once should set once=true")
	}
}

func TestLoadConfigCustomInterval(t *testing.T) {
	cfg, err := loadConfig(envFunc(map[string]string{
		"WAYFINDER_DB_URL":                "x",
		"WAYFINDER_ORCHESTRATOR_INTERVAL": "5s",
	}), nil)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.interval != 5*time.Second {
		t.Errorf("interval = %v, want 5s", cfg.interval)
	}
}

func TestLoadConfigInvalidIntervalFallsBack(t *testing.T) {
	for _, v := range []string{"nonsense", "0s", "-3s"} {
		cfg, err := loadConfig(envFunc(map[string]string{
			"WAYFINDER_DB_URL":                "x",
			"WAYFINDER_ORCHESTRATOR_INTERVAL": v,
		}), nil)
		if err != nil {
			t.Fatalf("loadConfig(%q): %v", v, err)
		}
		if cfg.interval != defaultInterval {
			t.Errorf("interval for %q = %v, want default", v, cfg.interval)
		}
	}
}

func TestLoadConfigLogLevel(t *testing.T) {
	cfg, _ := loadConfig(envFunc(map[string]string{
		"WAYFINDER_DB_URL":    "x",
		"WAYFINDER_LOG_LEVEL": "debug",
	}), nil)
	if cfg.logLevel != slog.LevelDebug {
		t.Errorf("logLevel = %v, want debug", cfg.logLevel)
	}
	// Invalid level falls back to info.
	cfg2, _ := loadConfig(envFunc(map[string]string{
		"WAYFINDER_DB_URL":    "x",
		"WAYFINDER_LOG_LEVEL": "bogus",
	}), nil)
	if cfg2.logLevel != slog.LevelInfo {
		t.Errorf("invalid level → %v, want info fallback", cfg2.logLevel)
	}
}

func TestLoadConfigUnknownFlagErrors(t *testing.T) {
	if _, err := loadConfig(envFunc(map[string]string{"WAYFINDER_DB_URL": "x"}), []string{"--nope"}); err == nil {
		t.Fatal("an unknown flag should be an error")
	}
}
