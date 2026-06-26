-- +migrate up
-- ONB-5 (ADR 0011): feed lifecycle from the admin UI. With feeds now created and
-- deleted from the browser (not only via the CLI), the feed name becomes the
-- human-facing key the operator picks from in the dashboard. A unique constraint
-- keeps that key unambiguous and lets the API return a clean 409 on a duplicate
-- (mirroring the tenant-slug uniqueness) instead of silently cataloguing two feeds
-- the operator cannot tell apart. Existing catalogues built by these migrations
-- never contain duplicate names, so adding the constraint is non-destructive.
ALTER TABLE feeds ADD CONSTRAINT feeds_name_unique UNIQUE (name);

-- +migrate down
ALTER TABLE feeds DROP CONSTRAINT IF EXISTS feeds_name_unique;
