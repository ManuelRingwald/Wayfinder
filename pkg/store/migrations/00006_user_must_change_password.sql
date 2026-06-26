-- +migrate up
-- ONB-1 (ADR 0011): zero-touch onboarding. A freshly seeded default admin
-- (subject "admin") is created with a known default password, so the very first
-- thing the operator must do after the first login is replace it. This flag gates
-- that: while true, every admin route except "change my own password" is refused
-- fail-closed (enforced in pkg/adminapi). Existing rows default to false (the
-- migration is non-breaking; accounts provisioned before this flag are unaffected).
ALTER TABLE users
    ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT false;

-- +migrate down
ALTER TABLE users DROP COLUMN IF EXISTS must_change_password;
