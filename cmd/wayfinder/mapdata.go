package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/basemap"
	"github.com/manuelringwald/wayfinder/pkg/coverage"
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

	// Radar-/Luftlageabdeckung (K4): the sensor list is a JSON blob; the ring
	// colour a plain string. Both fall back to the env-configured start-up value.
	coverageSensors *mapconfig.Setting
	coverageColor   *mapconfig.Setting
	envSensors      []coverage.SensorConfig // start-up env default

	// Wetter (K3): DWD radar / warnings / QNH. Enable flags are stored as
	// "true"/"false" strings. Enable/disable + availability are LIVE (read by
	// /api/map-config); URL/layer overrides are applied at the next restart (the
	// weather services are rebuilt from these effective values before their poll
	// loops start — no live goroutine reconfiguration, keeping a running feed safe).
	radarEnabled *mapconfig.Setting
	radarURL     *mapconfig.Setting
	radarLayer   *mapconfig.Setting
	warnEnabled  *mapconfig.Setting
	warnURL      *mapconfig.Setting
	warnLayer    *mapconfig.Setting
	qnhEnabled   *mapconfig.Setting
}

// platform_settings keys for the map-data plane. Namespaced so they never clash
// with other settings (e.g. the sealed OpenAIP key).
const (
	msBasemapStyleURL = "mapdata.basemap.style_url"
	msBasemapTheme    = "mapdata.basemap.theme"
	msCoverageSensors = "mapdata.coverage.sensors"
	msCoverageColor   = "mapdata.coverage.ring_color"
	msRadarEnabled    = "mapdata.weather.radar_enabled"
	msRadarURL        = "mapdata.weather.radar_wms_url"
	msRadarLayer      = "mapdata.weather.radar_layer"
	msWarnEnabled     = "mapdata.weather.warn_enabled"
	msWarnURL         = "mapdata.weather.warn_url"
	msWarnLayer       = "mapdata.weather.warn_layer"
	msQNHEnabled      = "mapdata.weather.qnh_enabled"

	domainBasemap = "basemap"

	defaultCoverageColor = "#5B8DEF"
	maxCoverageSensors   = 20
)

// boolStr renders a Go bool as the "true"/"false" string stored in settings.
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// effBool reads a boolean setting (stored as "true"/"false"); anything other
// than "true" is false.
func effBool(ctx context.Context, s *mapconfig.Setting) bool {
	v, err := s.Effective(ctx)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(v), "true")
}

// effStr reads a string setting, returning "" on error.
func effStr(ctx context.Context, s *mapconfig.Setting) string {
	v, _ := s.Effective(ctx)
	return v
}

// validBool validates a boolean-string admin PUT value.
func validBool(v string) error {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "false":
		return nil
	}
	return fmt.Errorf("value must be true or false")
}

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
	colorDefault := cfg.CoverageRingColor
	if colorDefault == "" {
		colorDefault = defaultCoverageColor
	}
	m := &mapDataConfig{
		registry:        mapconfig.NewRegistry(logger),
		logger:          logger,
		basemapSvc:      basemapSvc,
		styleURL:        mapconfig.NewSetting(st, msBasemapStyleURL, styleDefault),
		theme:           mapconfig.NewSetting(st, msBasemapTheme, cfg.MapTheme),
		coverageSensors: mapconfig.NewSetting(st, msCoverageSensors, ""),
		coverageColor:   mapconfig.NewSetting(st, msCoverageColor, colorDefault),
		envSensors:      cfg.CoverageSensors,

		radarEnabled: mapconfig.NewSetting(st, msRadarEnabled, boolStr(cfg.DWDRadarEnabled)),
		radarURL:     mapconfig.NewSetting(st, msRadarURL, cfg.DWDWMSURL),
		radarLayer:   mapconfig.NewSetting(st, msRadarLayer, cfg.DWDRadarLayer),
		warnEnabled:  mapconfig.NewSetting(st, msWarnEnabled, boolStr(cfg.DWDWarnEnabled)),
		warnURL:      mapconfig.NewSetting(st, msWarnURL, cfg.DWDWarnURL),
		warnLayer:    mapconfig.NewSetting(st, msWarnLayer, cfg.DWDWarnLayer),
		qnhEnabled:   mapconfig.NewSetting(st, msQNHEnabled, boolStr(cfg.QNHEnabled)),
	}
	m.registry.Register(domainBasemap, m.reloadBasemap)
	return m
}

// Weather availability — LIVE (read by /api/map-config): a source is available
// when it is enabled AND has an upstream URL configured. Mirrors the original
// env-based computation, now over the effective (override ?? env) values.
func (m *mapDataConfig) radarAvailable(ctx context.Context) bool {
	return effBool(ctx, m.radarEnabled) && strings.TrimSpace(effStr(ctx, m.radarURL)) != ""
}
func (m *mapDataConfig) warningsAvailable(ctx context.Context) bool {
	return effBool(ctx, m.warnEnabled) && strings.TrimSpace(effStr(ctx, m.warnURL)) != ""
}
func (m *mapDataConfig) qnhAvailable(ctx context.Context) bool {
	return effBool(ctx, m.qnhEnabled)
}

// effectiveRadar / effectiveWarn return the effective (override ?? env) weather
// source config, used at boot to (re)build the DWD services so a restart honours
// admin overrides. enabled is also gated on a non-empty URL (a source without a
// URL cannot fetch).
func (m *mapDataConfig) effectiveRadar(ctx context.Context) (enabled bool, url, layer string) {
	url = strings.TrimSpace(effStr(ctx, m.radarURL))
	return effBool(ctx, m.radarEnabled) && url != "", url, effStr(ctx, m.radarLayer)
}
func (m *mapDataConfig) effectiveWarn(ctx context.Context) (enabled bool, url, layer string) {
	url = strings.TrimSpace(effStr(ctx, m.warnURL))
	return effBool(ctx, m.warnEnabled) && url != "", url, effStr(ctx, m.warnLayer)
}

