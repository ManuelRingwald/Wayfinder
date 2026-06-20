-- +migrate up
-- A user has at most one view-config override. Replace the plain user index with
-- a partial UNIQUE index so UpsertUserOverride can use ON CONFLICT (user_id). The
-- tenant default (user_id IS NULL) stays governed by idx_view_configs_tenant_default.
DROP INDEX IF EXISTS idx_view_configs_user;
CREATE UNIQUE INDEX idx_view_configs_user ON view_configs (user_id) WHERE user_id IS NOT NULL;

-- +migrate down
DROP INDEX IF EXISTS idx_view_configs_user;
CREATE INDEX idx_view_configs_user ON view_configs (user_id);
