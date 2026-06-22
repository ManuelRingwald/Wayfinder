// Package feature provides Wayfinder 2.0's per-tenant feature entitlement
// service: HasFeature(...) answers "may this tenant use feature X" from data
// (the entitlements table), decoupled from billing (ADR 0005 §4; billing is a
// separate plane, WF2-51 dormant).
//
// Security — feature gating is FAIL-CLOSED. An unknown feature key or any
// backend error evaluates to false (default-deny) and is surfaced as a warn log
// plus a metric, never as an accidental grant. A feature gate must never open
// on uncertainty.
package feature

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
)

// ErrUnknownFeature is returned by Set when the key is not in the catalog. The
// admin API maps it to 400 so an operator can never persist a typo'd or removed
// feature key (which would then silently fail closed forever).
var ErrUnknownFeature = errors.New("feature: unknown feature key")

// Store is the persistence dependency, satisfied by *store.EntitlementRepo. It
// is kept as a narrow interface here so the service is unit-testable without a
// database (and so pkg/feature stays a leaf with no store/pgx import).
type Store interface {
	IsEnabled(ctx context.Context, tenantID int64, featureKey string) (bool, error)
	ListByTenant(ctx context.Context, tenantID int64) (map[string]bool, error)
	Set(ctx context.Context, tenantID int64, featureKey string, enabled bool) error
}

// Service answers per-tenant feature checks, fail-closed. It is safe for
// concurrent use: the only mutable state is the atomic fail-closed counters.
type Service struct {
	store  Store
	logger *slog.Logger

	dbErrors    atomic.Int64 // fail-closed because the store returned an error
	unknownKeys atomic.Int64 // fail-closed because the key is not in the catalog
}

// New returns a feature Service backed by store. A nil logger falls back to
// slog.Default() so fail-closed warnings are never silently dropped.
func New(store Store, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{store: store, logger: logger}
}

// HasFeature reports whether tenantID may use key. FAIL-CLOSED: an unknown key
// or any store error yields false (default-deny) plus a warn log and a metric
// increment.
func (s *Service) HasFeature(ctx context.Context, tenantID int64, key Key) bool {
	if !IsKnown(key) {
		s.unknownKeys.Add(1)
		s.logger.WarnContext(ctx, "feature check for unknown key denied (fail-closed)",
			"feature_key", string(key), "tenant_id", tenantID)
		return false
	}
	enabled, err := s.store.IsEnabled(ctx, tenantID, string(key))
	if err != nil {
		s.dbErrors.Add(1)
		s.logger.WarnContext(ctx, "feature check failed, denying (fail-closed)",
			"feature_key", string(key), "tenant_id", tenantID, "error", err)
		return false
	}
	return enabled
}

// Effective returns every known feature with its enabled state for tenantID
// (default-deny for keys the tenant has no row for). Keys stored in the database
// that are no longer in the catalog are ignored. On a store error it still
// returns a fully default-denied map alongside the error, so a caller that
// ignores the error nonetheless fails closed.
func (s *Service) Effective(ctx context.Context, tenantID int64) (map[Key]bool, error) {
	out := make(map[Key]bool, len(catalog))
	for _, k := range All() {
		out[k] = false // default-deny baseline
	}
	stored, err := s.store.ListByTenant(ctx, tenantID)
	if err != nil {
		s.dbErrors.Add(1)
		s.logger.WarnContext(ctx, "listing entitlements failed, defaulting to deny (fail-closed)",
			"tenant_id", tenantID, "error", err)
		return out, err
	}
	for k, enabled := range stored {
		if IsKnown(Key(k)) {
			out[Key(k)] = enabled
		}
	}
	return out, nil
}

// Set enables or disables a feature for a tenant. It rejects keys outside the
// catalog with ErrUnknownFeature, so the database never accumulates flags that
// no code reads — keeping the catalog the single source of truth for "which
// features exist". Writing is a super_admin-only action at the admin API edge.
func (s *Service) Set(ctx context.Context, tenantID int64, key Key, enabled bool) error {
	if !IsKnown(key) {
		return ErrUnknownFeature
	}
	return s.store.Set(ctx, tenantID, string(key), enabled)
}

// DBErrorCount returns how many checks failed closed due to a store error
// (exposed via /metrics as wayfinder_feature_check_failclosed_total{reason="db_error"}).
func (s *Service) DBErrorCount() int64 { return s.dbErrors.Load() }

// UnknownKeyCount returns how many checks failed closed due to an unknown key
// (exposed via /metrics as wayfinder_feature_check_failclosed_total{reason="unknown_key"}).
func (s *Service) UnknownKeyCount() int64 { return s.unknownKeys.Load() }
