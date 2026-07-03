package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AeroCacheRepo persists the fetched OpenAIP aeronautical GeoJSON (AERO-1, ADR
// 0018) so it survives a redeploy and the fetch model can be fetch-once instead of
// periodic. One row per (tenant, kind); a NULL tenant id is the global fallback
// cache. The GeoJSON is stored verbatim (TEXT) — the caller marshals/unmarshals,
// so this repo stays free of the aeronautical GeoJSON types.
type AeroCacheRepo struct {
	db *pgxpool.Pool
}

// NewAeroCacheRepo returns an AeroCacheRepo backed by the given pool.
func NewAeroCacheRepo(db *pgxpool.Pool) *AeroCacheRepo { return &AeroCacheRepo{db: db} }

// AeroCacheEntry is one persisted (tenant, kind) cache row.
type AeroCacheEntry struct {
	GeoJSON      string
	FeatureCount int
	FetchedAt    time.Time
}

// Load returns the persisted cache for (tenantID, kind), or ok=false when there is
// no row yet. tenantID nil selects the global fallback row.
func (r *AeroCacheRepo) Load(ctx context.Context, tenantID *int64, kind string) (AeroCacheEntry, bool, error) {
	// Two queries because a NULL tenant_id needs `IS NULL` (not `= NULL`), matching
	// the two partial unique indexes.
	var (
		row pgx.Row
		e   AeroCacheEntry
	)
	if tenantID == nil {
		const q = `SELECT geojson, feature_count, fetched_at FROM aeronautical_cache WHERE tenant_id IS NULL AND kind = $1`
		row = r.db.QueryRow(ctx, q, kind)
	} else {
		const q = `SELECT geojson, feature_count, fetched_at FROM aeronautical_cache WHERE tenant_id = $1 AND kind = $2`
		row = r.db.QueryRow(ctx, q, *tenantID, kind)
	}
	if err := row.Scan(&e.GeoJSON, &e.FeatureCount, &e.FetchedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AeroCacheEntry{}, false, nil
		}
		return AeroCacheEntry{}, false, wrap("load aero cache", err)
	}
	return e, true, nil
}

// Save upserts the cache row for (tenantID, kind). tenantID nil targets the global
// fallback row. Idempotent via the partial unique indexes.
func (r *AeroCacheRepo) Save(ctx context.Context, tenantID *int64, kind, geojson string, featureCount int, fetchedAt time.Time) error {
	if tenantID == nil {
		const q = `INSERT INTO aeronautical_cache (tenant_id, kind, geojson, feature_count, fetched_at)
			VALUES (NULL, $1, $2, $3, $4)
			ON CONFLICT (kind) WHERE tenant_id IS NULL
			DO UPDATE SET geojson = EXCLUDED.geojson, feature_count = EXCLUDED.feature_count, fetched_at = EXCLUDED.fetched_at`
		if _, err := r.db.Exec(ctx, q, kind, geojson, featureCount, fetchedAt); err != nil {
			return wrap("save global aero cache", err)
		}
		return nil
	}
	const q = `INSERT INTO aeronautical_cache (tenant_id, kind, geojson, feature_count, fetched_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, kind) WHERE tenant_id IS NOT NULL
		DO UPDATE SET geojson = EXCLUDED.geojson, feature_count = EXCLUDED.feature_count, fetched_at = EXCLUDED.fetched_at`
	if _, err := r.db.Exec(ctx, q, *tenantID, kind, geojson, featureCount, fetchedAt); err != nil {
		return wrap("save tenant aero cache", err)
	}
	return nil
}

// AeroCacheStatus is the summary the admin status route exposes for a tenant's
// persisted aeronautical cache (AERO-1): when it was last fetched and how many
// features are cached in total across kinds.
type AeroCacheStatus struct {
	FetchedAt    time.Time
	FeatureCount int
}

// Status returns the latest fetched_at and the summed feature_count across all
// kinds for tenantID (nil = global), or ok=false when nothing is cached yet.
func (r *AeroCacheRepo) Status(ctx context.Context, tenantID *int64) (AeroCacheStatus, bool, error) {
	var (
		row       pgx.Row
		fetchedAt *time.Time
		total     *int
	)
	if tenantID == nil {
		const q = `SELECT max(fetched_at), sum(feature_count) FROM aeronautical_cache WHERE tenant_id IS NULL`
		row = r.db.QueryRow(ctx, q)
	} else {
		const q = `SELECT max(fetched_at), sum(feature_count) FROM aeronautical_cache WHERE tenant_id = $1`
		row = r.db.QueryRow(ctx, q, *tenantID)
	}
	if err := row.Scan(&fetchedAt, &total); err != nil {
		return AeroCacheStatus{}, false, wrap("aero cache status", err)
	}
	if fetchedAt == nil { // no rows aggregated → nothing cached
		return AeroCacheStatus{}, false, nil
	}
	st := AeroCacheStatus{FetchedAt: *fetchedAt}
	if total != nil {
		st.FeatureCount = *total
	}
	return st, true, nil
}
