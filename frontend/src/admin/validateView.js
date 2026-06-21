// validateView mirrors the server-side validateView in pkg/adminapi/adminapi.go.
// It runs client-side so a tenant admin sees an out-of-range or inverted view
// *before* the PUT round-trips a 400. The server stays the source of truth
// (defence in depth): this is a UX courtesy, never the security boundary. Keep
// the two in lockstep — the field names are the wire (snake_case) DTO keys.
//
// Returns an array of human-readable error strings; an empty array means valid.
export function validateView(d) {
  const errors = []
  const num = (v) => typeof v === 'number' && Number.isFinite(v)

  if (!num(d.center_lat) || d.center_lat < -90 || d.center_lat > 90) {
    errors.push('center_lat out of range [-90,90]')
  }
  if (!num(d.center_lon) || d.center_lon < -180 || d.center_lon > 180) {
    errors.push('center_lon out of range [-180,180]')
  }
  if (!num(d.zoom) || d.zoom < 0 || d.zoom > 24) {
    errors.push('zoom out of range [0,24]')
  }

  const a = d.aoi
  if (a != null) {
    if (!num(a.min_lat) || !num(a.min_lon) || !num(a.max_lat) || !num(a.max_lon) ||
        a.min_lat < -90 || a.max_lat > 90 || a.min_lon < -180 || a.max_lon > 180) {
      errors.push('aoi out of range')
    } else if (a.min_lat > a.max_lat || a.min_lon > a.max_lon) {
      errors.push('aoi min must be <= max')
    }
  }

  if (d.fl_min != null && d.fl_min < 0) errors.push('fl_min must be >= 0')
  if (d.fl_max != null && d.fl_max < 0) errors.push('fl_max must be >= 0')
  if (d.fl_min != null && d.fl_max != null && d.fl_min > d.fl_max) {
    errors.push('fl_min must be <= fl_max')
  }

  return errors
}
