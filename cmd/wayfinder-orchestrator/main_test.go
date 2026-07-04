package main

import (
	"crypto/rand"
	"encoding/base64"
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

// A valid base64 32-byte WAYFINDER_SECRET_KEY is decoded; an unset or malformed
// key yields nil (credentialled sources then run anonymously) without failing the
// load.
func TestLoadConfigSecretKey(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	valid := base64.StdEncoding.EncodeToString(key)

	cfg, err := loadConfig(envFunc(map[string]string{"WAYFINDER_DB_URL": "x", "WAYFINDER_SECRET_KEY": valid}), nil)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfg.secretKey) != 32 {
		t.Errorf("secretKey len = %d, want 32", len(cfg.secretKey))
	}

	for _, v := range []string{"", "not-base64!!", base64.StdEncoding.EncodeToString([]byte("too short"))} {
		cfg, err := loadConfig(envFunc(map[string]string{"WAYFINDER_DB_URL": "x", "WAYFINDER_SECRET_KEY": v}), nil)
		if err != nil {
			t.Fatalf("loadConfig(%q): %v", v, err)
		}
		if cfg.secretKey != nil {
			t.Errorf("secretKey for %q = %v, want nil", v, cfg.secretKey)
		}
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

func TestLoadConfigBackendDefaultsToMemory(t *testing.T) {
	cfg, err := loadConfig(envFunc(map[string]string{"WAYFINDER_DB_URL": "x"}), nil)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.backend != backendMemory {
		t.Errorf("backend = %q, want memory", cfg.backend)
	}
	if cfg.fireflyNet != "host" {
		t.Errorf("fireflyNet = %q, want host", cfg.fireflyNet)
	}
}

func TestLoadConfigDockerRequiresImage(t *testing.T) {
	// docker backend without an image is rejected.
	_, err := loadConfig(envFunc(map[string]string{
		"WAYFINDER_DB_URL":               "x",
		"WAYFINDER_ORCHESTRATOR_BACKEND": "docker",
	}), nil)
	if err == nil {
		t.Fatal("docker backend without WAYFINDER_FIREFLY_IMAGE should fail")
	}
	// with an image it is accepted.
	cfg, err := loadConfig(envFunc(map[string]string{
		"WAYFINDER_DB_URL":               "x",
		"WAYFINDER_ORCHESTRATOR_BACKEND": "docker",
		"WAYFINDER_FIREFLY_IMAGE":        "firefly:1.0",
		"WAYFINDER_FIREFLY_NETWORK":      "bridge",
	}), nil)
	if err != nil {
		t.Fatalf("docker backend with image: %v", err)
	}
	if cfg.backend != backendDocker || cfg.fireflyImg != "firefly:1.0" || cfg.fireflyNet != "bridge" {
		t.Fatalf("docker config not parsed: %+v", cfg)
	}
}

func TestLoadConfigUnknownBackendErrors(t *testing.T) {
	_, err := loadConfig(envFunc(map[string]string{
		"WAYFINDER_DB_URL":               "x",
		"WAYFINDER_ORCHESTRATOR_BACKEND": "k8s",
	}), nil)
	if err == nil {
		t.Fatal("an unknown backend should be an error")
	}
}
