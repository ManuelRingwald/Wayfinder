-- +migrate up
-- ORCH-2c (ADR 0012 §6, NFR-SEC-004): per-feed source credentials. A feed's
-- source_config references credentials by handle only (cred_ref); the actual
-- secret values (OpenSky client secrets, FLARM/radar access) live here, one row
-- per (feed, cred_ref). The value is stored ENCRYPTED (AES-256-GCM, pkg/secret):
-- ciphertext holds base64(nonce||ciphertext||tag), so a database leak alone never
-- yields plaintext credentials. The encryption key is deployment-managed
-- (WAYFINDER_SECRET_KEY), never stored in the database.
--
-- The secret value is never returned to the browser (the admin API reports only
-- whether a ref is configured) and is resolved only by the separate orchestrator
-- control plane at container launch (mirrors the OpenAIP key isolation, ONB-6).
-- ON DELETE CASCADE removes a feed's secrets when the feed is deleted.
CREATE TABLE feed_secrets (
    feed_id    BIGINT NOT NULL REFERENCES feeds (id) ON DELETE CASCADE,
    cred_ref   TEXT NOT NULL,
    ciphertext TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (feed_id, cred_ref)
);

-- +migrate down
DROP TABLE IF EXISTS feed_secrets;
