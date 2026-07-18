package basemap

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
)

// mustHSL parses a colour or fails the test.
func mustHSL(t *testing.T, c string) (h, s, l, a float64) {
	t.Helper()
	h, s, l, a, ok := parseCSSColour(c)
	if !ok {
		t.Fatalf("parseCSSColour(%q) failed", c)
	}
	return h, s, l, a
}

func TestParseCSSColourForms(t *testing.T) {
	for _, tc := range []struct {
		in      string
		wantL   float64
		wantA   float64
		wantErr bool
	}{
		{"#fff", 1, 1, false},
		{"#000000", 0, 1, false},
		{"#80808080", 0.502, 0.502, false},
		{"rgb(255, 255, 255)", 1, 1, false},
		{"rgba(0,0,0,0.5)", 0, 0.5, false},
		{"hsl(210, 50%, 40%)", 0.4, 1, false},
		{"hsla(210,50%,40%,0.25)", 0.4, 0.25, false},
		{"tomato", 0, 0, true},       // named colours are left untouched
		{"{water}", 0, 0, true},      // template strings untouched
		{"rgb(1,2)", 0, 0, true},     // wrong arity
		{"#12345", 0, 0, true},       // bad hex length
		{"hsl(a,b%,c%)", 0, 0, true}, // non-numeric
		{"interpolate", 0, 0, true},  // expression keyword
	} {
		_, _, l, a, ok := parseCSSColour(tc.in)
		if tc.wantErr {
			if ok {
				t.Errorf("%q: expected parse failure", tc.in)
			}
			continue
		}
		if !ok {
			t.Errorf("%q: unexpected parse failure", tc.in)
			continue
		}
		if diff := l - tc.wantL; diff > 0.01 || diff < -0.01 {
			t.Errorf("%q: lightness %v, want ~%v", tc.in, l, tc.wantL)
		}
		if diff := a - tc.wantA; diff > 0.01 || diff < -0.01 {
			t.Errorf("%q: alpha %v, want ~%v", tc.in, a, tc.wantA)
		}
	}
}

// TestScopeColourBands: bright fills land near-black, dark strokes become
// faint light structure (inversion preserves contrast ordering), text sits in
// the muted light band, halos pin to the backdrop.
func TestScopeColourBands(t *testing.T) {
	brightLand := transformColours("#f4f2ec", roleBase).(string) // typical land fill
	darkStroke := transformColours("#333333", roleBase).(string) // typical boundary ink
	_, _, lLand, _ := mustHSL(t, brightLand)
	_, _, lStroke, _ := mustHSL(t, darkStroke)
	if lLand > 0.10 {
		t.Errorf("bright land → L=%v, want near-black (≤0.10): %s", lLand, brightLand)
	}
	if lStroke <= lLand {
		t.Errorf("contrast inversion lost: stroke L=%v not above land L=%v", lStroke, lLand)
	}
	if lStroke > baseLightMax+0.01 {
		t.Errorf("stroke L=%v exceeds the base band cap %v", lStroke, baseLightMax)
	}

	text := transformColours("#222222", roleText).(string)
	_, _, lText, _ := mustHSL(t, text)
	if lText < textLightMin || lText > textLightMax+0.01 {
		t.Errorf("text L=%v outside muted band [%v..%v]: %s", lText, textLightMin, textLightMax, text)
	}

	halo := transformColours("#ffffff", roleHalo).(string)
	_, _, lHalo, _ := mustHSL(t, halo)
	if lHalo > 0.07 {
		t.Errorf("halo L=%v, want backdrop-dark: %s", lHalo, halo)
	}
}

