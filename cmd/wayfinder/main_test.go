package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/basemap"
)

func TestMapConfigHandlerDefaultStyle(t *testing.T) {
	cfg := Config{
		MapCenterLat: 50.0379,
		MapCenterLon: 8.5622,
		MapZoom:      8,
		MapStyleURL:  "",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/map-config", nil)
	rec := httptest.NewRecorder()

	mapConfigHandler(cfg)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		CenterLat float64         `json:"center_lat"`
		CenterLon float64         `json:"center_lon"`
		Zoom      float64         `json:"zoom"`
		Style     json.RawMessage `json:"style"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.CenterLat != cfg.MapCenterLat || body.CenterLon != cfg.MapCenterLon || body.Zoom != cfg.MapZoom {
		t.Errorf("unexpected center/zoom: %+v", body)
	}

	var style map[string]any
	if err := json.Unmarshal(body.Style, &style); err != nil {
		t.Fatalf("expected style to be a JSON object, got %s: %v", body.Style, err)
	}
	if style["version"] != float64(8) {
		t.Errorf("expected style version 8, got %v", style["version"])
	}
	// A glyphs endpoint is mandatory: a symbol layer with a text-field renders
	// no text without one, which previously left all track/aero labels invisible.
	if glyphs, ok := style["glyphs"].(string); !ok || glyphs == "" {
		t.Errorf("expected non-empty glyphs URL in built-in style, got %v", style["glyphs"])
	}
}

func TestMapConfigHandlerCustomStyleURL(t *testing.T) {
	cfg := Config{
		MapCenterLat: 1,
		MapCenterLon: 2,
		MapZoom:      3,
		MapStyleURL:  "https://example.com/style.json",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/map-config", nil)
	rec := httptest.NewRecorder()

	mapConfigHandler(cfg)(rec, req)

	var body struct {
		Style string `json:"style"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Style != cfg.MapStyleURL {
		t.Errorf("expected style %q, got %q", cfg.MapStyleURL, body.Style)
	}
}

// TestMapConfigHandlerCorrelationAvailable pins the #245 Teil B / ADR 0024 UI
// gate: map-config reports correlation_available iff a Firefly command token is
// configured, so the panel only shows correlation controls that can succeed.
func TestMapConfigHandlerCorrelationAvailable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token string
		want  bool
	}{
		{"token set → available", "s3cr3t-token", true},
		{"token empty → unavailable", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{FireflyCommandToken: tc.token}
			req := httptest.NewRequest(http.MethodGet, "/api/map-config", nil)
			rec := httptest.NewRecorder()
			mapConfigHandler(cfg)(rec, req)

			var body struct {
				CorrelationAvailable bool `json:"correlation_available"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.CorrelationAvailable != tc.want {
				t.Errorf("correlation_available = %v, want %v", body.CorrelationAvailable, tc.want)
			}
		})
	}
}

func TestLoadConfigParsesSecurityEnvVars(t *testing.T) {
	for _, env := range []struct{ key, value string }{
		{"WAYFINDER_ALLOWED_ORIGINS", "https://a.example, https://b.example"},
		{"WAYFINDER_TLS_CERT", "/tmp/cert.pem"},
		{"WAYFINDER_TLS_KEY", "/tmp/key.pem"},
	} {
		t.Setenv(env.key, env.value)
	}

	cfg := loadConfig()

	wantOrigins := []string{"https://a.example", "https://b.example"}
	if len(cfg.AllowedOrigins) != len(wantOrigins) {
		t.Fatalf("expected %d allowed origins, got %v", len(wantOrigins), cfg.AllowedOrigins)
	}
	for i, want := range wantOrigins {
		if cfg.AllowedOrigins[i] != want {
			t.Errorf("allowed origin %d: expected %q, got %q", i, want, cfg.AllowedOrigins[i])
		}
	}

	if cfg.TLSCertFile != "/tmp/cert.pem" {
		t.Errorf("expected TLSCertFile %q, got %q", "/tmp/cert.pem", cfg.TLSCertFile)
	}
	if cfg.TLSKeyFile != "/tmp/key.pem" {
		t.Errorf("expected TLSKeyFile %q, got %q", "/tmp/key.pem", cfg.TLSKeyFile)
	}
}

// TestLoadConfigParsesFireflyCommandToken pins the ADR 0024 §E2 command token env:
// present ⇒ carried into cfg (enables /api/correlation); unset ⇒ empty (endpoint
// disabled, 503).
func TestLoadConfigParsesFireflyCommandToken(t *testing.T) {
	t.Setenv("WAYFINDER_FIREFLY_COMMAND_TOKEN", "s3cr3t-token")
	if got := loadConfig().FireflyCommandToken; got != "s3cr3t-token" {
		t.Errorf("FireflyCommandToken = %q, want s3cr3t-token", got)
	}
}

func TestLoadConfigParsesLogLevel(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"WARN", slog.LevelWarn},
	} {
		t.Setenv("WAYFINDER_LOG_LEVEL", tc.env)

		cfg := loadConfig()

		if cfg.LogLevel != tc.want {
			t.Errorf("WAYFINDER_LOG_LEVEL=%q: expected level %v, got %v", tc.env, tc.want, cfg.LogLevel)
		}
	}
}

func TestLoadConfigLogLevelDefaultsToInfo(t *testing.T) {
	_ = os.Unsetenv("WAYFINDER_LOG_LEVEL")

	cfg := loadConfig()

	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("expected default log level info, got %v", cfg.LogLevel)
	}
}

func TestLoadConfigInvalidLogLevelFallsBackToDefault(t *testing.T) {
	t.Setenv("WAYFINDER_LOG_LEVEL", "not-a-level")

	cfg := loadConfig()

	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("expected default log level info for invalid input, got %v", cfg.LogLevel)
	}
}

// ADR 0017 (connected-by-default): the DWD radar + warnings overlays are ON by
// default (public DWD URLs baked in), and only disabled via an explicit
// WAYFINDER_DWD_..._ENABLED=false flag.
func TestLoadConfigDWDConnectedByDefault(t *testing.T) {
	for _, k := range []string{
		"WAYFINDER_DWD_RADAR_ENABLED", "WAYFINDER_DWD_WMS_URL",
		"WAYFINDER_DWD_WARN_ENABLED", "WAYFINDER_DWD_WARN_URL",
	} {
		_ = os.Unsetenv(k)
	}
	cfg := loadConfig()
	if !cfg.DWDRadarEnabled {
		t.Error("DWDRadarEnabled: want true by default (connected-by-default)")
	}
	if cfg.DWDWMSURL == "" {
		t.Error("DWDWMSURL: want a non-empty public-DWD default")
	}
	if !cfg.DWDWarnEnabled {
		t.Error("DWDWarnEnabled: want true by default")
	}
	if cfg.DWDWarnURL == "" {
		t.Error("DWDWarnURL: want a non-empty public-DWD default")
	}
}

func TestLoadConfigDWDEnabledOptOut(t *testing.T) {
	t.Setenv("WAYFINDER_DWD_RADAR_ENABLED", "false")
	t.Setenv("WAYFINDER_DWD_WARN_ENABLED", "0")
	cfg := loadConfig()
	if cfg.DWDRadarEnabled {
		t.Error("WAYFINDER_DWD_RADAR_ENABLED=false must disable the radar overlay")
	}
	if cfg.DWDWarnEnabled {
		t.Error("WAYFINDER_DWD_WARN_ENABLED=0 must disable the warnings overlay")
	}
	// The ENABLED flag is the opt-out — the URL default stays.
	if cfg.DWDWMSURL == "" {
		t.Error("URL default should remain even when the overlay is disabled")
	}
}

func TestLoadConfigDWDURLOverride(t *testing.T) {
	t.Setenv("WAYFINDER_DWD_WMS_URL", "https://mirror.example/geoserver/dwd/wms")
	cfg := loadConfig()
	if cfg.DWDWMSURL != "https://mirror.example/geoserver/dwd/wms" {
		t.Errorf("DWDWMSURL override: got %q", cfg.DWDWMSURL)
	}
}

func TestLoadConfigQNHConnectedByDefault(t *testing.T) {
	for _, k := range []string{"WAYFINDER_QNH_ENABLED", "WAYFINDER_METAR_STATIONS", "WAYFINDER_METAR_URL"} {
		_ = os.Unsetenv(k)
	}
	cfg := loadConfig()
	if !cfg.QNHEnabled {
		t.Error("QNHEnabled: want true by default (connected-by-default, CBD-3)")
	}
}

func TestLoadConfigQNHEnabledOptOut(t *testing.T) {
	t.Setenv("WAYFINDER_QNH_ENABLED", "false")
	cfg := loadConfig()
	if cfg.QNHEnabled {
		t.Error("WAYFINDER_QNH_ENABLED=false must disable the QNH source")
	}
}

func TestLoadConfigMetarStationsFallback(t *testing.T) {
	t.Setenv("WAYFINDER_METAR_STATIONS", "EDDF, EDDL ,")
	cfg := loadConfig()
	if len(cfg.MetarStations) != 2 || cfg.MetarStations[0] != "EDDF" || cfg.MetarStations[1] != "EDDL" {
		t.Errorf("MetarStations = %v, want [EDDF EDDL] (trimmed, blanks dropped)", cfg.MetarStations)
	}
}

func TestLoadConfigOpenAIPRefreshDeprecated(t *testing.T) {
	_ = os.Unsetenv("WAYFINDER_OPENAIP_REFRESH")
	if loadConfig().OpenAIPRefreshDeprecated {
		t.Error("unset WAYFINDER_OPENAIP_REFRESH should not flag deprecation")
	}
	t.Setenv("WAYFINDER_OPENAIP_REFRESH", "12h")
	if !loadConfig().OpenAIPRefreshDeprecated {
		t.Error("a set WAYFINDER_OPENAIP_REFRESH should flag the deprecation warning")
	}
}

func TestEnvBool(t *testing.T) {
	const key = "WAYFINDER_TEST_ENVBOOL"
	_ = os.Unsetenv(key)
	if !envBool(key, true) {
		t.Error("unset should return default true")
	}
	if envBool(key, false) {
		t.Error("unset should return default false")
	}
	for v, want := range map[string]bool{"true": true, "false": false, "1": true, "0": false, "TRUE": true} {
		t.Setenv(key, v)
		if got := envBool(key, !want); got != want {
			t.Errorf("envBool(%q) = %v, want %v", v, got, want)
		}
	}
	t.Setenv(key, "maybe") // unparseable → default
	if !envBool(key, true) {
		t.Error("unparseable value should fall back to the default")
	}
}

func TestMapConfigHandlerDarkThemeByDefault(t *testing.T) {
	cfg := Config{MapTheme: mapThemeDark}

	req := httptest.NewRequest(http.MethodGet, "/api/map-config", nil)
	rec := httptest.NewRecorder()
	mapConfigHandler(cfg)(rec, req)

	var body struct {
		Theme string          `json:"theme"`
		Style json.RawMessage `json:"style"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Theme != mapThemeDark {
		t.Errorf("expected theme %q, got %q", mapThemeDark, body.Theme)
	}

	var style map[string]any
	if err := json.Unmarshal(body.Style, &style); err != nil {
		t.Fatalf("expected style object, got %s: %v", body.Style, err)
	}
	sources, ok := style["sources"].(map[string]any)
	if !ok {
		t.Fatalf("expected sources object, got %v", style["sources"])
	}
	if _, ok := sources["carto-dark"]; !ok {
		t.Errorf("expected dark theme to use the carto-dark source, got sources %v", sources)
	}
}

func TestMapConfigHandlerOSMTheme(t *testing.T) {
	cfg := Config{MapTheme: mapThemeOSM}

	req := httptest.NewRequest(http.MethodGet, "/api/map-config", nil)
	rec := httptest.NewRecorder()
	mapConfigHandler(cfg)(rec, req)

	var body struct {
		Theme string          `json:"theme"`
		Style json.RawMessage `json:"style"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Theme != mapThemeOSM {
		t.Errorf("expected theme %q, got %q", mapThemeOSM, body.Theme)
	}

	var style map[string]any
	if err := json.Unmarshal(body.Style, &style); err != nil {
		t.Fatalf("expected style object, got %s: %v", body.Style, err)
	}
	sources := style["sources"].(map[string]any)
	if _, ok := sources["osm"]; !ok {
		t.Errorf("expected osm theme to use the osm source, got sources %v", sources)
	}
}

func TestMapConfigHandlerCustomStyleURLReportsTheme(t *testing.T) {
	cfg := Config{MapStyleURL: "https://example.com/style.json", MapTheme: mapThemeDark}

	req := httptest.NewRequest(http.MethodGet, "/api/map-config", nil)
	rec := httptest.NewRecorder()
	mapConfigHandler(cfg)(rec, req)

	var body struct {
		Theme string `json:"theme"`
		Style string `json:"style"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Style != cfg.MapStyleURL {
		t.Errorf("expected custom style URL %q, got %q", cfg.MapStyleURL, body.Style)
	}
	if body.Theme != mapThemeDark {
		t.Errorf("expected reported theme %q, got %q", mapThemeDark, body.Theme)
	}
}

// TestMapConfigHandlerBKGTheme: the "bkg"/"bkg-dark" themes (ADR 0026) must
// hand the browser Wayfinder's own style endpoint (string URL, not an inline
// style) so the server-side rewrite (glyphs → /glyphs, dark transform) is
// always in the path.
func TestMapConfigHandlerBKGTheme(t *testing.T) {
	for _, theme := range []string{mapThemeBKG, mapThemeBKGDark} {
		cfg := Config{MapTheme: theme}

		req := httptest.NewRequest(http.MethodGet, "/api/map-config", nil)
		rec := httptest.NewRecorder()
		mapConfigHandler(cfg)(rec, req)

		var body struct {
			Theme string `json:"theme"`
			Style string `json:"style"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Style != "/basemap/style.json" {
			t.Errorf("theme %q: expected style \"/basemap/style.json\", got %q", theme, body.Style)
		}
		if body.Theme != theme {
			t.Errorf("expected reported theme %q, got %q", theme, body.Theme)
		}
	}
}

// TestLoadConfigBKGStyleURL: default is the public basemap.world "Farbe" style
// (official Germany + BKG-curated world context, ADR 0026 Nachtrag); the env
// var overrides it (Germany-only, grey variant, self-hosted mirror).
func TestLoadConfigBKGStyleURL(t *testing.T) {
	_ = os.Unsetenv("WAYFINDER_BKG_STYLE_URL")
	if cfg := loadConfig(); cfg.BKGStyleURL != basemap.DefaultStyleURL {
		t.Errorf("default BKGStyleURL = %q, want %q", cfg.BKGStyleURL, basemap.DefaultStyleURL)
	}
	// The default must be the world style — a Germany-only default would leave
	// cross-border sectors with an empty void at the national border.
	if !strings.Contains(basemap.DefaultStyleURL, "basemapworld") {
		t.Errorf("DefaultStyleURL %q is not the basemap.world style", basemap.DefaultStyleURL)
	}
	t.Setenv("WAYFINDER_BKG_STYLE_URL", "https://mirror.example/style.json")
	if cfg := loadConfig(); cfg.BKGStyleURL != "https://mirror.example/style.json" {
		t.Errorf("BKGStyleURL override not applied: %q", cfg.BKGStyleURL)
	}
}

func TestLoadConfigMapTheme(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want string
	}{
		{"", mapThemeDark},            // default
		{"dark", mapThemeDark},        //
		{"osm", mapThemeOSM},          //
		{"OSM", mapThemeOSM},          // case-insensitive
		{"bkg", mapThemeBKG},          // ADR 0026
		{"BKG", mapThemeBKG},          // case-insensitive
		{"bkg-dark", mapThemeBKGDark}, // ADR 0026 Nachtrag / H2
		{"nonsense", mapThemeDark},    // invalid → default
	} {
		if tc.env == "" {
			_ = os.Unsetenv("WAYFINDER_MAP_THEME")
		} else {
			t.Setenv("WAYFINDER_MAP_THEME", tc.env)
		}

		cfg := loadConfig()

		if cfg.MapTheme != tc.want {
			t.Errorf("WAYFINDER_MAP_THEME=%q: expected theme %q, got %q", tc.env, tc.want, cfg.MapTheme)
		}
	}
}

func TestLoadYAMLFileAppliesValues(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "wayfinder-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`
map:
  center_lat: 48.1234
  center_lon: 11.5678
  zoom: 9
openaip:
  radius_km: 56
`)
	_ = f.Close()

	cfg := Config{
		MapCenterLat:    50.0379,
		MapCenterLon:    8.5622,
		MapZoom:         8,
		OpenAIPRadiusKM: 250,
	}
	loadYAMLFile(f.Name(), &cfg, slog.Default())

	if cfg.MapCenterLat != 48.1234 {
		t.Errorf("center_lat: got %v, want 48.1234", cfg.MapCenterLat)
	}
	if cfg.MapCenterLon != 11.5678 {
		t.Errorf("center_lon: got %v, want 11.5678", cfg.MapCenterLon)
	}
	if cfg.MapZoom != 9 {
		t.Errorf("zoom: got %v, want 9", cfg.MapZoom)
	}
	if cfg.OpenAIPRadiusKM != 56 {
		t.Errorf("radius_km: got %v, want 56", cfg.OpenAIPRadiusKM)
	}
}

func TestLoadYAMLFileMissingFileIsNonFatal(t *testing.T) {
	cfg := Config{MapCenterLat: 50.0}
	loadYAMLFile("/nonexistent/wayfinder.yaml", &cfg, slog.Default())
	if cfg.MapCenterLat != 50.0 {
		t.Errorf("defaults must be preserved when file is missing")
	}
}

func TestLoadYAMLFileInvalidYAMLIsNonFatal(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "wayfinder-bad-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(":::invalid yaml:::")
	_ = f.Close()

	cfg := Config{MapCenterLat: 50.0}
	loadYAMLFile(f.Name(), &cfg, slog.Default())
	if cfg.MapCenterLat != 50.0 {
		t.Errorf("defaults must be preserved on YAML parse error")
	}
}

func TestLoadYAMLFilePartialOverride(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "wayfinder-partial-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Only radius_km set — other defaults must remain unchanged
	_, _ = f.WriteString(`
openaip:
  radius_km: 100
`)
	_ = f.Close()

	cfg := Config{MapCenterLat: 50.0379, MapCenterLon: 8.5622, MapZoom: 8, OpenAIPRadiusKM: 250}
	loadYAMLFile(f.Name(), &cfg, slog.Default())

	if cfg.MapCenterLat != 50.0379 {
		t.Errorf("center_lat must be unchanged when not set in YAML")
	}
	if cfg.OpenAIPRadiusKM != 100 {
		t.Errorf("radius_km: got %v, want 100", cfg.OpenAIPRadiusKM)
	}
}

func TestLoadYAMLFileEnvVarWins(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "wayfinder-envwin-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`
map:
  center_lat: 48.0
`)
	_ = f.Close()
	t.Setenv("WAYFINDER_CONFIG_FILE", f.Name())
	t.Setenv("WAYFINDER_MAP_CENTER_LAT", "51.5")
	defer func() {
		_ = os.Unsetenv("WAYFINDER_CONFIG_FILE")
		_ = os.Unsetenv("WAYFINDER_MAP_CENTER_LAT")
	}()

	cfg := loadConfig()
	// Env-var (51.5) must win over YAML (48.0)
	if cfg.MapCenterLat != 51.5 {
		t.Errorf("env var must override YAML: got %v, want 51.5", cfg.MapCenterLat)
	}
}

func TestLoadConfigSecurityEnvVarsDefaultEmpty(t *testing.T) {
	for _, key := range []string{"WAYFINDER_ALLOWED_ORIGINS", "WAYFINDER_TLS_CERT", "WAYFINDER_TLS_KEY"} {
		_ = os.Unsetenv(key)
	}

	cfg := loadConfig()

	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("expected no allowed origins by default, got %v", cfg.AllowedOrigins)
	}
	if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		t.Errorf("expected empty TLS config by default, got cert=%q key=%q", cfg.TLSCertFile, cfg.TLSKeyFile)
	}
}
