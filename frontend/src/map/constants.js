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

// ASD-007: selection halo — a cyan ring around the currently selected track
// (design template symbolNode: r=11, stroke primary). One GeoJSON source holds
// at most one Point (the selected track's live position); the layer draws the
// ring under the symbols so the symbol stays crisp on top.
export const SELECTION_SOURCE_ID = 'track-selection'
export const SELECTION_LAYER_ID = 'track-selection-ring'

// Paket 6: Sensor coverage ring overlay — radar range circles fetched from
// /api/coverage/rings as a static GeoJSON FeatureCollection.
export const COVERAGE_SOURCE_ID = 'coverage-rings'
export const COVERAGE_RINGS_LAYER_ID = 'coverage-rings-lines'
export const COVERAGE_CENTER_LAYER_ID = 'coverage-center-circles'

// ASD-012: Range-ring overlay — concentric constant-ground-distance circles
// around the configured display centre, operator-tunable live via the sidebar
// (distinct from the Paket-6 sensor coverage rings). Spacing/count live as
// reactive store state (stores/asd.js); these are only the defaults + choices.
export const RANGE_RINGS_SOURCE_ID = 'range-rings'
export const RANGE_RINGS_LAYER_ID = 'range-rings-lines'
export const RANGE_RINGS_LABEL_LAYER_ID = 'range-rings-labels'
export const RANGE_RING_SPACING_OPTIONS_NM = [5, 10, 15]
export const DEFAULT_RANGE_RING_SPACING_NM = 10
export const DEFAULT_RANGE_RING_COUNT = 5
export const MAX_RANGE_RING_COUNT = 10

// WX-A (ADR 0016): DWD weather-radar overlay. A MapLibre raster source whose
// tiles are proxied by Wayfinder (/api/weather/radar/{z}/{x}/{y}.png → DWD WMS in
// EPSG:3857). The overlay sits above the base map but below the aeronautical and
// track layers; opacity keeps the air picture readable through it.
export const WEATHER_RADAR_SOURCE_ID = 'weather-radar'
export const WEATHER_RADAR_LAYER_ID = 'weather-radar-raster'
export const WEATHER_RADAR_TILES_URL = '/api/weather/radar/{z}/{x}/{y}.png'
export const WEATHER_RADAR_OPACITY = 0.6
// GeoNutzV / CC BY 4.0 requires the DWD source note on any DWD-derived layer.
export const DWD_ATTRIBUTION = '© Deutscher Wetterdienst'

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
// Symbol footprint reserved so OTHER tracks' labels avoid this dot. Matches the
// design template's deconfliction half-extent (symPad=9) and the enlarged track
// symbols (up to 12 CSS px diameter after the ASD-007 resize).
export const SYMBOL_BBOX_R_PX = 9
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

// ASD-011: Airspace type groups for the category filter. OpenAIP encodes
// airspace type as a numeric enum; these groups map the enum values to
// operationally meaningful categories with distinct display colours.
// lineWidth and fillOpacity drive the per-group MapLibre paint expressions.
export const AIRSPACE_GROUPS = [
  {
    id: 'ctr',
    label: 'Kontrollzonen (CTR)',
    color: '#e040fb',      // magenta — safety-critical, around airports
    types: [4, 13],        // 4=CTR, 13=ATZ
    lineWidth: 1.5,
    fillOpacity: 0.10,
  },
  {
    id: 'tma',
    label: 'TMA / CTA',
    color: '#448aff',      // blue — controlled upper airspace
    types: [7, 26],        // 7=TMA, 26=CTA
    lineWidth: 1.0,
    fillOpacity: 0.06,
  },
  {
    id: 'restricted',
    label: 'Beschränkungsgebiete',
    color: '#ff6d00',      // orange — restricted/danger/prohibited
    types: [1, 2, 3],      // 1=Restricted, 2=Danger, 3=Prohibited
    lineWidth: 1.5,
    fillOpacity: 0.12,
  },
  {
    id: 'info',
    label: 'FIS / RMZ / TMZ',
    color: '#607d8b',      // blue-grey — advisory, often dimmable
    types: [5, 6, 10, 30], // 5=TMZ, 6=RMZ, 10=FIR, 30=FIZ
    lineWidth: 0.8,
    fillOpacity: 0.04,
  },
]

// ASD-007: Track symbol colours by ICAO target type. These are independent of
// the base-map palette — they must remain distinguishable on both dark and OSM
// bases. In the current demo Firefly only emits civil tracks (friendlyCivil);
// the remaining colours are reserved for future IFF/Mode-3A differentiation.
// Full rationale and hex values documented in docs/design/color-tokens.md §3.1.
export const TRACK_COLORS = {
  friendlyCivil:    '#41c4e8', // cyan  — civil confirmed track
  friendlyMilitary: '#ffa726', // amber — military confirmed track
  hostile:          '#ff4338', // red   — hostile / ordnance (= error colour)
  unknown:          '#ffd23e', // gold  — not yet correlated
  neutral:          '#43c66b', // green — neutral track
}

// WF2-40: Track state colours. These were inlined in the track layer's
// circle-color expression; factored out so the provenance symbol icons
// (which bake the state colour in at draw time) and any legend share one
// source of truth. Precedence matches the old expression:
// filtered > coasting > confirmed > tentative.
export const TRACK_STATE_COLORS = {
  filtered:  '#455a64', // blue-grey: outside FL filter range (ASD-005)
  coasting:  '#ff9800', // orange: no recent update
  confirmed: '#4caf50', // green: confirmed track
  tentative: '#9e9e9e', // grey: tentative track
}

// Foreground palettes per base-map theme (ASD-003 Häppchen 3a). On the dark
// "Radar Dark Mode" base, labels are light with a dark halo so they stay
// legible; on the bright OSM base the original dark-on-white palette is used.
// ASD-007: updated to align with docs/design/color-tokens.md §3.2 and §3.3.
export const PALETTES = {
  dark: {
    label: '#dce6f0',        // = on-surface token
    labelHalo: '#000000',
    vector: '#9ec8de',       // speed-vector line (SVL)
    trail: '#3a5a72',        // history trail (subdued, no map distraction)
    symbolStroke: '#000000',
    airspaceFillColor: '#3a6fb0', // used with opacity 0.12 in layers.js
    airspaceLine: '#5b8fd6',
    airspaceText: '#9fc0e8',
    airways: '#2a8fa8',
    aeroHalo: '#000000',
    rangeRing: '#4a7d96', // ASD-012: subdued cyan-grey distance grid
    selection: '#23d3e6', // ASD-007: selection halo — cyan primary
  },
  osm: {
    label: '#212121',
    labelHalo: '#ffffff',
    vector: '#212121',
    trail: '#90a4ae',
    symbolStroke: '#000000',
    airspaceFillColor: '#1f4ea8',
    airspaceLine: '#1f4ea8',
    airspaceText: '#22305a',
    airways: '#1a6a7a',
    aeroHalo: '#ffffff',
    rangeRing: '#3d6b82', // ASD-012: distance grid, readable on the bright base
    selection: '#0097a7', // ASD-007: selection halo — deeper cyan for the bright base
  },
}
