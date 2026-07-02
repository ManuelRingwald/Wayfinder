-- +migrate up
-- Reskin Häppchen 3a (Design System v1): optional per-tenant ICAO location
-- indicator for the ASD header (e.g. "EDGG" / "EDGG·KTG"). The CAT062 wire
-- contract carries no sector/FIR identity, so this header label is CONFIG, not
-- track data — surfaced only when set (Vorgabe: keine Fake-UI). It lives on the
-- view config next to the other per-tenant display settings. NULL means "unset":
-- the header simply omits the ICAO chip. Short free-form label.
ALTER TABLE view_configs ADD COLUMN icao TEXT;

-- +migrate down
ALTER TABLE view_configs DROP COLUMN IF EXISTS icao;
