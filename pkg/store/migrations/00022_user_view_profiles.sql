-- +migrate up
-- View-Profile (ADR 0023): a user's personal, NAMED display presets for the ASD
-- scope — up to three per user, one optionally the login default. `settings` is
-- an OPAQUE JSON object of frontend display toggles (layer visibility, range-ring
-- config, FL filter, base map, …); the backend stores and returns it verbatim and
-- never interprets it, so adding a new toggle needs no migration. Strictly
-- per-user (ON DELETE CASCADE with the user). This is display config, not track
-- data — CAT062 carries none of it. Distinct from view_configs (tenant/user MAP
-- FRAMING: centre/zoom/AOI/AoR), which is unaffected.
CREATE TABLE user_view_profiles (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    settings   JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_user_view_profiles_user ON user_view_profiles (user_id);
-- At most one default profile per user (partial unique index).
CREATE UNIQUE INDEX user_view_profiles_default_uniq ON user_view_profiles (user_id) WHERE is_default;

-- +migrate down
DROP TABLE IF EXISTS user_view_profiles;
