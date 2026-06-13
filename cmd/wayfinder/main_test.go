package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