// TestTransformColoursRecursesAndPreservesAlpha: colours nested in
// interpolate-expressions and legacy stops are converted; alpha survives;
// non-colour values pass through untouched.
func TestTransformColoursRecursesAndPreservesAlpha(t *testing.T) {
	expr := []any{"interpolate", []any{"linear"}, []any{"zoom"}, 5.0, "rgba(240,240,240,0.8)", 10.0, "#000"}
	out := transformColours(expr, roleBase).([]any)
	if out[0] != "interpolate" || out[3] != 5.0 {
		t.Errorf("non-colour expression parts changed: %v", out)
	}
	c1, ok := out[4].(string)
	if !ok || !strings.HasPrefix(c1, "rgba(") || !strings.HasSuffix(c1, ",0.8)") {
		t.Errorf("alpha not preserved through transform: %v", out[4])
	}
	if out[5] != 10.0 {
		t.Errorf("zoom stop changed: %v", out[5])
	}
	stops := map[string]any{"stops": []any{[]any{8.0, "#ffffff"}}}
	sOut := transformColours(stops, roleBase).(map[string]any)
	stop := sOut["stops"].([]any)[0].([]any)
	if stop[0] != 8.0 {
		t.Errorf("stop zoom changed: %v", stop)
	}
	if c, _ := stop[1].(string); c == "#ffffff" {
		t.Errorf("stop colour not transformed")
	}
}

// TestDarkenStyleEndToEnd runs the transform over a realistic layer mix.
func TestDarkenStyleEndToEnd(t *testing.T) {
	raw := `{
		"version": 8,
		"layers": [
			{"id": "bg", "type": "background", "paint": {"background-color": "#f4f2ec"}},
			{"id": "water", "type": "fill", "paint": {"fill-color": "rgb(190, 220, 240)", "fill-opacity": 0.9}},
			{"id": "road", "type": "line", "paint": {"line-color": "#ffffff", "line-width": 2}},
			{"id": "place", "type": "symbol",
			 "layout": {"text-field": "{name}", "text-font": ["BM Web Regular"]},
			 "paint": {"text-color": "#222222", "text-halo-color": "#ffffff", "icon-opacity": 0.8}},
			{"id": "shield", "type": "symbol", "layout": {"icon-image": "shield"}}
		]
	}`
	var style map[string]any
	if err := json.Unmarshal([]byte(raw), &style); err != nil {
		t.Fatal(err)
	}
	darkenStyle(style)

	layers := style["layers"].([]any)
	bg := layers[0].(map[string]any)["paint"].(map[string]any)["background-color"].(string)
	_, _, lBG, _ := mustHSL(t, bg)
	if lBG > 0.10 {
		t.Errorf("background L=%v, want near-black: %s", lBG, bg)
	}

	place := layers[3].(map[string]any)["paint"].(map[string]any)
	_, _, lText, _ := mustHSL(t, place["text-color"].(string))
	if lText < textLightMin {
		t.Errorf("place text not lifted into muted band: %v", place["text-color"])
	}
	_, _, lHalo, _ := mustHSL(t, place["text-halo-color"].(string))
	if lHalo > 0.07 {
		t.Errorf("halo not backdrop-dark: %v", place["text-halo-color"])
	}
	if got := place["icon-opacity"].(float64); got > 0.8*iconDimOpacity+0.001 {
		t.Errorf("existing icon-opacity not dimmed: %v", got)
	}

	// text-field/fonts/widths/opacities untouched
	layout := layers[3].(map[string]any)["layout"].(map[string]any)
	if layout["text-field"] != "{name}" {
		t.Errorf("text-field changed: %v", layout["text-field"])
	}
	road := layers[2].(map[string]any)["paint"].(map[string]any)
	if road["line-width"] != 2.0 {
		t.Errorf("line-width changed: %v", road["line-width"])
	}
	water := layers[1].(map[string]any)["paint"].(map[string]any)
	if water["fill-opacity"] != 0.9 {
		t.Errorf("fill-opacity changed: %v", water["fill-opacity"])
	}

	// icon-only symbol layer without paint gets the dim default
	shield := layers[4].(map[string]any)["paint"].(map[string]any)
	if shield["icon-opacity"] != iconDimOpacity {
		t.Errorf("shield icon-opacity = %v, want %v", shield["icon-opacity"], iconDimOpacity)
	}
}

// TestDarkVariantThroughService: the Dark config flag applies the transform in
// the full fetch→rewrite→serve path and keeps the glyph rewrite intact.
func TestDarkVariantThroughService(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := NewService(srv.Client(), Config{StyleURL: srv.URL + "/styles/bm_web_col.json", Dark: true}, nil)

	code, style, _ := getStyle(t, svc)
	if code != 200 {
		t.Fatalf("HTTP %d", code)
	}
	if style["glyphs"] != localGlyphsTemplate {
		t.Errorf("dark variant lost the glyphs rewrite")
	}
}
