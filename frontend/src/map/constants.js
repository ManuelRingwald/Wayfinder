// ASD map layer and source IDs, display constants, and palette definitions.
// These are extracted verbatim from the original app.js so all modules share
// one authoritative source of truth for every magic number.

export const TRACKS_SOURCE_ID = 'tracks'
export const TRACKS_LAYER_ID = 'tracks-points'
export const VECTORS_SOURCE_ID = 'track-vectors'
export const VECTORS_LAYER_ID = 'track-vectors-lines'
export const TRAILS_SOURCE_ID = 'track-trails'
export const TRAILS_LAYER_ID = 'track-trails-lines'
// ASD-004a: individual position-dot layer, rendered above the trail line.
export const HISTORY_DOTS_SOURCE_ID = 'track-history-dots'
export const HISTORY_DOTS_LAYER_ID = 'track-history-dots-circles'

// Aeronautical overlay layers (ASD-003, fed by the OpenAIP backend via
// /api/airspace, /api/navaids, /api/waypoints). They render beneath the track
// layers so tracks always dominate the scope.
export const AIRSPACE_SOURCE_ID = 'airspace'
export const AIRSPACE_FILL_LAYER_ID = 'airspace-fill'
export const AIRSPACE_LINE_LAYER_ID = 'airspace-line'
export const AIRSPACE_LABEL_LAYER_ID = 'airspace-label'
export const NAVAIDS_SOURCE_ID = 'navaids'
export const NAVAIDS_LAYER_ID = 'navaids-symbols'
export const WAYPOINTS_SOURCE_ID = 'waypoints'
export const WAYPOINTS_LAYER_ID = 'waypoints-symbols'

// How often the frontend re-pulls the aeronautical GeoJSON. The backend itself
// refreshes from OpenAIP on the AIRAC-paced interval; this only needs to be
// frequent enough to pick up a backend cache update, not to hit OpenAIP.
export const AERO_REFRESH_MS = 5 * 60 * 1000

// Speed-vector look-ahead: how many seconds of travel the vector line
// represents (standard ASD-style speed vector line, SVL).
export const VECTOR_LOOKAHEAD_S = 60

// Maximum number of past positions kept per track for the trail display.
export const TRAIL_MAX_POINTS = 20

// Mean Earth radius (m), used for the local meters-to-degrees conversion of
// the vector endpoint. Sufficient accuracy for display purposes.
export const EARTH_RADIUS_M = 6371000

// ASD-004c: duration of the TSE graceful fade-out animation in milliseconds.
export const FADE_DURATION_MS = 1500

// ASD-002: Anti-Garbling — separate GeoJSON sources for deconflicted labels
// and leader lines (lines from symbol to data-block anchor).
export const LABELS_SOURCE_ID = 'track-labels'
export const LABELS_LAYER_ID = 'track-labels-text'
export const LEADER_LINES_SOURCE_ID = 'track-leader-lines'
export const LEADER_LINES_LAYER_ID = 'track-leader-lines-lines'

// ASD-002: Deconfliction geometry constants (all values in screen pixels).
// LABEL_TEXT_SIZE      : data-block text size; used as the symbol layer's "text-size".
// LABEL_SLOT_RADIUS_PX : distance from symbol centre to label anchor candidate.
// LABEL_W/H_PX         : conservative bounding box for a 3-line data block at text-size 11.
// SYMBOL_BBOX_R_PX     : symbol footprint reserved so OTHER tracks' labels avoid this dot.
// LEADER_THRESHOLD_PX  : minimum symbol→label distance before a leader line is drawn.
export const LABEL_TEXT_SIZE = 11
export const LABEL_SLOT_RADIUS_PX = 20
export const LABEL_W_PX = 62
export const LABEL_H_PX = 46
export const SYMBOL_BBOX_R_PX = 8
export const LEADER_THRESHOLD_PX = 10

// ASD-002: Eight candidate placement slots as normalised screen-space direction
// vectors, ordered right-first following ATC scope convention. Each vector is
// scaled by LABEL_SLOT_RADIUS_PX to get the candidate label centre in pixels.
export const LABEL_SLOTS = [
  [ 1.2,  0.3],  // right (ATC default)
  [ 0,    1.4],  // below
  [-1.2,  0.3],  // left
  [ 0,   -1.4],  // above
  [ 1.2, -0.5],  // upper-right
  [-1.2, -0.5],  // upper-left
  [ 1.2,  1.0],  // lower-right
  [-1.2,  1.0],  // lower-left
]

// Maximum number of track history points kept. Alias kept for test
// compatibility with different naming conventions seen in the codebase.
export const MAX_HISTORY_PTS = TRAIL_MAX_POINTS
export const HISTORY_MAX_PTS = TRAIL_MAX_POINTS

// Foreground palettes per base-map theme (ASD-003 Häppchen 3a). On the dark
// "Radar Dark Mode" base, labels are light with a dark halo so they stay
// legible; on the bright OSM base the original dark-on-white palette is used.
// Track-status colours (confirmed/coasting/tentative) read well on both bases.
export const PALETTES = {
  dark: {
    label: '#e8eef5',
    labelHalo: '#000000',
    vector: '#cfd8dc',
    trail: '#607d8b',
    symbolStroke: '#000000',
    airspaceLine: '#5b8fd6',
    airspaceText: '#9fc0e8',
    aeroHalo: '#000000',
  },
  osm: {
    label: '#212121',
    labelHalo: '#ffffff',
    vector: '#212121',
    trail: '#90a4ae',
    symbolStroke: '#000000',
    airspaceLine: '#1f4ea8',
    airspaceText: '#22305a',
    aeroHalo: '#ffffff',
  },
}
