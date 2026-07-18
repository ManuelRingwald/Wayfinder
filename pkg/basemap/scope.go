// Radar-scope dark transform (ADR 0026 Nachtrag / H2, FR-UI-031).
//
// The "bkg-dark" theme derives a dark base map from the SAME official
// basemap.de vector tiles as the bright "bkg" theme — not from a hand-written
// style. A hand-authored dark style would need the (large, drifting) BKG tile
// schema; instead the fetched upstream style is recoloured RULE-BASED in HSL
// space, which is schema-agnostic and survives upstream style updates:
//
//   - fills/lines (roleBase): lightness is inverted into a near-black band and
//     saturation collapsed — bright land becomes the scope backdrop, dark
//     road/boundary strokes become faint light-grey structure.
//   - map text (roleText): pushed into a muted light band so place names stay
//     readable on the dark base without competing with track labels.
//   - halos (roleHalo): pinned near-black, matching the backdrop.
//   - symbol icons (road shields …): dimmed via icon-opacity so coloured
//     signage does not glow on the scope.
//
// The transform touches only colour VALUES (strings that parse as CSS colours,
// recursively inside expressions/stops); layer structure, filters and zoom
// behaviour of the official style are preserved. Alpha is preserved.
package basemap

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Lightness/saturation bands of the scope look. Base band starts at the
// near-black backdrop (#070b12-ish, ADR 0015 design tokens) and stays low so
// geographic context reads as faint structure, the way a controller's scope
// backdrop does; text sits clearly above it but below the track-label white.
const (
	baseLightMin = 0.035
	baseLightMax = 0.38
	baseSatScale = 0.35

	textLightMin = 0.52
	textLightMax = 0.72
	textSatScale = 0.30

	haloLight    = 0.045
	haloSatScale = 0.30

	iconDimOpacity = 0.35
)

// colourRole selects the transform band for a property's colours.
type colourRole int

const (
	roleBase colourRole = iota
	roleText
	roleHalo
)

// darkenStyle applies the scope transform to a decoded style in place.
func darkenStyle(style map[string]any) {
	layers, _ := style["layers"].([]any)
	for _, l := range layers {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		isSymbol := lm["type"] == "symbol"
		for _, section := range []string{"paint", "layout"} {
			m, ok := lm[section].(map[string]any)
			if !ok {
				continue
			}
			for prop, v := range m {
				m[prop] = transformColours(v, roleForProperty(prop, isSymbol))
			}
		}
		if isSymbol {
			dimSymbolIcons(lm)
		}
	}
}

// roleForProperty maps a style property name to its colour role. Only symbol
// layers carry map text; their halos get the backdrop colour so glyph edges
// blend into the scope instead of glowing.
func roleForProperty(prop string, isSymbol bool) colourRole {
	if strings.Contains(prop, "halo") {
		return roleHalo
	}
	if isSymbol && strings.Contains(prop, "text-color") {
		return roleText
	}
	return roleBase
}

// dimSymbolIcons dims a symbol layer's icons (road shields, POI sprites). An
// existing numeric icon-opacity is scaled; expressions are left untouched
// (never guess inside upstream logic); absent opacity gets the dim default.
func dimSymbolIcons(layer map[string]any) {
	paint, ok := layer["paint"].(map[string]any)
	if !ok {
		paint = map[string]any{}
		layer["paint"] = paint
	}
	switch v := paint["icon-opacity"].(type) {
	case nil:
		paint["icon-opacity"] = iconDimOpacity
	case float64:
		paint["icon-opacity"] = v * iconDimOpacity
	}
}

// transformColours walks any style value (plain colour string, expression
// array, legacy stops object) and converts every parseable colour string.
// Non-colour values pass through unchanged.
func transformColours(v any, role colourRole) any {
	switch t := v.(type) {
	case string:
		if h, s, l, a, ok := parseCSSColour(t); ok {
			return formatColour(scopeColour(h, s, l, role), a)
		}
		return v
	case []any:
		for i, e := range t {
			t[i] = transformColours(e, role)
		}
		return t
	case map[string]any:
		for k, e := range t {
			t[k] = transformColours(e, role)
		}
		return t
	default:
		return v
	}
}

// hsl bundles a transformed colour (alpha is carried separately).
type hsl struct{ h, s, l float64 }

// scopeColour maps an upstream colour into the role's scope band. Lightness is
// INVERTED for the base band (bright land → darkest, dark strokes → the light
// end of the dark band) and for text (dark ink → bright end of the muted band),
// preserving the upstream's relative contrast ordering.
func scopeColour(h, s, l float64, role colourRole) hsl {
	switch role {
	case roleText:
		return hsl{h, s * textSatScale, textLightMin + (textLightMax-textLightMin)*(1-l)}
	case roleHalo:
		return hsl{h, s * haloSatScale, haloLight}
	default:
		return hsl{h, s * baseSatScale, baseLightMin + (baseLightMax-baseLightMin)*(1-l)}
	}
}

