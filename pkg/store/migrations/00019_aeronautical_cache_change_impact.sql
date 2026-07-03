-- +migrate up
-- AERO-3 (ADR 0018): change-impact of the last OpenAIP refresh, per (tenant, kind).
-- At each fetch Wayfinder diffs the fresh data against the previously-persisted one
-- and records how much churned: the previous feature count and how many features
-- were added/removed (by content — an in-place edit counts as one removed + one
-- added). NULL means "no prior comparison yet" (the very first fetch). The change
-- time is the existing fetched_at. Robust: counts are exact and need no assumption
-- about a stable OpenAIP feature id.
ALTER TABLE aeronautical_cache ADD COLUMN prev_feature_count INTEGER;
ALTER TABLE aeronautical_cache ADD COLUMN added INTEGER;
ALTER TABLE aeronautical_cache ADD COLUMN removed INTEGER;

-- +migrate down
ALTER TABLE aeronautical_cache DROP COLUMN IF EXISTS prev_feature_count;
ALTER TABLE aeronautical_cache DROP COLUMN IF EXISTS added;
ALTER TABLE aeronautical_cache DROP COLUMN IF EXISTS removed;
