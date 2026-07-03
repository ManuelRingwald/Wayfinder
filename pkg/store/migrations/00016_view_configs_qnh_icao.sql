-- +migrate up
-- CBD-3 (QNH per-Tenant, ADR 0016/0017): optional per-tenant aerodrome ICAO whose
-- QNH (altimeter setting, hPa) is shown in the ASD header infobox. This is a REAL
-- aerodrome location indicator (e.g. "EDDH") passed to the NOAA/AWC METAR poller —
-- distinct from the display-only header label `icao` (a sector/FIR string like
-- "EDGG·KTG"). NULL means "unset": the tenant sees no QNH (the source is on by
-- default, but there is no airport to poll). Short ICAO code (4 chars).
ALTER TABLE view_configs ADD COLUMN qnh_icao TEXT;

-- +migrate down
ALTER TABLE view_configs DROP COLUMN IF EXISTS qnh_icao;
