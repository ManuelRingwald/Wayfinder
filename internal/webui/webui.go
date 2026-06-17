// Package webui serves the Wayfinder ASD frontend (static HTML/JS/CSS).
// The dist/ directory is produced by `npm run build` in frontend/ and is
// embedded into the Go binary at compile time. See ADR 0002.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded frontend assets.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
