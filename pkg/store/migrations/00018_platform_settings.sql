-- +migrate up
-- AERO-2 (ADR 0018): a small key/value store for platform-wide settings that are
-- managed at runtime through the admin UI rather than only via env-vars. First
-- user: the GLOBAL OpenAIP API key (key = 'openaip_global_key'), stored **sealed**
-- (AES-256-GCM via pkg/secret, WAYFINDER_SECRET_KEY) — value holds ciphertext, never
-- plaintext. Generic on purpose so later platform settings reuse the same table.
CREATE TABLE platform_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate down
DROP TABLE IF EXISTS platform_settings;
