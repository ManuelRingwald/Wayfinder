-- +migrate up
-- ORCH-2c 3b (ADR 0012 §5): change-driven reconcile. The orchestrator control
-- plane LISTENs on the 'wayfinder_reconcile' channel and converges immediately
-- when the desired state changes, instead of waiting up to one reconcile interval.
--
-- These triggers NOTIFY on every change to the tables that define the desired set
-- of running tracker instances:
--   * feeds         — a feed (and its source_config/coverage_bbox) is the spec;
--   * subscriptions — a feed is desired iff it has >= 1 active subscription.
--
-- The triggers are STATEMENT-level (not row-level): a bulk change emits a single
-- notification, which is enough because the listener recomputes the *full* desired
-- state on any signal — the payload is therefore empty. Doing the NOTIFY in the
-- database (not the application) catches every writer, including manual SQL, not
-- just the wayfinder server's own mutations.
--
-- feed_secrets was intentionally NOT covered here originally: a secret value was
-- not spec-relevant until container injection landed (ORCH-5). That premise no
-- longer holds — since ORCH-5b a feed secret resolves into the tracker spec and
-- its spec hash — so migration 00020 adds the feed_secrets trigger (#177). This
-- comment is kept as history; the trigger itself lives in 00020.
CREATE OR REPLACE FUNCTION wayfinder_notify_reconcile() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('wayfinder_reconcile', '');
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER feeds_notify_reconcile
    AFTER INSERT OR UPDATE OR DELETE ON feeds
    FOR EACH STATEMENT EXECUTE FUNCTION wayfinder_notify_reconcile();

CREATE TRIGGER subscriptions_notify_reconcile
    AFTER INSERT OR UPDATE OR DELETE ON subscriptions
    FOR EACH STATEMENT EXECUTE FUNCTION wayfinder_notify_reconcile();

-- +migrate down
DROP TRIGGER IF EXISTS subscriptions_notify_reconcile ON subscriptions;
DROP TRIGGER IF EXISTS feeds_notify_reconcile ON feeds;
DROP FUNCTION IF EXISTS wayfinder_notify_reconcile();
