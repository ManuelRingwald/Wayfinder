-- +migrate up
-- AP7 (ADR 0009 §5): server-side session registry. A stateless signed cookie
-- (ADR 0006) can be neither counted nor revoked before its expiry; a DB-backed
-- registry makes sessions countable (per-access concurrent-session limit) and
-- revocable (immediate pause/delete/logout), and it is shared across replicas
-- (Kubernetes, ADR 0007) — an in-memory registry per pod would undercut both.
--
-- The cookie carries an opaque, signed token; the row's id stores
-- base64url(sha256(token)) rather than the token itself, so a database dump does
-- not hand out usable cookies — the raw token exists only in the browser.
CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,                    -- base64url(sha256(cookie token))
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),  -- first login; anchors the absolute max lifetime (WF2-12.6)
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    client_meta  JSONB NOT NULL DEFAULT '{}'::jsonb   -- best-effort user agent / IP for an admin session view
);

-- Count and revoke a single access's sessions (login limit check, pause/delete
-- cascade). Also serves the tenant-pause cascade via a users sub-select.
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
-- Sweep expired rows (janitor) without scanning the whole table.
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- Per-access concurrent-session limit (ADR 0009 §5). NULL = fall back to the
-- deployment default (WAYFINDER_SESSION_LIMIT_DEFAULT); a non-negative value
-- overrides it for this access. 0 means unlimited (enforcement off). Existing
-- rows default to NULL, so the migration is non-breaking.
ALTER TABLE users
    ADD COLUMN session_limit INT
    CHECK (session_limit IS NULL OR session_limit >= 0);

-- +migrate down
ALTER TABLE users DROP COLUMN IF EXISTS session_limit;
DROP TABLE IF EXISTS sessions;
