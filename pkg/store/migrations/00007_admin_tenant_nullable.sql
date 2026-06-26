-- +migrate up
-- ONB-3 (ADR 0011): strict separation of platform admins and tenant users. A
-- platform admin is global — it belongs to no tenant — while a tenant user
-- (pilot/controller account) always belongs to exactly one tenant. Until now
-- every user, admins included, was homed under a tenant (a seed artefact with no
-- meaning for an admin). This migration makes that separation a hard database
-- invariant.
--
-- Step 1: tenant_id becomes nullable (admins carry NULL).
ALTER TABLE users ALTER COLUMN tenant_id DROP NOT NULL;

-- Step 2: detach every existing admin from its (meaningless) tenant. Safe: an
-- admin's tenant was never used for scoping — admin routes ignore TenantID and
-- the /ws read path resolves an admin to an empty scope (TenantID 0).
UPDATE users SET tenant_id = NULL WHERE role = 'admin';

-- Step 3: enforce the invariant — admin XOR tenant. An admin must have no tenant;
-- a user must have one. Fail-closed at the database so neither the application nor
-- a stray manual write can create a half-state.
ALTER TABLE users ADD CONSTRAINT users_role_tenant_check CHECK (
    (role = 'admin' AND tenant_id IS NULL)
    OR
    (role = 'user'  AND tenant_id IS NOT NULL)
);

-- +migrate down
-- Best-effort rollback for local development only. Restoring tenant_id for an
-- admin requires inventing a tenant association that was deliberately removed, so
-- this is semantically lossy: every detached admin is re-homed under the
-- lowest-id tenant. Do not rely on this in production.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_tenant_check;
UPDATE users
SET tenant_id = (SELECT id FROM tenants ORDER BY id LIMIT 1)
WHERE role = 'admin' AND tenant_id IS NULL;
ALTER TABLE users ALTER COLUMN tenant_id SET NOT NULL;
