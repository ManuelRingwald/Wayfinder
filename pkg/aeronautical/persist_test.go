package aeronautical

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// fakeCacheStore is an in-memory CacheStore for the persistence tests (AERO-1). It
// keys by kind only (single logical cache), which is enough to exercise
// hydrate/persist/fetch-once without a database.
type fakeCacheStore struct {
	mu      sync.Mutex
	data    map[Kind]FeatureCollection
	at      map[Kind]time.Time
	changes map[Kind]ChangeSummary
	saves   int
}

func newFakeCacheStore() *fakeCacheStore {
	return &fakeCacheStore{data: map[Kind]FeatureCollection{}, at: map[Kind]time.Time{}, changes: map[Kind]ChangeSummary{}}
}

func (f *fakeCacheStore) Load(_ context.Context, _ *int64, kind Kind) (FeatureCollection, time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.data[kind]
	return fc, f.at[kind], ok, nil
}

func (f *fakeCacheStore) Save(_ context.Context, _ *int64, kind Kind, fc FeatureCollection, change ChangeSummary, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[kind] = fc
	f.at[kind] = at
	f.changes[kind] = change
	f.saves++
	return nil
}

func (f *fakeCacheStore) changeFor(kind Kind) ChangeSummary {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.changes[kind]
}

func (f *fakeCacheStore) saveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saves
}

// failClient points at a server that fails, to prove a code path made no network
// call (a fetch would bump FetchFailureCount).
func failClient(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.Client(), srv.URL, "k")
}

func TestHydrateLoadsPersistedWithoutFetch(t *testing.T) {
	store := newFakeCacheStore()
	// Pre-populate the persistent cache as if a previous run had fetched navaids.
	store.data[KindNavaid] = FeatureCollection{Type: "FeatureCollection",
		Features: []Feature{{Type: "Feature"}, {Type: "Feature"}}}
	store.at[KindNavaid] = time.Unix(1_700_000_000, 0)

	s := NewService(failClient(t), Config{Enabled: true, Store: store}, nil)
	s.Hydrate(context.Background())

	if got := len(s.Serve(KindNavaid).Features); got != 2 {
		t.Fatalf("hydrated navaids = %d, want 2", got)
	}
	if s.FetchSuccessCount() != 0 || s.FetchFailureCount() != 0 {
		t.Error("Hydrate must not perform any network fetch")
	}
	if got := s.CacheAgeSeconds(time.Unix(1_700_000_030, 0)); got < 29 || got > 31 {
		t.Errorf("hydrated cache age = %d, want ~30 (from persisted fetched_at)", got)
	}
}

func TestBootstrapOnceSkipsFetchWhenPersistedDataExists(t *testing.T) {
	store := newFakeCacheStore()
	store.data[KindNavaid] = FeatureCollection{Type: "FeatureCollection", Features: []Feature{{Type: "Feature"}}}

	// A redeploy: BootstrapOnce hydrates and, because data is present, must NOT fetch
	// (the client would fail; a fetch would show up as a failure).
	s := NewService(failClient(t), Config{Enabled: true, Store: store}, nil)
	s.BootstrapOnce(context.Background())

	if s.FetchSuccessCount() != 0 || s.FetchFailureCount() != 0 {
		t.Error("BootstrapOnce with persisted data must hydrate only, never fetch (redeploy)")
	}
	if got := len(s.Serve(KindNavaid).Features); got != 1 {
		t.Errorf("served navaids = %d, want 1 (from hydrate)", got)
	}
}

func TestBootstrapOnceFetchesAndPersistsWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()
	store := newFakeCacheStore()

	// Fresh install: empty store + a key → one fetch populates and persists.
	s := NewService(NewClient(srv.Client(), srv.URL, "k"), Config{Enabled: true, Store: store}, nil)
	s.BootstrapOnce(context.Background())

	if !s.HasData() {
		t.Fatal("BootstrapOnce on an empty store should have fetched and populated the cache")
	}
	if store.saveCount() == 0 {
		t.Error("a successful fetch must be persisted to the store")
	}
}

func feat(name string) Feature {
	return Feature{Type: "Feature", Properties: map[string]any{"name": name}}
}

