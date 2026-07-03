package main

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/secret"
)

// memSettings is an in-memory settingsStore for the global-key adapter tests.
type memSettings struct{ m map[string]string }

func newMemSettings() *memSettings { return &memSettings{m: map[string]string{}} }

func (s *memSettings) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := s.m[key]
	return v, ok, nil
}
func (s *memSettings) Set(_ context.Context, key, value string) error { s.m[key] = value; return nil }
func (s *memSettings) Delete(_ context.Context, key string) error     { delete(s.m, key); return nil }

func testCipher(t *testing.T) *secret.Cipher {
	t.Helper()
	c, err := secret.NewCipher(bytes.Repeat([]byte{0x2a}, secret.KeySize))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	return c
}

func TestGlobalOpenAIPSealRoundTrip(t *testing.T) {
	ctx := context.Background()
	ms := newMemSettings()
	g := newGlobalOpenAIP(ms, testCipher(t), "env-fallback", slog.Default())

	if !g.Available() {
		t.Fatal("cipher present → Available should be true")
	}
	if configured, _ := g.Configured(ctx); configured {
		t.Error("nothing set yet → not configured")
	}
	// Before any set, the effective key is the env fallback.
	if got := g.effectiveKey(ctx); got != "env-fallback" {
		t.Errorf("effectiveKey = %q, want env fallback", got)
	}

	if err := g.SetKey(ctx, "live-key"); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	if configured, _ := g.Configured(ctx); !configured {
		t.Error("after set → configured")
	}
	// Stored value must be sealed, never the plaintext.
	if stored := ms.m[openaipGlobalSettingKey]; stored == "" || stored == "live-key" {
		t.Errorf("stored value %q must be a non-empty ciphertext, not plaintext", stored)
	}
	if got := g.effectiveKey(ctx); got != "live-key" {
		t.Errorf("effectiveKey = %q, want decrypted live-key", got)
	}

	// Clearing removes the row and falls back to env.
	if err := g.SetKey(ctx, ""); err != nil {
		t.Fatalf("SetKey clear: %v", err)
	}
	if configured, _ := g.Configured(ctx); configured {
		t.Error("after clear → not configured")
	}
	if got := g.effectiveKey(ctx); got != "env-fallback" {
		t.Errorf("effectiveKey after clear = %q, want env fallback", got)
	}
}

func TestGlobalOpenAIPNoCipher(t *testing.T) {
	ctx := context.Background()
	g := newGlobalOpenAIP(newMemSettings(), nil, "env-fallback", slog.Default())

	if g.Available() {
		t.Error("no cipher → Available should be false")
	}
	if err := g.SetKey(ctx, "k"); err == nil {
		t.Error("storing a key without a cipher must fail (no plaintext at rest)")
	}
	// Clearing is still allowed, and the effective key is the env fallback.
	if err := g.SetKey(ctx, ""); err != nil {
		t.Errorf("clear without cipher should be allowed: %v", err)
	}
	if got := g.effectiveKey(ctx); got != "env-fallback" {
		t.Errorf("effectiveKey = %q, want env fallback", got)
	}
}

func TestGlobalOpenAIPDecryptFailureFallsBackToEnv(t *testing.T) {
	ctx := context.Background()
	ms := newMemSettings()
	g := newGlobalOpenAIP(ms, testCipher(t), "env-fallback", slog.Default())
	if err := g.SetKey(ctx, "live-key"); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	// Simulate a rotated WAYFINDER_SECRET_KEY: a different cipher can't open the blob.
	other, err := secret.NewCipher(bytes.Repeat([]byte{0x99}, secret.KeySize))
	if err != nil {
		t.Fatal(err)
	}
	g.cipher = other
	if got := g.effectiveKey(ctx); got != "env-fallback" {
		t.Errorf("undecryptable blob should fall back to env, got %q", got)
	}
}
