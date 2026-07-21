// Package mapconfig is the runtime configuration plane for the map-data
// subsystems (weather, base map, radar coverage, aeronautical) — the foundation
// for making them admin-editable without a restart (Epic #307, K0).
//
// The design (ADR 0033):
//   - Every setting has a START-UP ENV DEFAULT and an optional DB OVERRIDE stored
//     in platform_settings. The EFFECTIVE value is the DB override when present,
//     else the env default. Clearing the override ("reset to default") deletes
//     the row. So a fresh deployment with no DB config behaves exactly as before
//     (12-Factor stays intact); the admin UI only overrides when it wants to.
//   - A change publishes to a hot-reload Registry (reload.go) so the owning
//     service re-reads and re-applies without a restart, keeping its last-good
//     value on failure (never crash on operator input — CLAUDE §7).
//   - Admin-set URLs are fetched server-side, so they pass ValidateFetchURL
//     (urlguard.go) before they are stored (SSRF boundary).
//
// Secrets (e.g. the OpenAIP key) are NOT handled here — they keep the sealed
// pattern (pkg/secret + platform_settings) so a plaintext value never reaches
// this plane or the UI. mapconfig is for non-secret values (URLs, themes, flags,
// JSON blobs).
package mapconfig

import "context"

// Store is the key/value surface mapconfig needs (satisfied by
// *store.SettingsRepo). Values are opaque strings; mapconfig owns the "override
// vs default" semantics on top of it.
type Store interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

// Setting is one runtime-overridable value: a platform_settings key with a
// start-up env default. It is safe for concurrent use if the underlying Store is
// (SettingsRepo is). The env default is captured once at construction (the
// process's start-up config); the override lives in the DB and can change live.
type Setting struct {
	store      Store
	key        string
	envDefault string
}

// NewSetting binds a platform_settings key to its start-up env default.
func NewSetting(store Store, key, envDefault string) *Setting {
	return &Setting{store: store, key: key, envDefault: envDefault}
}

// Key returns the platform_settings key (for logging / diagnostics).
func (s *Setting) Key() string { return s.key }

// Default returns the start-up env default (the value used when not overridden).
func (s *Setting) Default() string { return s.envDefault }

// Effective returns the value in force: the DB override when a row exists, else
// the env default. A store error is returned with the env default as a safe
// fallback, so a transient DB hiccup degrades to the deployment default rather
// than to an empty value.
func (s *Setting) Effective(ctx context.Context) (string, error) {
	v, ok, err := s.store.Get(ctx, s.key)
	if err != nil {
		return s.envDefault, err
	}
	if !ok {
		return s.envDefault, nil
	}
	return v, nil
}

// Overridden reports whether an explicit DB override exists (vs. running on the
// env default). Drives the admin UI's "overridden / default" indicator.
func (s *Setting) Overridden(ctx context.Context) (bool, error) {
	_, ok, err := s.store.Get(ctx, s.key)
	return ok, err
}

// Set writes an explicit override. An empty value is treated as a reset (the row
// is deleted), so "clear the field" means "fall back to the env default" rather
// than "force an empty value" — the safe operator intent.
func (s *Setting) Set(ctx context.Context, value string) error {
	if value == "" {
		return s.store.Delete(ctx, s.key)
	}
	return s.store.Set(ctx, s.key, value)
}

// Reset removes the override so the setting falls back to the env default.
func (s *Setting) Reset(ctx context.Context) error {
	return s.store.Delete(ctx, s.key)
}