// parseCSSColour parses the colour syntaxes that occur in MapLibre styles:
// #rgb/#rgba/#rrggbb/#rrggbbaa, rgb()/rgba(), hsl()/hsla(). Returns HSL + alpha.
// Anything else (named colours, expressions, plain strings) reports !ok and is
// left untouched — better an occasional original colour than a mangled style.
func parseCSSColour(s string) (h, sat, l, a float64, ok bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch {
	case strings.HasPrefix(s, "#"):
		return parseHex(s)
	case strings.HasPrefix(s, "rgb"):
		nums, alpha, okN := parseFuncArgs(s)
		if !okN || len(nums) != 3 {
			return 0, 0, 0, 0, false
		}
		h, sat, l = rgbToHSL(nums[0]/255, nums[1]/255, nums[2]/255)
		return h, sat, l, alpha, true
	case strings.HasPrefix(s, "hsl"):
		nums, alpha, okN := parseFuncArgs(s)
		if !okN || len(nums) != 3 {
			return 0, 0, 0, 0, false
		}
		return nums[0], nums[1] / 100, nums[2] / 100, alpha, true
	}
	return 0, 0, 0, 0, false
}

// parseHex handles 3/4/6/8-digit hex colours.
func parseHex(s string) (h, sat, l, a float64, ok bool) {
	hex := s[1:]
	var r, g, b uint64
	a = 1
	var err error
	switch len(hex) {
	case 3, 4:
		r, err = strconv.ParseUint(strings.Repeat(hex[0:1], 2), 16, 8)
		if err == nil {
			g, err = strconv.ParseUint(strings.Repeat(hex[1:2], 2), 16, 8)
		}
		if err == nil {
			b, err = strconv.ParseUint(strings.Repeat(hex[2:3], 2), 16, 8)
		}
		if err == nil && len(hex) == 4 {
			var av uint64
			av, err = strconv.ParseUint(strings.Repeat(hex[3:4], 2), 16, 8)
			a = float64(av) / 255
		}
	case 6, 8:
		r, err = strconv.ParseUint(hex[0:2], 16, 8)
		if err == nil {
			g, err = strconv.ParseUint(hex[2:4], 16, 8)
		}
		if err == nil {
			b, err = strconv.ParseUint(hex[4:6], 16, 8)
		}
		if err == nil && len(hex) == 8 {
			var av uint64
			av, err = strconv.ParseUint(hex[6:8], 16, 8)
			a = float64(av) / 255
		}
	default:
		return 0, 0, 0, 0, false
	}
	if err != nil {
		return 0, 0, 0, 0, false
	}
	h, sat, l = rgbToHSL(float64(r)/255, float64(g)/255, float64(b)/255)
	return h, sat, l, a, true
}

// parseFuncArgs extracts the numeric arguments of rgb()/rgba()/hsl()/hsla().
// Percent signs and the "deg" unit are tolerated; alpha defaults to 1.
func parseFuncArgs(s string) (nums []float64, alpha float64, ok bool) {
	open := strings.IndexByte(s, '(')
	close := strings.LastIndexByte(s, ')')
	if open < 0 || close < open {
		return nil, 0, false
	}
	parts := strings.FieldsFunc(s[open+1:close], func(r rune) bool { return r == ',' || r == '/' || r == ' ' })
	alpha = 1
	hasAlpha := strings.HasPrefix(s, "rgba") || strings.HasPrefix(s, "hsla")
	for i, p := range parts {
		p = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(p), "%"), "deg")
		f, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, 0, false
		}
		if hasAlpha && i == 3 {
			alpha = f
			continue
		}
		if i >= 4 {
			return nil, 0, false
		}
		if i == 3 { // rgb()/hsl() modern syntax with slash-alpha
			alpha = f
			continue
		}
		nums = append(nums, f)
	}
	if len(nums) != 3 {
		return nil, 0, false
	}
	return nums, alpha, true
}

// rgbToHSL / hslToRGB implement the standard CSS colour-space conversion.
func rgbToHSL(r, g, b float64) (h, s, l float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2
	d := max - min
	if d == 0 {
		return 0, 0, l
	}
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}

func hslToRGB(h, s, l float64) (r, g, b float64) {
	c := (1 - math.Abs(2*l-1)) * s
	hh := math.Mod(h, 360) / 60
	x := c * (1 - math.Abs(math.Mod(hh, 2)-1))
	switch {
	case hh < 1:
		r, g, b = c, x, 0
	case hh < 2:
		r, g, b = x, c, 0
	case hh < 3:
		r, g, b = 0, c, x
	case hh < 4:
		r, g, b = 0, x, c
	case hh < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	m := l - c/2
	return r + m, g + m, b + m
}

// formatColour renders the transformed colour as #rrggbb, or rgba(…) when the
// original carried alpha < 1 (hex-with-alpha support varies across renderers).
func formatColour(c hsl, a float64) string {
	r, g, b := hslToRGB(c.h, clamp01(c.s), clamp01(c.l))
	ri, gi, bi := int(math.Round(r*255)), int(math.Round(g*255)), int(math.Round(b*255))
	if a >= 1 {
		return fmt.Sprintf("#%02x%02x%02x", ri, gi, bi)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", ri, gi, bi, strconv.FormatFloat(a, 'g', 4, 64))
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}
