// Package basemap serves the official BKG basemap.de base-map style to the
// browser (ADR 0026). The upstream style JSON is fetched SERVER-side, rewritten
// and cached, so the browser only ever talks to Wayfinder for the style and for
// glyphs — the two resources where the naive "point MapLibre at the BKG URL"
// approach breaks Wayfinder:
//
//   - A MapLibre style has exactly ONE "glyphs" URL. Wayfinder's own overlay
//     layers (track labels, aeronautical text) require the self-hosted
//     "Roboto Mono Medium" glyphs (/glyphs, ADR 0015 air-gap decision). The
//     upstream BKG style points "glyphs" at the BKG server, which does not know
//     our fontstack — track labels would silently render blank. The rewrite
//     points "glyphs" back at Wayfinder; the glyph handler then serves embedded
//     local fontstacks itself and proxies unknown (BKG) fontstacks upstream.
//   - Relative URLs inside the upstream style (sprite, tiles) would resolve
//     against Wayfinder's origin once the style is served from /basemap/…, so
//     they are absolutised against the upstream style URL first.
//
// The tile traffic itself stays browser→BKG (public, keyless service); a fully
// self-hosted/air-gapped deployment points WAYFINDER_BKG_STYLE_URL at its own
// mirror and everything follows.
//
// Defensive-consumer rules (CLAUDE.md §7) apply: the upstream is never trusted
// blindly — response sizes are capped, timeouts enforced, browser-supplied path
// segments validated before they touch an upstream URL, and a fetch failure
// serves the last-good style (stale) rather than tearing down the scope.
package basemap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultStyleURL is the public basemap.world Web Vektor "Farbe" style
// (ADR 0026 Nachtrag basemap.world): it draws from TWO tile archives — the
// official basemap.de archive inside Germany plus a BKG-curated world archive
// (OSM/NaturalEarth) outside — so cross-border sectors get surrounding context
// instead of an empty void at the national border. Operators can pin the
// Germany-only style (GermanyOnlyStyleURL), the grey variant (bm_web_gry) or a
// self-hosted mirror via WAYFINDER_BKG_STYLE_URL.
const DefaultStyleURL = "https://sgx.geodatenzentrum.de/gdz_basemapworld_vektor/styles/bm_web_wld_col.json"

// GermanyOnlyStyleURL is the basemap.de-only "Farbe" style (H1 default until
// the basemap.world Nachtrag): strictly official data, ending at the border.
const GermanyOnlyStyleURL = "https://sgx.geodatenzentrum.de/gdz_basemapde_vektor/styles/bm_web_col.json"

// defaultAttribution is injected when the upstream style carries no attribution
// on any source — the basemap.de terms of use require a visible credit.
const defaultAttribution = "© basemap.de / BKG"

// localGlyphsTemplate is Wayfinder's own glyph endpoint, written into the served
// style so ALL fontstacks — BKG base-map fonts and Wayfinder's Roboto Mono —
// resolve against Wayfinder (single glyphs URL per style, see package comment).
const localGlyphsTemplate = "/glyphs/{fontstack}/{range}.pbf"

// maxStyleBytes caps the upstream style JSON read into memory. Real basemap.de
// styles are a few hundred KiB; 8 MiB guards against a hostile/runaway upstream.
const maxStyleBytes = 8 << 20

// maxGlyphBytes caps a single proxied glyph PBF. Real ranges are tens of KiB.
const maxGlyphBytes = 2 << 20

// defaultStyleTTL is how long a fetched style is served before a refetch is
// attempted. basemap.de updates monthly; 12 h keeps a long-running server
// current without hammering the upstream. On refetch failure the stale style
// keeps being served (availability over freshness for a base map).
const defaultStyleTTL = 12 * time.Hour

// maxGlyphCacheEntries bounds the in-memory glyph cache. Glyphs are immutable
// (no TTL); a full cache evicts an arbitrary entry. A style uses a handful of
// fontstacks × ≤256 ranges, so 512 entries comfortably covers real usage.
const maxGlyphCacheEntries = 512

// rangeRe validates the {range}.pbf path segment of a glyph request before it
// is interpolated into the upstream URL (e.g. "0-255.pbf").
var rangeRe = regexp.MustCompile(`^[0-9]{1,7}-[0-9]{1,7}\.pbf$`)

