package mapconfig

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeStore is an in-memory platform_settings double. errOn forces an error on
// the named op ("get"/"set"/"delete") to exercise the fallback paths.
type fakeStore struct {
	m     map[string]string
	errOn string
}

func newFakeStore() *fakeStore { return &fakeStore{m: map[string]string{}} }

func (f *fakeStore) Get(_ context.Context, key string) (string, bool, error) {
	if f.errOn == "get" {
		return "", false, errors.New("boom")
	}
	v, ok := f.m[key]
	return v, ok, nil
}
func (f *fakeStore) Set(_ context.Context, key, value string) error {
	if f.errOn == "set" {
		return errors.New("boom")
	}
	f.m[key] = value
	return nil
}
func (f *fakeStore) Delete(_ context.Context, key string) error {
	if f.errOn == "delete" {
		return errors.New("boom")
	}
	delete(f.m, key)
	return nil
}

func TestSettingEffective(t *testing.T) {
	ctx := context.Background()
	st := newFakeStore()
	s := NewSetting(st, "map.style_url", "https://default.example/style.json")

	// No override → env default.
	if got, _ := s.Effective(ctx); got != "https://default.example/style.json" {
		t.Fatalf("default: got %q", got)
	}
	if ov, _ := s.Overridden(ctx); ov {
		t.Fatal("should not be overridden initially")
	}

	// Override → DB value.
	if err := s.Set(ctx, "https://mirror.example/s.json"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Effective(ctx); got != "https://mirror.example/s.json" {
		t.Fatalf("override: got %q", got)
	}
	if ov, _ := s.Overridden(ctx); !ov {
		t.Fatal("should be overridden after Set")
	}

	// Empty Set = reset to default.
	if err := s.Set(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Effective(ctx); got != s.Default() {
		t.Fatalf("empty-set reset: got %q", got)
	}

	// Reset also clears.
	_ = s.Set(ctx, "x")
	if err := s.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if ov, _ := s.Overridden(ctx); ov {
		t.Fatal("Reset should clear the override")
	}
}

func TestSettingEffectiveFallsBackOnStoreError(t *testing.T) {
	st := newFakeStore()
	st.errOn = "get"
	s := NewSetting(st, "k", "https://default/")
	got, err := s.Effective(context.Background())
	if err == nil {
		t.Fatal("expected the store error to surface")
	}
	if got != "https://default/" {
		t.Fatalf("a store hiccup must degrade to the env default, got %q", got)
	}
}

func TestRegistryTrigger(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry(nil)

	// Unknown domain → no-op, no error.
	if err := reg.Trigger(ctx, "nope"); err != nil {
		t.Fatalf("unknown domain should be a no-op, got %v", err)
	}

	called := 0
	reg.Register("weather", func(context.Context) error { called++; return nil })
	if err := reg.Trigger(ctx, "weather"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("reload fn not called (called=%d)", called)
	}

	// Re-register replaces.
	reg.Register("weather", func(context.Context) error { return errors.New("bad config") })
	err := reg.Trigger(ctx, "weather")
	if err == nil || !strings.Contains(err.Error(), "reload weather") {
		t.Fatalf("reload error should be wrapped + returned, got %v", err)
	}

	// nil fn ignored → only "weather" is registered.
	reg.Register("x", nil)
	if got := reg.Domains(); len(got) != 1 || got[0] != "weather" {
		t.Fatalf("nil fn must be ignored, domains=%v", got)
	}
}

func TestValidateFetchURL(t *testing.T) {
	ok := []string{
		"https://maps.dwd.de/geoserver/dwd/wms",
		"http://example.org/style.json",
		"https://8.8.8.8/tiles", // public IP literal is fine
	}
	for _, u := range ok {
		if err := ValidateFetchURL(u, nil); err != nil {
			t.Errorf("expected OK for %q, got %v", u, err)
		}
	}

	bad := []string{
		"",
		"file:///etc/passwd",
		"gopher://evil",
		"https://",                      // no host
		"http://localhost:8080/x",       // internal name
		"https://svc.internal/x",        // internal suffix
		"https://printer.local/x",       // mDNS
		"http://127.0.0.1/x",            // loopback
		"http://10.0.0.5/x",             // private
		"http://169.254.169.254/latest", // link-local (cloud metadata)
		"http://[::1]/x",                // IPv6 loopback
		"http://192.168.1.1/x",          // private
	}
	for _, u := range bad {
		if err := ValidateFetchURL(u, nil); err == nil {
			t.Errorf("expected rejection for %q", u)
		}
	}
}

func TestValidateFetchURLAllowlist(t *testing.T) {
	allow := []string{".dwd.de", "sgx.geodatenzentrum.de"}
	if err := ValidateFetchURL("https://maps.dwd.de/wms", allow); err != nil {
		t.Errorf("suffix allow should pass: %v", err)
	}
	if err := ValidateFetchURL("https://sgx.geodatenzentrum.de/x", allow); err != nil {
		t.Errorf("exact allow should pass: %v", err)
	}
	if err := ValidateFetchURL("https://evil.example/x", allow); err == nil {
		t.Error("host outside allowlist must be rejected")
	}
}

func TestResourceHandlerGetPut(t *testing.T) {
	st := newFakeStore()
	reg := NewRegistry(nil)
	reloaded := 0
	reg.Register("basemap", func(context.Context) error { reloaded++; return nil })
	res := &Resource{
		Setting:  NewSetting(st, "map.style_url", "https://default/style.json"),
		Registry: reg,
		Domain:   "basemap",
		Validate: func(v string) error { return ValidateFetchURL(v, nil) },
	}
	h := res.Handler()

	// GET → default state.
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/admin/mapdata/basemap", nil))
	var got state
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Value != "https://default/style.json" || got.Overridden {
		t.Fatalf("GET default: %+v", got)
	}

	// PUT valid → stored + reloaded.
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"value":"https://mirror.example/s.json"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT valid: code %d", rec.Code)
	}
	if reloaded != 1 {
		t.Fatalf("reload not triggered (=%d)", reloaded)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if !got.Overridden || got.Value != "https://mirror.example/s.json" {
		t.Fatalf("PUT state: %+v", got)
	}

	// PUT invalid URL → 400, not stored.
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"value":"http://169.254.169.254/"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("SSRF URL must be 400, got %d", rec.Code)
	}

	// PUT empty → reset to default.
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"value":""}`)))
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Overridden {
		t.Fatalf("empty PUT should reset, got %+v", got)
	}

	// Method not allowed.
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST should be 405, got %d", rec.Code)
	}
}

func TestResourceHandlerReloadErrorSurfaced(t *testing.T) {
	st := newFakeStore()
	reg := NewRegistry(nil)
	reg.Register("weather", func(context.Context) error { return errors.New("upstream unreachable") })
	res := &Resource{Setting: NewSetting(st, "w.url", "https://d/"), Registry: reg, Domain: "weather"}

	rec := httptest.NewRecorder()
	res.Handler()(rec, httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"value":"https://ok.example/wms"}`)))
	// Stored, but reload failed → 200 with reload_error (honest, non-destructive).
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
	var got state
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.ReloadError == "" || !got.Overridden {
		t.Fatalf("reload error should be surfaced with the stored value: %+v", got)
	}
}
