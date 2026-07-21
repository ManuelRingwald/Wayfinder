package mapconfig

import (
	"context"
	"errors"
)

// errNoTenantScope is returned by a write on a TenantSetting without a concrete
// tenant scope (tenantID 0 / nil store) — a per-tenant override cannot be stored
// for "no tenant".
var errNoTenantScope = errors.New("mapconfig: no tenant scope for a per-tenant override")

// TenantStore is the per-tenant key/value surface mapconfig needs for tenant-scoped
// overrides (satisfied by *store.TenantMapSettingsRepo). Like Store, values are
// opaque strings; this file owns the "tenant-override vs global-default" semantics.
type TenantStore interface {
	Get(ctx context.Context, tenantID int64, key string) (string, bool, error)
	Set(ctx context.Context, tenantID int64, key, value string) error
	Delete(ctx context.Context, tenantID int64, key string) error
}

// TenantSetting layers an optional per-tenant override on top of a global Setting
// (Epic #307 hybrid, ADR 0035). The effective value resolves in three tiers:
//
//	tenant-override (tenant_map_settings) ?? global override (platform_settings) ?? env default
//
// A tenantID of 0 — a platform admin has no tenant (ONB-3) — or a nil store skips
// the tenant tier entirely and behaves exactly like the wrapped global Setting, so
// existing global behaviour is preserved. A tenant store error degrades to the
// global value (never fail the read on a per-tenant hiccup), mirroring Setting.
type TenantSetting struct {
	store  TenantStore
	global *Setting
}

// NewTenantSetting wraps a global Setting with a per-tenant override layer. store
// may be nil (then the setting is effectively global-only).
func NewTenantSetting(store TenantStore, global *Setting) *TenantSetting {
	return &TenantSetting{store: store, global: global}
}

// Key returns the underlying platform_settings/tenant key.
func (t *TenantSetting) Key() string { return t.global.key }

// Global returns the wrapped global Setting (for reading the platform-wide value).
func (t *TenantSetting) Global() *Setting { return t.global }

// Effective returns the value in force for the given tenant: the tenant override
// when present and non-empty, else the global effective value (global override or
// env default). tenantID 0 / nil store → the global value.
func (t *TenantSetting) Effective(ctx context.Context, tenantID int64) (string, error) {
	if t.store != nil && tenantID != 0 {
		v, ok, err := t.store.Get(ctx, tenantID, t.global.key)
		if err == nil && ok && v != "" {
			return v, nil
		}
		// A store error or an absent/empty row falls through to the global value:
		// a per-tenant DB hiccup must not break the read (degrade to platform default).
	}
	return t.global.Effective(ctx)
}

// Overridden reports whether THIS tenant has an explicit override row (vs. running
// on the global/env value). Drives the admin UI's per-tenant "overridden" chip.
// Always false for tenantID 0 / nil store.
func (t *TenantSetting) Overridden(ctx context.Context, tenantID int64) (bool, error) {
	if t.store == nil || tenantID == 0 {
		return false, nil
	}
	_, ok, err := t.store.Get(ctx, tenantID, t.global.key)
	return ok, err
}

// Set writes a per-tenant override. An empty value is a reset (the row is deleted),
// so "clear the field" means "fall back to the global/env value" — the safe
// operator intent, matching Setting.Set. tenantID 0 / nil store is a no-op error.
func (t *TenantSetting) Set(ctx context.Context, tenantID int64, value string) error {
	if t.store == nil || tenantID == 0 {
		return errNoTenantScope
	}
	if value == "" {
		return t.store.Delete(ctx, tenantID, t.global.key)
	}
	return t.store.Set(ctx, tenantID, t.global.key, value)
}

// Reset removes this tenant's override so the setting falls back to global/env.
func (t *TenantSetting) Reset(ctx context.Context, tenantID int64) error {
	if t.store == nil || tenantID == 0 {
		return errNoTenantScope
	}
	return t.store.Delete(ctx, tenantID, t.global.key)
}