// Config configures the basemap Service.
type Config struct {
	// StyleURL is the upstream MapLibre style JSON. Empty applies DefaultStyleURL.
	StyleURL string
	// TTL is the style cache lifetime. 0 applies defaultStyleTTL.
	TTL time.Duration
	// Dark applies the radar-scope dark transform (scope.go, ADR 0026
	// Nachtrag / H2): the same official tiles, recoloured rule-based into the
	// near-black scope look. Used by the "bkg-dark" theme.
	Dark bool
}

// Service fetches, rewrites and caches the upstream style and proxies unknown
// glyph fontstacks to the upstream glyph endpoint recorded during the rewrite.
type Service struct {
	httpClient *http.Client
	styleURL   string
	ttl        time.Duration
	dark       bool
	logger     *slog.Logger
	now        func() time.Time // injectable clock for deterministic tests

	mu             sync.Mutex
	style          []byte // rewritten style, ready to serve
	styleFetchedAt time.Time
	upstreamGlyphs string // absolutised upstream glyphs template ("" = none)

	glyphMu    sync.Mutex
	glyphCache map[string][]byte

	fetchSuccess    atomic.Int64
	fetchFailure    atomic.Int64
	lastSuccessUnix atomic.Int64
}

// NewService builds a basemap Service. A nil httpClient falls back to
// http.DefaultClient (no timeout) — production always injects a timed client.
func NewService(httpClient *http.Client, cfg Config, logger *slog.Logger) *Service {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = slog.Default()
	}
	styleURL := strings.TrimSpace(cfg.StyleURL)
	if styleURL == "" {
		styleURL = DefaultStyleURL
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultStyleTTL
	}
	return &Service{
		httpClient: httpClient,
		styleURL:   styleURL,
		ttl:        ttl,
		dark:       cfg.Dark,
		logger:     logger,
		now:        time.Now,
		glyphCache: make(map[string][]byte),
	}
}

// FetchSuccessCount, FetchFailureCount and CacheAgeSeconds expose the standard
// upstream-source metrics trio (same idiom as pkg/weathertiles), covering both
// style and proxied-glyph fetches.
func (s *Service) FetchSuccessCount() int64 { return s.fetchSuccess.Load() }

// FetchFailureCount returns the total number of failed upstream fetches.
func (s *Service) FetchFailureCount() int64 { return s.fetchFailure.Load() }

// CacheAgeSeconds returns the age of the last successful upstream fetch in
// seconds, or -1 if none has succeeded yet.
func (s *Service) CacheAgeSeconds(now time.Time) int64 {
	last := s.lastSuccessUnix.Load()
	if last == 0 {
		return -1
	}
	return int64(now.Sub(time.Unix(last, 0)).Seconds())
}

// StyleHandler serves the rewritten style at GET /basemap/style.json. A fetch
// failure with no cached style yields 502 — the map cannot come up without a
// style, and an honest error beats an empty white canvas with no explanation.
func (s *Service) StyleHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		style, err := s.ensureStyle(r.Context())
		if err != nil {
			s.logger.Error("basemap style unavailable", slog.String("error", err.Error()))
			http.Error(w, "basemap style unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Short browser cache: the style is small and the server-side TTL is the
		// real freshness control.
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(style)
	}
}

// ensureStyle returns the cached rewritten style, refetching it when the TTL
// has expired. On refetch failure a stale cached style is served (logged), so
// a temporary upstream outage never blanks a running scope.
func (s *Service) ensureStyle(ctx context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.style != nil && s.now().Sub(s.styleFetchedAt) < s.ttl {
		return s.style, nil
	}
	raw, err := s.fetch(ctx, s.styleURL, maxStyleBytes)
	if err == nil {
		var rewritten []byte
		var upstreamGlyphs string
		rewritten, upstreamGlyphs, err = rewriteStyle(raw, s.styleURL, s.dark)
		if err == nil {
			s.style = rewritten
			s.styleFetchedAt = s.now()
			s.upstreamGlyphs = upstreamGlyphs
			return s.style, nil
		}
	}
	if s.style != nil {
		s.logger.Warn("basemap style refresh failed; serving stale style",
			slog.String("error", err.Error()))
		return s.style, nil
	}
	return nil, err
}

