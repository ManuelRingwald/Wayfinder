-- +migrate up
-- AP1 (ADR 0009): collapse the three-role model (operator / tenant_admin /
-- super_admin) into two roles (user / admin). Every privileged user becomes
-- admin; every unprivileged user becomes user. The migration is idempotent:
-- rows already holding the new values are untouched by the WHERE clause.
UPDATE users SET role = 'admin' WHERE role IN ('super_admin', 'tenant_admin');
UPDATE users SET role = 'user'  WHERE role = 'operator';

-- +migrate down
-- Reversing the merge is lossy (we cannot tell former super_admin from
-- former tenant_admin), so the down section re-maps uniformly to tenant_admin.
UPDATE users SET role = 'tenant_admin' WHERE role = 'admin';
UPDATE users SET role = 'operator'     WHERE role = 'user';
