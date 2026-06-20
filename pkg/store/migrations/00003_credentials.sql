-- +migrate up
-- Builtin-mode credentials, kept in a separate table from users: only users that
-- log in with a local password have a row here; OIDC/proxy users have none
-- (ADR 0006 §5). password_hash is an argon2id PHC string (pkg/auth).
CREATE TABLE credentials (
    user_id       BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate down
DROP TABLE IF EXISTS credentials;
