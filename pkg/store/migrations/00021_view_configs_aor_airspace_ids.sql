-- +migrate up
-- ASD-014 (ADR 0021): the tenant's Area of Responsibility — the set of OpenAIP
-- airspace ids (stable `_id`, surfaced since ASD-014.1) that make up the
-- controlled volumes (CTR/TMA) the ASD highlights distinctly from the surrounding
-- context airspace. Stored as a JSON array of strings; NULL means "no AoR
-- configured". The id is the robust key: airspace names drift per AIRAC cycle.
-- This is display config, not track data — CAT062 carries no sector identity.
ALTER TABLE view_configs ADD COLUMN aor_airspace_ids JSONB;

-- +migrate down
ALTER TABLE view_configs DROP COLUMN IF EXISTS aor_airspace_ids;
