// Package webui serves the Wayfinder ASD frontend (static HTML/JS/CSS).
// The dist/ directory is produced by `npm run build` in frontend/ and is
// embedded into the Go binary at compile time. See ADR 0002.
package webui

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed dist
var distFS embed.FS

// glyphsFS embeds the self-hosted MapLibre glyph PBFs (Roboto Mono Medium SDF
// ranges), so the scope renders its ATC data blocks in the monospace face with
// NO runtime font CDN — the same air-gap principle as the @fontsource UI fonts
// (ADR 0015). The PBFs are generated once from the Roboto Mono TTF with fontnik
// and committed; the build needs no font tooling. Layout: one directory per
// fontstack ("Roboto Mono Medium"), one file per 256-codepoint range.
//
//go:embed "glyphs"
var glyphsFS embed.FS

// Handler returns an http.Handler that serves the embedded frontend assets with
// SPA history-mode fallback (WF2-32): any request that does not resolve to a real
// embedded file is answered with index.html, so the client-side router owns deep
// links like /admin even on a hard reload or bookmark. The API surface (/api/…,
// /ws, /health, /ready, /metrics) is registered as more specific mux patterns and
// therefore never reaches this handler, so the fallback can never shadow a real
// endpoint. The shell HTML is served no-cache (it is tiny and changes every build,
// while the hashed assets under /assets are immutable and cache freely).
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))
	// Embedded files carry no real modification time; a stable per-process value
	// is fine for the fallback shell, which is sent no-cache anyway.
	startup := time.Now()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" || fileExists(sub, name) {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Unknown path → hand the shell to the SPA router.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", startup, bytes.NewReader(index))
	}), nil
}

// GlyphsHandler serves the embedded MapLibre glyph PBFs at
// /glyphs/{fontstack}/{range}.pbf (the URL the map style's "glyphs" template
// expands to). The fontstack segment arrives percent-decoded in r.URL.Path (it
// contains spaces, e.g. "Roboto Mono Medium"), which matches the embedded
// directory name directly. Only *.pbf under the embedded tree is served; a
// missing range yields 404 (MapLibre then renders those code points blank),
// never a directory listing or path escape. The glyphs are immutable, so they
// cache aggressively.
func GlyphsHandler() (http.Handler, error) {
	sub, err := fs.Sub(glyphsFS, "glyphs")
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// path.Clean collapses any ".." so the lookup cannot escape the embed FS.
		name := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(r.URL.Path, "/glyphs/")), "/")
		if !strings.HasSuffix(name, ".pbf") || !fileExists(sub, name) {
			http.NotFound(w, r)
			return
		}
		f, err := sub.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = f.Close() }()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = io.Copy(w, f)
	}), nil
}

// fileExists reports whether name is a regular (non-directory) file in fsys.
// Directories return false so a bare directory path falls through to the SPA
// shell instead of triggering a file-server directory redirect/listing.
func fileExists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return !info.IsDir()
}
