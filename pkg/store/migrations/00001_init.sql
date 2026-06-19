-- +migrate up
-- Wayfinder 2.0 multi-tenant configuration & identity schema (ADR 0005/0006).
-- Tenants are isolated organisations; feeds are a global catalogue that tenants
-- subscribe to (Hybrid model, ADR 0005 §2). Sensor mix is a feed property, not a
-- per-track tag (ADR 0005 §8). The runner applies everything in the "up" section
-- (above the "+migrate down" marker) inside a single transaction.

CREATE TABLE tenants (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- subject is the OIDC subject (proxy mode) or the username (builtin mode),
-- ADR 0006 §5. role gates authorisation: operator | tenant_admin | super_admin.
CREATE TABLE users (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject    TEXT NOT NULL UNIQUE,
    email      TEXT,
    role       TEXT NOT NULL DEFAULT 'operator',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_tenant ON users (tenant_id);

-- Global feed catalogue. sensor_mix is informational metadata (e.g. ["ADS-B"],
-- ["PSR","SSR","ADS-B"]); visibility is governed solely by subscriptions.
CREATE TABLE feeds (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            TEXT NOT NULL,
    multicast_group TEXT NOT NULL,
    port            INTEGER NOT NULL,
    region          TEXT,
    sensor_mix      JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Which feeds a tenant is allowed to see (the heart of cross-tenant isolation:
-- a track only reaches a tenant whose subscription covers its feed, NFR-SEC-003).
CREATE TABLE subscriptions (
    tenant_id  BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    feed_id    BIGINT NOT NULL REFERENCES feeds (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, feed_id)
);

-- Tenant default view (user_id NULL) plus optional per-user overrides. aoi is a
-- bounding box / polygon, layers a free-form toggle map.
CREATE TABLE view_configs (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id    BIGINT REFERENCES users (id) ON DELETE CASCADE,
    center_lat DOUBLE PRECISION NOT NULL,
    center_lon DOUBLE PRECISION NOT NULL,
    zoom       DOUBLE PRECISION NOT NULL DEFAULT 8,
    aoi        JSONB,
    fl_min     INTEGER,
    fl_max     INTEGER,
    layers     JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- At most one tenant-default view config (the row with no user override).
CREATE UNIQUE INDEX idx_view_configs_tenant_default
    ON view_configs (tenant_id) WHERE user_id IS NULL;
CREATE INDEX idx_view_configs_user ON view_configs (user_id);

-- Feature flags as data (ADR 0005 §4): tenant.HasFeature(feature_key).
CREATE TABLE entitlements (
    tenant_id   BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    feature_key TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (tenant_id, feature_key)
);

-- +migrate down
DROP TABLE IF EXISTS entitlements;
DROP TABLE IF EXISTS view_configs;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS feeds;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;
