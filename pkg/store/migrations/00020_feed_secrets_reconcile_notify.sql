-- +migrate up
-- #177 (ORCH-5b): couple feed-secret changes to the change-driven reconcile.
--
-- 00012 deliberately left feed_secrets out of the reconcile NOTIFY, on the
-- premise that a secret value was "not yet spec-relevant until container
-- injection lands (ORCH-5)". That premise no longer holds: ORCH-5b resolves a
-- feed's secrets into the tracker spec (orchestrator/desired.go →
-- Spec.ResolvedSecrets) and the credential env feeds the spec hash
-- (dockerbackend/sources.go), so a changed secret must re-apply/restart the
-- spawned tracker. Without this trigger a newly stored OpenSky credential was
-- only picked up on the next interval sweep — to the operator it looked like the
-- key had "no effect" (the source kept running anonymously and rate-limited).
--
-- Same shape as the feeds/subscriptions triggers (00012): STATEMENT-level with an
-- empty payload (the listener recomputes the full desired state on any signal),
-- and in the database so it catches every writer — including the secrets admin
-- API, which does not itself emit a notify.
CREATE TRIGGER feed_secrets_notify_reconcile
    AFTER INSERT OR UPDATE OR DELETE ON feed_secrets
    FOR EACH STATEMENT EXECUTE FUNCTION wayfinder_notify_reconcile();

-- +migrate down
DROP TRIGGER IF EXISTS feed_secrets_notify_reconcile ON feed_secrets;
