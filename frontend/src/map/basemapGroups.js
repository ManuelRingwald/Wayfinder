// E0 (#291, part of epic #290): bucket the BKG base-map style layers into
// ELEMENT GROUPS so the sidebar can later toggle "only rivers" / "only roads"
// (E2/#293). This is the schema-agnostic DATA BASIS — no UI.
//
// basemap.de's exact `source-layer` names DRIFT with upstream style updates
// (and the basemap.world style uses different, OSM-derived names), so the
// mapping is PATTERN-based over each layer's source-layer + id + type — never a
// hard-coded name list. Every layer lands in exactly one group; anything we do
// not recognise falls into 'other', so a layer is NEVER silently dropped (the
// #274 base-map master keeps switching the full set regardless of grouping).

// The element groups, in a stable display order. 'other' is the catch-all for
// layers no rule claims (kept visible with the rest under the base-map master).
export const BASEMAP_GROUPS = [
  'water',
  'traffic',
  'vegetation',
  'settlement',
  'building',
  'boundary',
  'label',
  'background',
  'other',
]

// E2 (#293): the element groups the sidebar exposes as individual switches, with
// German labels. 'other' is deliberately NOT here — it is the unclassified
// catch-all and simply follows the base-map master (a switch over unknown layers
// would be unpredictable). The rest map 1:1 to BASEMAP_GROUPS.
export const BASEMAP_ELEMENTS = [
  { id: 'water', label: 'Gewässer' },
  { id: 'traffic', label: 'Verkehr' },
  { id: 'vegetation', label: 'Vegetation' },
  { id: 'settlement', label: 'Siedlung' },
  { id: 'building', label: 'Gebäude' },
  { id: 'boundary', label: 'Grenzen' },
  { id: 'label', label: 'Beschriftung' },
  { id: 'background', label: 'Hintergrund' },
]

// E3 (#294): one-click element presets, so the operator rarely touches the eight
// switches individually. Each preset names a full element set; every BASEMAP_
// ELEMENTS id MUST be listed (a preset test guards this) so applying one is
// deterministic. Minimal = orientation only (coast/borders/labels on the bare
// scope); Standard = a clean operational map (+ roads + backdrop); Detailliert =
// everything. Anything not matching a preset is "Benutzerdefiniert".
export const BASEMAP_PRESETS = [
  {
    id: 'minimal',
    label: 'Minimal',
    elements: { water: true, traffic: false, vegetation: false, settlement: false, building: false, boundary: true, label: true, background: false },
  },
  {
    id: 'standard',
    label: 'Standard',
    elements: { water: true, traffic: true, vegetation: false, settlement: false, building: false, boundary: true, label: true, background: true },
  },
  {
    id: 'detail',
    label: 'Detailliert',
    elements: { water: true, traffic: true, vegetation: true, settlement: true, building: true, boundary: true, label: true, background: true },
  },
]

/**
 * matchPreset returns the id of the preset whose element set equals `current`,
 * or null ("Benutzerdefiniert") when none matches. Compared over BASEMAP_ELEMENTS
 * so extra/missing keys never falsely match.
 * @param {Record<string, boolean>} current basemapElementVisibility
 * @returns {string|null}
 */
export function matchPreset(current) {
  if (!current) return null
  for (const p of BASEMAP_PRESETS) {
    if (BASEMAP_ELEMENTS.every((e) => !!current[e.id] === !!p.elements[e.id])) return p.id
  }
  return null
}

