-- +migrate up
-- AP6 (ADR 0009): per-access lifecycle. A user (access account) can be paused
-- without losing its row/configuration, and reactivated later. The status is a
-- closed set enforced by a CHECK constraint; existing rows default to 'active'
-- so the migration is non-breaking. Tenant-level pause reuses the tenants.status
-- column that already exists (00001_init); login enforces both fail-closed.
ALTER TABLE users
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'paused'));

-- +migrate down
ALTER TABLE users DROP COLUMN IF EXISTS status;