func TestDiffCollections(t *testing.T) {
	// First fetch: no prior → HasPrev false, no counts.
	if got := diffCollections(nil, FeatureCollection{Features: []Feature{feat("A")}}); got.HasPrev {
		t.Errorf("nil prior should yield HasPrev=false, got %+v", got)
	}

	prev := &FeatureCollection{Features: []Feature{feat("A"), feat("B"), feat("C")}}
	next := FeatureCollection{Features: []Feature{feat("A"), feat("C"), feat("D"), feat("E")}}
	got := diffCollections(prev, next)
	// B removed; D, E added; A, C unchanged.
	if !got.HasPrev || got.PrevFeatureCount != 3 || got.Added != 2 || got.Removed != 1 {
		t.Errorf("diff = %+v, want prev 3 / +2 / -1", got)
	}

	// Identical collections → no churn.
	same := diffCollections(prev, *prev)
	if same.Added != 0 || same.Removed != 0 {
		t.Errorf("identical diff = %+v, want no churn", same)
	}

	// An in-place content edit shows as one removed + one added (content-keyed).
	edited := FeatureCollection{Features: []Feature{feat("A"), feat("B"), feat("C-edited")}}
	e := diffCollections(prev, edited)
	if e.Added != 1 || e.Removed != 1 {
		t.Errorf("in-place edit diff = %+v, want +1/-1", e)
	}
}

func TestRefreshAllComputesChangeSummary(t *testing.T) {
	// Seed the in-memory cache with a prior navaid collection, then refresh with a
	// changed one and prove the persisted change summary reflects the churn.
	store := newFakeCacheStore()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids)) // yields 2 navaid features
	}))
	defer srv.Close()
	s := NewService(NewClient(srv.Client(), srv.URL, "k"), Config{Enabled: true, Store: store}, nil)
	// Prime navaid cache with a single (different) feature so the diff is non-trivial.
	prior := FeatureCollection{Type: "FeatureCollection", Features: []Feature{feat("OLD")}}
	s.cache[KindNavaid].Store(&prior)

	s.refreshAll(context.Background())
	// Navaid had a prior in the cache → its change summary carries HasPrev with churn
	// (the OLD feature removed, the 2 sample navaids added).
	nav := store.changeFor(KindNavaid)
	if !nav.HasPrev || nav.PrevFeatureCount != 1 || nav.Added != 2 || nav.Removed != 1 {
		t.Errorf("navaid change = %+v, want prev 1 / +2 / -1", nav)
	}
	// Airspace had no prior (nil cache) → first-fetch, HasPrev false.
	if store.changeFor(KindAirspace).HasPrev {
		t.Error("airspace had no prior; its change should be HasPrev=false")
	}
}

func TestRefreshAllPersistsEveryKind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()
	store := newFakeCacheStore()

	s := NewService(NewClient(srv.Client(), srv.URL, "k"), Config{Enabled: true, Store: store}, nil)
	s.refreshAll(context.Background())

	// One save per kind (all three fetch successfully against the stub).
	if got := store.saveCount(); got != len(allKinds) {
		t.Errorf("persisted %d kinds, want %d", got, len(allKinds))
	}
}

func TestRegistryForceRefetchesDespitePersistedData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()
	store := newFakeCacheStore()
	store.data[KindNavaid] = FeatureCollection{Type: "FeatureCollection", Features: []Feature{{Type: "Feature"}}}

	factory := func(apiKey string) *Client { return NewClient(srv.Client(), srv.URL, apiKey) }
	reg := NewRegistry(context.Background(), nil, factory, store, nil)
	defer reg.StopAll()

	// force=true: re-fetch even though the hydrated cache already has data.
	reg.Start(7, "k", BoundingBox{}, true)
	waitFor(t, func() bool { return reg.FetchSuccessCount() > 0 })
}

func TestRegistryNewTenantHydratesWithoutFetch(t *testing.T) {
	store := newFakeCacheStore()
	store.data[KindNavaid] = FeatureCollection{Type: "FeatureCollection", Features: []Feature{{Type: "Feature"}, {Type: "Feature"}}}

	factory := func(apiKey string) *Client { return NewClient(http.DefaultClient, "http://unused.invalid", apiKey) }
	reg := NewRegistry(context.Background(), nil, factory, store, nil)
	defer reg.StopAll()

	// A brand-new tenant on boot with persisted data (force=false): hydrate, no fetch.
	reg.Start(7, "k", BoundingBox{}, false)
	waitFor(t, func() bool { return len(reg.Serve(7, KindNavaid).Features) == 2 })
	if reg.FetchFailureCount() != 0 {
		t.Error("a new tenant with persisted data must hydrate, not fetch (no failures expected)")
	}
}
