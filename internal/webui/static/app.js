// Wayfinder ASD frontend. Loads the configured map style and centers the
// view on the configured position (see /api/map-config, served by the
// Wayfinder backend).

async function main() {
  const res = await fetch("/api/map-config");
  const cfg = await res.json();

  new maplibregl.Map({
    container: "map",
    style: cfg.style,
    center: [cfg.center_lon, cfg.center_lat],
    zoom: cfg.zoom,
  });
}

main();
