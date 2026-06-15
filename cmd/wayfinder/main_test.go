package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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

func TestAuthMiddlewareDisabledWithoutToken(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := authMiddleware("", next)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected request to pass through when no token is configured")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for an unauthenticated request")
	})

	handler := authMiddleware("secret", next)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareRejectsWrongToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for a wrong token")
	})

	handler := authMiddleware("secret", next)

	req := httptest.NewRequest(http.MethodGet, "/ws?token=wrong", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareAcceptsTokenViaQueryParam(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := authMiddleware("secret", next)

	req := httptest.NewRequest(http.MethodGet, "/ws?token=secret", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected request with correct token to pass through")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddlewareAcceptsBearerHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := authMiddleware("secret", next)

	req := httptest.NewRequest(http.MethodGet, "/api/map-config", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected request with correct bearer token to pass through")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestLoadConfigParsesSecurityEnvVars(t *testing.T) {
	for _, env := range []struct{ key, value string }{
		{"WAYFINDER_ALLOWED_ORIGINS", "https://a.example, https://b.example"},
		{"WAYFINDER_AUTH_TOKEN", "topsecret"},
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

	if cfg.AuthToken != "topsecret" {
		t.Errorf("expected AuthToken %q, got %q", "topsecret", cfg.AuthToken)
	}
	if cfg.TLSCertFile != "/tmp/cert.pem" {
		t.Errorf("expected TLSCertFile %q, got %q", "/tmp/cert.pem", cfg.TLSCertFile)
	}
	if cfg.TLSKeyFile != "/tmp/key.pem" {
		t.Errorf("expected TLSKeyFile %q, got %q", "/tmp/key.pem", cfg.TLSKeyFile)
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
	os.Unsetenv("WAYFINDER_LOG_LEVEL")

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

func TestLoadConfigSecurityEnvVarsDefaultEmpty(t *testing.T) {
	for _, key := range []string{"WAYFINDER_ALLOWED_ORIGINS", "WAYFINDER_AUTH_TOKEN", "WAYFINDER_TLS_CERT", "WAYFINDER_TLS_KEY"} {
		os.Unsetenv(key)
	}

	cfg := loadConfig()

	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("expected no allowed origins by default, got %v", cfg.AllowedOrigins)
	}
	if cfg.AuthToken != "" {
		t.Errorf("expected empty AuthToken by default, got %q", cfg.AuthToken)
	}
	if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		t.Errorf("expected empty TLS config by default, got cert=%q key=%q", cfg.TLSCertFile, cfg.TLSKeyFile)
	}
}
