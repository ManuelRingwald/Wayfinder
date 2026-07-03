-- +migrate up
-- AERO-1 (ADR 0018): persist the fetched OpenAIP aeronautical GeoJSON so it
-- survives a redeploy and the fetch model can move from periodic-refresh to
-- fetch-once. One row per (tenant, kind); tenant_id NULL is the global fallback
-- cache (the deployment's WAYFINDER_OPENAIP_API_KEY). geojson is stored verbatim
-- (TEXT) — it is served back to the browser unchanged; feature_count and
-- fetched_at make a staling cache observable (surfaced in the admin status route).
CREATE TABLE aeronautical_cache (
    tenant_id     BIGINT REFERENCES tenants(id) ON DELETE CASCADE, -- NULL = global fallback
    kind          TEXT        NOT NULL,                            -- 'airspace' | 'navaid' | 'waypoint'
    geojson       TEXT        NOT NULL,
    feature_count INTEGER     NOT NULL,
    fetched_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per (tenant, kind) for real tenants, and one row per kind for the global
-- (NULL) cache. Two partial unique indexes because NULLs are distinct in a plain
-- unique index — mirrors the view_configs tenant/user split (migration 00002).
CREATE UNIQUE INDEX aeronautical_cache_tenant_kind
    ON aeronautical_cache (tenant_id, kind) WHERE tenant_id IS NOT NULL;
CREATE UNIQUE INDEX aeronautical_cache_global_kind
    ON aeronautical_cache (kind) WHERE tenant_id IS NULL;

-- +migrate down
DROP TABLE IF EXISTS aeronautical_cache;
