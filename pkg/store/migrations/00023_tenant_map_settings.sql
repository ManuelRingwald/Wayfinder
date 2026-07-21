-- +migrate up
-- Per-tenant map-data overrides (Epic #307, hybrid scope, ADR 0035). A tenant may
-- override a small subset of the global map-data settings — currently the base map
-- theme + style URL — without affecting any other tenant. The EFFECTIVE value is
-- resolved as: tenant-override ?? global (platform_settings) ?? env default.
--
-- Kept SEPARATE from the global `platform_settings` table so the platform-wide
-- values (including the SEALED global OpenAIP key) are untouched and the two
-- scopes cannot collide. Non-secret values only — same rule as platform_settings.
-- ON DELETE CASCADE: removing a tenant drops its overrides.
CREATE TABLE tenant_map_settings (
    tenant_id  BIGINT      NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key        TEXT        NOT NULL,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

-- +migrate down
DROP TABLE IF EXISTS tenant_map_settings;
