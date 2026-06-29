-- +migrate up
-- ORCH-1 (ADR 0012): a feed gains the generic source configuration the
-- orchestrator needs to launch its dedicated Firefly instance. Until now a feed
-- was a passive catalogue row (multicast group/port + informational sensor_mix);
-- nobody *produced* the stream. source_config describes, in deliberately
-- Firefly-agnostic terms, which live inputs the feed's tracker should open
-- (adsb_opensky / flarm_aprs / radar_asterix, each with its own parameters — a
-- query bbox for the area-bounded internet sources, a SAC/SIC sensor identity
-- for real radar, an optional cred_ref pointing at a per-feed secret). It is a
-- JSONB array so new source kinds are additive without a schema break, and it
-- defaults to '[]' so every existing feed reads as "no sources yet" (a
-- scenario/placeholder tracker) — non-breaking.
--
-- coverage_bbox is the *coarse outer bound* derived from the source bboxes (plus
-- a margin): the generic geographic limit Wayfinder hands to Firefly
-- (FIREFLY_COVERAGE_BBOX). It is intentionally separate from the tenant's
-- precise inner display/isolation AOI (view_configs.aoi, WF2-21.2), which stays
-- the authoritative billing/security boundary and never moves to Firefly
-- (ADR 0012 §3, coarse-outer vs precise-inner, defense-in-depth). NULL means
-- "not yet derived".
ALTER TABLE feeds ADD COLUMN source_config JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE feeds ADD COLUMN coverage_bbox JSONB;

-- +migrate down
ALTER TABLE feeds DROP COLUMN IF EXISTS coverage_bbox;
ALTER TABLE feeds DROP COLUMN IF EXISTS source_config;