// Reload updates the upstream style URL and dark flag at runtime (#310, K2) and
// forces a refetch on the next request. Defensive: the cached last-good style is
// KEPT (styleFetchedAt reset, not the cache), so ensureStyle serves the previous
// style if the refetch with the new settings fails — a bad new URL never blanks a
// running scope. An empty styleURL falls back to DefaultStyleURL.
func (s *Service) Reload(styleURL string, dark bool) {
	styleURL = strings.TrimSpace(styleURL)
	if styleURL == "" {
		styleURL = DefaultStyleURL
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.styleURL = styleURL
	s.dark = dark
	s.styleFetchedAt = time.Time{} // TTL expired → next ensureStyle refetches; last-good kept
}

// StyleJSON returns the rewritten (URL-absolutised) style — the same bytes
// StyleHandler serves. Consumed by pkg/basemapsearch (#277), whose sector
// index must read the SAME tile source the map renders (online or mirror).
func (s *Service) StyleJSON(ctx context.Context) ([]byte, error) {
	return s.ensureStyle(ctx)
}

// upstreamGlyphsTemplate returns the recorded upstream glyphs URL template,
// fetching the style first if it has never been loaded (a browser may request
// glyphs before the style on a warm reload).
func (s *Service) upstreamGlyphsTemplate(ctx context.Context) string {
	if _, err := s.ensureStyle(ctx); err != nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upstreamGlyphs
}

// GlyphsHandler wraps the embedded local glyph handler: fontstacks Wayfinder
// hosts itself (isLocal) are served from the embed, anything else is proxied to
// the upstream glyph endpoint recorded from the style rewrite. Without an
// upstream template the request falls through to the local handler (404 for
// unknown stacks — same behaviour as before this package existed).
func (s *Service) GlyphsHandler(local http.Handler, isLocal func(fontstack string) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fontstack, rng, ok := parseGlyphPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if isLocal(fontstack) {
			local.ServeHTTP(w, r)
			return
		}
		tmpl := s.upstreamGlyphsTemplate(r.Context())
		if tmpl == "" {
			local.ServeHTTP(w, r)
			return
		}
		s.serveProxiedGlyph(r.Context(), w, tmpl, fontstack, rng)
	})
}

