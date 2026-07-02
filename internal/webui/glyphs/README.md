# Self-hosted MapLibre glyphs (FR-UI-023, ADR 0015 Nachtrag-2)

These SDF glyph PBFs let the ASD scope render its data blocks (and all scope
text) in **Roboto Mono** — the design template's data-block face — without any
runtime font CDN. They are embedded into the Go binary (`//go:embed glyphs` in
`internal/webui/webui.go`) and served at `/glyphs/{fontstack}/{range}.pbf`
(`webui.GlyphsHandler`). Both built-in map styles point their `"glyphs"` at that
endpoint (`cmd/wayfinder/main.go`).

## Layout

```
Roboto Mono Medium/<start>-<end>.pbf   # one file per 256-codepoint range
```

Ranges `0-1023` are vendored (Basic Latin + Latin-1 Supplement + Latin
Extended-A/B) — enough for ASCII, German umlauts, the degree sign, and common
European navaid/place-name diacritics. A code point outside the vendored ranges
returns 404 and MapLibre renders it blank; extend the range list below if needed.

## Regenerating (one-time dev step, not a build dependency)

The committed PBFs are the source of truth for the build. To regenerate them
(e.g. to add ranges or bump the font), use [`fontnik`](https://github.com/mapbox/node-fontnik):

```bash
# 1. Fetch the Roboto Mono Medium TTF (Apache-2.0):
curl -sSL -o RobotoMono-Medium.ttf \
  https://raw.githubusercontent.com/googlefonts/RobotoMono/main/fonts/ttf/RobotoMono-Medium.ttf

# 2. Generate the SDF ranges:
npm install fontnik
node -e '
  const fontnik = require("fontnik"), fs = require("fs");
  const font = fs.readFileSync("RobotoMono-Medium.ttf");
  const out = "internal/webui/glyphs/Roboto Mono Medium";
  fs.mkdirSync(out, { recursive: true });
  for (const [s, e] of [[0,255],[256,511],[512,767],[768,1023]])
    fontnik.range({ font, start: s, end: e }, (err, d) => {
      if (err) throw err;
      fs.writeFileSync(`${out}/${s}-${e}.pbf`, d);
    });
'
```

The fontstack **directory name** (`Roboto Mono Medium`) must match the
`text-font` value used by the map layers (`frontend/src/map/layers.js`).