// effectiveSensors returns the live coverage sensor list: the stored JSON
// override, else the start-up env sensors. A malformed override degrades to the
// env sensors (never an empty/broken overlay).
func (m *mapDataConfig) effectiveSensors(ctx context.Context) []coverage.SensorConfig {
	raw, err := m.coverageSensors.Effective(ctx)
	if err != nil || strings.TrimSpace(raw) == "" {
		return m.envSensors
	}
	var s []coverage.SensorConfig
	if json.Unmarshal([]byte(raw), &s) != nil {
		return m.envSensors
	}
	return s
}

// effectiveRingColor returns the live coverage ring colour.
func (m *mapDataConfig) effectiveRingColor(ctx context.Context) string {
	v, err := m.coverageColor.Effective(ctx)
	if err != nil || strings.TrimSpace(v) == "" {
		return defaultCoverageColor
	}
	return v
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
	mux.Handle("/api/admin/mapdata/coverage", wrap(m.coverageHandler()))

	// Wetter (K3): enable flags (bool), upstream URLs (SSRF-checked), layers.
	res := func(s *mapconfig.Setting, validate func(string) error) *mapconfig.Resource {
		return &mapconfig.Resource{Setting: s, Validate: validate} // no reload domain: applied at restart
	}
	urlV := func(v string) error { return mapconfig.ValidateFetchURL(v, nil) }
	mux.Handle("/api/admin/mapdata/weather/radar-enabled", wrap(res(m.radarEnabled, validBool).Handler()))
	mux.Handle("/api/admin/mapdata/weather/radar-url", wrap(res(m.radarURL, urlV).Handler()))
	mux.Handle("/api/admin/mapdata/weather/radar-layer", wrap(res(m.radarLayer, nil).Handler()))
	mux.Handle("/api/admin/mapdata/weather/warn-enabled", wrap(res(m.warnEnabled, validBool).Handler()))
	mux.Handle("/api/admin/mapdata/weather/warn-url", wrap(res(m.warnURL, urlV).Handler()))
	mux.Handle("/api/admin/mapdata/weather/warn-layer", wrap(res(m.warnLayer, nil).Handler()))
	mux.Handle("/api/admin/mapdata/weather/qnh-enabled", wrap(res(m.qnhEnabled, validBool).Handler()))
}

// coverageRequest / coverageResponse are the admin coverage payloads.
type coverageRequest struct {
	Sensors   []coverage.SensorConfig `json:"sensors"`
	RingColor string                  `json:"ring_color"`
}
type coverageResponse struct {
	Sensors    []coverage.SensorConfig `json:"sensors"`
	RingColor  string                  `json:"ring_color"`
	Overridden bool                    `json:"overridden"`
}

// coverageHandler serves GET/PUT for the radar-coverage sensor list + ring
// colour (K4). GET returns the effective list; PUT validates + stores it. The
// /coverage GeoJSON and /api/map-config recompute from the effective values on
// their next request — no service to reload.
func (m *mapDataConfig) coverageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		switch r.Method {
		case http.MethodGet:
			m.writeCoverage(w, ctx)
		case http.MethodPut:
			var req coverageRequest
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			if err := validateSensors(req.Sensors); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			blob, err := json.Marshal(req.Sensors)
			if err != nil {
				http.Error(w, "could not encode sensors", http.StatusInternalServerError)
				return
			}
			if err := m.coverageSensors.Set(ctx, string(blob)); err != nil {
				http.Error(w, "could not store sensors", http.StatusInternalServerError)
				return
			}
			if err := m.coverageColor.Set(ctx, strings.TrimSpace(req.RingColor)); err != nil {
				http.Error(w, "could not store ring colour", http.StatusInternalServerError)
				return
			}
			m.writeCoverage(w, ctx)
		case http.MethodDelete:
			// Reset to the env default (delete both overrides). Distinct from a
			// PUT with an empty list, which is an explicit "zero sensors" override.
			_ = m.coverageSensors.Reset(ctx)
			_ = m.coverageColor.Reset(ctx)
			m.writeCoverage(w, ctx)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (m *mapDataConfig) writeCoverage(w http.ResponseWriter, ctx context.Context) {
	overridden, _ := m.coverageSensors.Overridden(ctx)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(coverageResponse{
		Sensors:    m.effectiveSensors(ctx),
		RingColor:  m.effectiveRingColor(ctx),
		Overridden: overridden,
	})
}

// validateSensors screens an admin-supplied sensor list before it is stored.
func validateSensors(sensors []coverage.SensorConfig) error {
	if len(sensors) > maxCoverageSensors {
		return fmt.Errorf("at most %d sensors", maxCoverageSensors)
	}
	for i, s := range sensors {
		if s.Lat < -90 || s.Lat > 90 {
			return fmt.Errorf("sensor %d: latitude out of range", i+1)
		}
		if s.Lon < -180 || s.Lon > 180 {
			return fmt.Errorf("sensor %d: longitude out of range", i+1)
		}
		if s.MaxRangeM <= 0 {
			return fmt.Errorf("sensor %d: max range must be > 0", i+1)
		}
		if s.MinRangeM < 0 || s.MinRangeM >= s.MaxRangeM {
			return fmt.Errorf("sensor %d: min range must be ≥ 0 and < max range", i+1)
		}
	}
	return nil
}
