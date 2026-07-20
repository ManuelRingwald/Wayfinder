package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/manuelringwald/wayfinder/pkg/basemap"
	"github.com/manuelringwald/wayfinder/pkg/mapconfig"
)

// mapDataConfig is the runtime configuration plane for the map-data subsystems
// (Epic #307), built on the K0 mapconfig primitives. Each setting resolves
// DB-override ?? env-default (platform_settings); a change triggers a defensive
// hot-reload of the owning service. mapConfigHandler reads the effective values
// here so /api/map-config reflects live overrides. Admin endpoints under
// /api/admin/mapdata/* (RequireRole(admin)) read/write the settings.
//
// K2 covers the base map (style URL + theme). K3–K5 add weather / coverage /
// aeronautical settings to the same plane.
type mapDataConfig struct {
	registry   *mapconfig.Registry
	logger     *slog.Logger
	basemapSvc *basemap.Service // may be nil (a custom MapStyleURL bypasses the service)

	// Basiskarte (K2)
	styleURL *mapconfig.Setting
	theme    *mapconfig.Setting
}

// platform_settings keys for the map-data plane. Namespaced so they never clash
// with other settings (e.g. the sealed OpenAIP key).
const (
	msBasemapStyleURL = "mapdata.basemap.style_url"
	msBasemapTheme    = "mapdata.basemap.theme"

	domainBasemap = "basemap"
)

// newMapDataConfig seeds each setting with the process start-up env default and
// registers the per-subsystem reloads. basemapSvc may be nil.
func newMapDataConfig(st mapconfig.Store, cfg Config, basemapSvc *basemap.Service, logger *slog.Logger) *mapDataConfig {
	if logger == nil {
		logger = slog.Default()
	}
	styleDefault := cfg.BKGStyleURL
	if styleDefault == "" {
		styleDefault = basemap.DefaultStyleURL
	}
	m := &mapDataConfig{
		registry:   mapconfig.NewRegistry(logger),
		logger:     logger,
		basemapSvc: basemapSvc,
		styleURL:   mapconfig.NewSetting(st, msBasemapStyleURL, styleDefault),
		theme:      mapconfig.NewSetting(st, msBasemapTheme, cfg.MapTheme),
	}
	m.registry.Register(domainBasemap, m.reloadBasemap)
	return m
}

// reloadBasemap re-reads the effective style URL + theme and applies them to the
// basemap service (forces a re-fetch, keeps last-good on failure). A nil service
// (custom style) is a no-op.
func (m *mapDataConfig) reloadBasemap(ctx context.Context) error {
	if m.basemapSvc == nil {
		return nil
	}
	url, err := m.styleURL.Effective(ctx)
	if err != nil {
		return err
	}
	theme, err := m.theme.Effective(ctx)
	if err != nil {
		return err
	}
	m.basemapSvc.Reload(url, theme == mapThemeBKGDark)
	return nil
}

// applyAtBoot applies any stored DB overrides to the services once at start-up,
// so a restart honours values set through the admin UI (the services were built
// from env defaults before the DB pool was open). Best-effort: a failure logs and
// leaves the start-up config in place.
func (m *mapDataConfig) applyAtBoot(ctx context.Context) {
	if err := m.registry.Trigger(ctx, domainBasemap); err != nil {
		m.logger.Warn("mapdata boot apply failed; using start-up config",
			slog.String("domain", domainBasemap), slog.String("error", err.Error()))
	}
}

// effectiveTheme reads the live base-map theme for mapConfigHandler (falls back
// to the dark default on an empty/failed read).
func (m *mapDataConfig) effectiveTheme(ctx context.Context) string {
	v, err := m.theme.Effective(ctx)
	if err != nil || v == "" {
		return mapThemeBKGDark
	}
	return v
}

// validTheme validates a theme value for the admin PUT.
func validTheme(v string) error {
	if v == mapThemeBKG || v == mapThemeBKGDark {
		return nil
	}
	return fmt.Errorf("theme must be %q or %q", mapThemeBKG, mapThemeBKGDark)
}

// mountAdminRoutes wires the map-data admin endpoints. wrap applies the admin
// auth middleware (tenantMW ∘ requireAdmin).
func (m *mapDataConfig) mountAdminRoutes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	styleRes := &mapconfig.Resource{
		Setting:  m.styleURL,
		Registry: m.registry,
		Domain:   domainBasemap,
		Validate: func(v string) error { return mapconfig.ValidateFetchURL(v, nil) },
	}
	themeRes := &mapconfig.Resource{
		Setting:  m.theme,
		Registry: m.registry,
		Domain:   domainBasemap,
		Validate: validTheme,
	}
	mux.Handle("/api/admin/mapdata/basemap/style-url", wrap(styleRes.Handler()))
	mux.Handle("/api/admin/mapdata/basemap/theme", wrap(themeRes.Handler()))
}