// Priority-ordered classification rules: FIRST match wins. More specific groups
// precede more generic ones so an overlapping name is classified by its
// strongest signal — e.g. a forest/park "…landuse…" hits `vegetation` before the
// generic `settlement` landuse rule; a building before the settlement bucket.
// Each test receives (haystack, type): haystack = "<source-layer> <id>" lowercased.
const RULES = [
  // Labels / text FIRST: a symbol layer IS "Beschriftung" (place/road/water
  // names, POI text) whatever theme it sits over — so it must be claimed before
  // the theme rules below, or e.g. a "water_name" symbol would be miscounted as
  // water geometry. Explicit caption stems cover any non-symbol label layer.
  ['label', (h, t) => t === 'symbol' || /(label|beschriftung|\bname\b|\btext\b|\bpoi\b|\bplace\b|\bort\b|caption|schrift)/.test(h)],
  // Backdrop / relief: the base fill + terrain shading (also the map's own
  // background layer, matched by type).
  ['background', (h, t) => t === 'background' || /(hintergrund|background|relief|hillshade|shading|terrain|\bdem\b|contour|hoehenlinie|höhenlinie)/.test(h)],
  // Administrative boundaries (basemap.de: "Verwaltungseinheit_*"/"…grenze").
  ['boundary', (h) => /(grenze|boundary|border|\badmin|verwaltung)/.test(h)],
  // Buildings ("Gebaeude"/building). Before settlement so a building footprint
  // is not swallowed by the generic land-use bucket.
  ['building', (h) => /(gebaeude|gebäude|building|bauwerk)/.test(h)],
  // Water: areas (lakes/sea) + lines (rivers/streams/canals). "gewaesser" is the
  // basemap.de stem; the OSM stems (water/river/…) cover basemap.world. Plain
  // "see" is deliberately omitted (it is a substring of "chaussee", a road).
  ['water', (h) => /(wasser|gewaesser|gewässer|water|hydro|ocean|\bsea\b|lake|river|stream|waterway|fluss|bach|kanal|canal|teich|weiher)/.test(h)],
  // Traffic: roads (by class), rail, ways ("Verkehrsflaeche/-linie", road/rail/…).
  ['traffic', (h) => /(strasse|straße|\broad|street|highway|motorway|verkehr|bahn|\brail|transport|\bweg|pfad|\bpath|bruecke|brücke|bridge|tunnel|runway|aeroway|autobahn)/.test(h)],
  // Vegetation / natural cover (forest/park/grass/farmland). Before settlement so
  // green land-use is not mislabelled built-up.
  ['vegetation', (h) => /(vegetation|wald|forest|gruen|grün|\bgreen\b|\bpark\b|\bwood|meadow|grass|scrub|heath|farmland|acker|landwirt|orchard|vineyard|moor)/.test(h)],
  // Settlement / land use (built-up, residential, industrial, "Siedlungsflaeche"/
  // "Nutzung"/landuse) — the generic land bucket after the specific ones above.
  ['settlement', (h) => /(siedl|urban|residential|\bbuilt|industrial|commercial|landuse|landcover|nutzung|ortslage|bebauung|quarter|neighbourhood)/.test(h)],
]

/**
 * classifyBasemapLayer returns the element group for one MapLibre style layer.
 * @param {{id?: string, type?: string, 'source-layer'?: string}} layer
 * @returns {string} one of BASEMAP_GROUPS ('other' when no rule matches)
 */
export function classifyBasemapLayer(layer) {
  if (!layer) return 'other'
  // Normalise separators (underscores, dashes, dots) to spaces so \b word
  // boundaries work on multi-part ids like "landcover_wood" — '_' is a \w char,
  // so without this "\bwood" would not match. Within-token substrings still hit.
  const haystack = `${layer['source-layer'] || ''} ${layer.id || ''}`
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, ' ')
  const type = layer.type
  for (const [group, test] of RULES) {
    if (test(haystack, type)) return group
  }
  return 'other'
}

/**
 * bucketBasemapLayers groups a style's layers by element group, returning
 * { group: layerId[] } for every group in BASEMAP_GROUPS (empty arrays kept, so
 * callers can iterate a stable shape). The synthetic scope-floor layer is
 * excluded — it is not part of the official base map.
 *
 * @param {Array<{id?: string, type?: string, 'source-layer'?: string}>} styleLayers
 * @param {string} [excludeId] a layer id to skip (the synthetic background floor)
 * @returns {Record<string, string[]>}
 */
export function bucketBasemapLayers(styleLayers, excludeId) {
  const groups = {}
  for (const g of BASEMAP_GROUPS) groups[g] = []
  for (const l of styleLayers || []) {
    if (!l || !l.id || l.id === excludeId) continue
    groups[classifyBasemapLayer(l)].push(l.id)
  }
  return groups
}
