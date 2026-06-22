package feature

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

// fakeStore is an in-memory Store for unit tests, with optional error injection.
type fakeStore struct {
	enabled map[string]bool // IsEnabled lookups
	list    map[string]bool // ListByTenant result
	err     error           // injected error for both methods
	isCalls int             // number of IsEnabled invocations
}

func (f *fakeStore) IsEnabled(_ context.Context, _ int64, key string) (bool, error) {
	f.isCalls++
	if f.err != nil {
		return false, f.err
	}
	return f.enabled[key], nil
}

func (f *fakeStore) ListByTenant(_ context.Context, _ int64) (map[string]bool, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeStore) Set(_ context.Context, _ int64, key string, enabled bool) error {
	if f.err != nil {
		return f.err
	}
	if f.list == nil {
		f.list = map[string]bool{}
	}
	f.list[key] = enabled
	return nil
}

// countingHandler captures how many records at >=Warn were emitted.
type countingHandler struct{ warns int }

func (h *countingHandler) Enabled(_ context.Context, l slog.Level) bool { return true }
func (h *countingHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		h.warns++
	}
	return nil
}
func (h *countingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *countingHandler) WithGroup(string) slog.Handler      { return h }

func newSvc(t *testing.T, fs *fakeStore) (*Service, *countingHandler) {
	t.Helper()
	h := &countingHandler{}
	return New(fs, slog.New(h)), h
}

func TestHasFeatureKnownEnabled(t *testing.T) {
	svc, _ := newSvc(t, &fakeStore{enabled: map[string]bool{"stca": true}})
	if !svc.HasFeature(context.Background(), 7, STCA) {
		t.Error("HasFeature(STCA) = false, want true")
	}
}

func TestHasFeatureKnownDisabled(t *testing.T) {
	// No row for the tenant → default-deny (fakeStore returns zero value false).
	svc, _ := newSvc(t, &fakeStore{enabled: map[string]bool{}})
	if svc.HasFeature(context.Background(), 7, MultiFeed) {
		t.Error("HasFeature(MultiFeed) = true, want false (default-deny)")
	}
}

func TestHasFeatureUnknownKeyFailsClosed(t *testing.T) {
	fs := &fakeStore{enabled: map[string]bool{"bogus": true}}
	svc, logs := newSvc(t, fs)

	if svc.HasFeature(context.Background(), 7, "bogus") {
		t.Error("HasFeature(unknown) = true, want false (fail-closed)")
	}
	if fs.isCalls != 0 {
		t.Errorf("store consulted for unknown key (%d calls), want 0", fs.isCalls)
	}
	if got := svc.UnknownKeyCount(); got != 1 {
		t.Errorf("UnknownKeyCount = %d, want 1", got)
	}
	if logs.warns != 1 {
		t.Errorf("warn logs = %d, want 1", logs.warns)
	}
}

func TestHasFeatureStoreErrorFailsClosed(t *testing.T) {
	fs := &fakeStore{err: errors.New("db down")}
	svc, logs := newSvc(t, fs)

	if svc.HasFeature(context.Background(), 7, STCA) {
		t.Error("HasFeature on store error = true, want false (fail-closed)")
	}
	if got := svc.DBErrorCount(); got != 1 {
		t.Errorf("DBErrorCount = %d, want 1", got)
	}
	if logs.warns != 1 {
		t.Errorf("warn logs = %d, want 1", logs.warns)
	}
}

func TestEffectiveDefaultDeny(t *testing.T) {
	svc, _ := newSvc(t, &fakeStore{list: map[string]bool{}})
	eff, err := svc.Effective(context.Background(), 7)
	if err != nil {
		t.Fatalf("Effective err = %v", err)
	}
	if len(eff) != 3 {
		t.Fatalf("Effective len = %d, want 3 (full catalog)", len(eff))
	}
	for k, v := range eff {
		if v {
			t.Errorf("Effective[%q] = true, want false (default-deny)", k)
		}
	}
}

func TestEffectiveOverlaysStoredAndIgnoresUnknown(t *testing.T) {
	fs := &fakeStore{list: map[string]bool{
		"stca":           true,
		"premium_layers": false,
		"legacy_removed": true, // not in catalog → must be ignored
	}}
	svc, _ := newSvc(t, fs)

	eff, err := svc.Effective(context.Background(), 7)
	if err != nil {
		t.Fatalf("Effective err = %v", err)
	}
	if !eff[STCA] {
		t.Error("Effective[stca] = false, want true")
	}
	if eff[MultiFeed] {
		t.Error("Effective[multi_feed] = true, want false (unset → deny)")
	}
	if eff[PremiumLayers] {
		t.Error("Effective[premium_layers] = true, want false")
	}
	if _, ok := eff[Key("legacy_removed")]; ok {
		t.Error("Effective leaked an unknown stored key")
	}
}

func TestEffectiveStoreErrorFailsClosed(t *testing.T) {
	fs := &fakeStore{err: errors.New("db down")}
	svc, logs := newSvc(t, fs)

	eff, err := svc.Effective(context.Background(), 7)
	if err == nil {
		t.Error("Effective err = nil, want error")
	}
	// Even on error the map is fully populated and all-deny (fail-closed).
	if len(eff) != 3 {
		t.Fatalf("Effective len on error = %d, want 3", len(eff))
	}
	for k, v := range eff {
		if v {
			t.Errorf("Effective[%q] = true on error, want false (fail-closed)", k)
		}
	}
	if got := svc.DBErrorCount(); got != 1 {
		t.Errorf("DBErrorCount = %d, want 1", got)
	}
	if logs.warns != 1 {
		t.Errorf("warn logs = %d, want 1", logs.warns)
	}
}

func TestSetRejectsUnknownKey(t *testing.T) {
	fs := &fakeStore{}
	svc, _ := newSvc(t, fs)
	if err := svc.Set(context.Background(), 7, "bogus", true); !errors.Is(err, ErrUnknownFeature) {
		t.Errorf("Set(unknown) err = %v, want ErrUnknownFeature", err)
	}
	if _, reached := fs.list["bogus"]; reached {
		t.Error("unknown key must not reach the store")
	}
}

func TestSetKnownKeyPersists(t *testing.T) {
	fs := &fakeStore{}
	svc, _ := newSvc(t, fs)
	if err := svc.Set(context.Background(), 7, STCA, true); err != nil {
		t.Fatalf("Set(STCA) err = %v", err)
	}
	if !fs.list["stca"] {
		t.Error("known key not persisted to the store")
	}
}

func TestNilLoggerDoesNotPanic(t *testing.T) {
	svc := New(&fakeStore{enabled: map[string]bool{}}, nil)
	// Exercise both fail-closed paths to ensure the nil-logger fallback works.
	_ = svc.HasFeature(context.Background(), 1, "bogus")
	if svc.UnknownKeyCount() != 1 {
		t.Errorf("UnknownKeyCount = %d, want 1", svc.UnknownKeyCount())
	}
}