// serveProxiedGlyph serves one upstream glyph range through the bounded cache.
func (s *Service) serveProxiedGlyph(ctx context.Context, w http.ResponseWriter, tmpl, fontstack, rng string) {
	key := fontstack + "/" + rng
	s.glyphMu.Lock()
	data, ok := s.glyphCache[key]
	s.glyphMu.Unlock()
	if !ok {
		// The fontstack is a browser-supplied path segment: escape it before it
		// becomes part of the upstream URL (rng is already regexp-validated).
		u := strings.ReplaceAll(tmpl, "{fontstack}", url.PathEscape(fontstack))
		u = strings.ReplaceAll(u, "{range}", strings.TrimSuffix(rng, ".pbf"))
		var err error
		data, err = s.fetch(ctx, u, maxGlyphBytes)
		if err != nil {
			s.logger.Warn("basemap glyph proxy failed",
				slog.String("fontstack", fontstack), slog.String("range", rng),
				slog.String("error", err.Error()))
			http.Error(w, "glyph unavailable", http.StatusBadGateway)
			return
		}
		s.glyphMu.Lock()
		if len(s.glyphCache) >= maxGlyphCacheEntries {
			for k := range s.glyphCache { // evict one arbitrary entry: bounded memory, glyphs are refetchable
				delete(s.glyphCache, k)
				break
			}
		}
		s.glyphCache[key] = data
		s.glyphMu.Unlock()
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}

// fetch GETs url with the size cap and counts the metrics trio.
func (s *Service) fetch(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		s.fetchFailure.Add(1)
		return nil, fmt.Errorf("basemap: build request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.fetchFailure.Add(1)
		return nil, fmt.Errorf("basemap: fetch %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		s.fetchFailure.Add(1)
		return nil, fmt.Errorf("basemap: fetch %s: HTTP %d", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		s.fetchFailure.Add(1)
		return nil, fmt.Errorf("basemap: read %s: %w", rawURL, err)
	}
	if int64(len(data)) > maxBytes {
		s.fetchFailure.Add(1)
		return nil, fmt.Errorf("basemap: %s exceeds %d bytes", rawURL, maxBytes)
	}
	s.fetchSuccess.Add(1)
	s.lastSuccessUnix.Store(s.now().Unix())
	return data, nil
}

// rewriteStyle adapts the upstream style for serving from Wayfinder's origin:
// "glyphs" is pointed at the local endpoint (returning the absolutised upstream
// template for the proxy), relative sprite/tile/source URLs are absolutised
// against the upstream style URL, and an attribution is injected when the
// upstream carries none (basemap.de terms of use). With dark set, the
// radar-scope colour transform (scope.go, H2) is applied on top.
func rewriteStyle(raw []byte, styleURL string, dark bool) (out []byte, upstreamGlyphs string, err error) {
	base, err := url.Parse(styleURL)
	if err != nil {
		return nil, "", fmt.Errorf("basemap: bad style URL: %w", err)
	}
	var style map[string]any
	if err := json.Unmarshal(raw, &style); err != nil {
		return nil, "", fmt.Errorf("basemap: upstream style is not valid JSON: %w", err)
	}

	if g, ok := style["glyphs"].(string); ok && g != "" {
		upstreamGlyphs = absolutizeTemplate(base, g)
	}
	style["glyphs"] = localGlyphsTemplate

	switch sprite := style["sprite"].(type) {
	case string:
		style["sprite"] = absolutizeTemplate(base, sprite)
	case []any: // multi-sprite form: [{"id":…,"url":…}, …]
		for _, entry := range sprite {
			if m, ok := entry.(map[string]any); ok {
				if u, ok := m["url"].(string); ok {
					m["url"] = absolutizeTemplate(base, u)
				}
			}
		}
	}

	sources, _ := style["sources"].(map[string]any)
	hasAttribution := false
	names := make([]string, 0, len(sources))
	for name, src := range sources {
		names = append(names, name)
		m, ok := src.(map[string]any)
		if !ok {
			continue
		}
		if u, ok := m["url"].(string); ok {
			m["url"] = absolutizeTemplate(base, u)
		}
		if tiles, ok := m["tiles"].([]any); ok {
			for i, t := range tiles {
				if ts, ok := t.(string); ok {
					tiles[i] = absolutizeTemplate(base, ts)
				}
			}
		}
		if a, ok := m["attribution"].(string); ok && strings.TrimSpace(a) != "" {
			hasAttribution = true
		}
	}
	if !hasAttribution && len(names) > 0 {
		sort.Strings(names) // deterministic pick across map iteration order
		if m, ok := sources[names[0]].(map[string]any); ok {
			m["attribution"] = defaultAttribution
		}
	}

	if dark {
		darkenStyle(style)
	}

	out, err = json.Marshal(style)
	if err != nil {
		return nil, "", fmt.Errorf("basemap: re-encode style: %w", err)
	}
	return out, upstreamGlyphs, nil
}

// absolutizeTemplate resolves a possibly-relative URL against base, preserving
// MapLibre's {placeholder} template segments (url.Parse accepts them verbatim).
// Unparseable values are returned unchanged — better an odd URL in the style
// than a hard failure for a cosmetic field.
func absolutizeTemplate(base *url.URL, raw string) string {
	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if ref.IsAbs() {
		return raw
	}
	resolved := base.ResolveReference(ref).String()
	// url.String() percent-encodes the braces of MapLibre's {z}/{x}/{y}/… URL
	// templates; restore them so the client-side templating still matches.
	resolved = strings.ReplaceAll(resolved, "%7B", "{")
	return strings.ReplaceAll(resolved, "%7D", "}")
}

// parseGlyphPath splits "/glyphs/{fontstack}/{range}.pbf" into its validated
// segments. The fontstack arrives percent-decoded in the URL path; it must be a
// single, sane path segment before it is matched against local stacks or
// re-escaped into an upstream URL.
func parseGlyphPath(p string) (fontstack, rng string, ok bool) {
	rest, found := strings.CutPrefix(p, "/glyphs/")
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	fontstack, rng = parts[0], parts[1]
	if fontstack == "" || len(fontstack) > 200 || strings.Contains(fontstack, "..") {
		return "", "", false
	}
	for _, r := range fontstack {
		if r < 0x20 || r == 0x7f {
			return "", "", false
		}
	}
	if !rangeRe.MatchString(rng) {
		return "", "", false
	}
	return fontstack, rng, true
}
