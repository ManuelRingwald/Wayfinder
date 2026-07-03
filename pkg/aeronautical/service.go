package aeronautical

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// endpointPaths maps each kind to the internal frontend endpoint that serves
// its cached GeoJSON (ADR 0004).
var endpointPaths = map[Kind]string{
	KindAirspace: "/api/airspace",
	KindNavaid:   "/api/navaids",
	KindWaypoint: "/api/waypoints",
}

// allKinds is the fetch/serve order.
var allKinds = []Kind{KindAirspace, KindNavaid, KindWaypoint}

// CacheStore persists the fetched GeoJSON so it survives a redeploy (AERO-1, ADR
// 0018). It is injected so the aeronautical package stays free of the store/DB
// detail. A nil store means "in-memory only" (no persistence) — used by tests and
// as a graceful fallback. tenantID nil addresses the global fallback cache row.
type CacheStore interface {
	Load(ctx context.Context, tenantID *int64, kind Kind) (fc FeatureCollection, fetchedAt time.Time, ok bool, err error)
	// Save persists the fetched collection plus the change summary of this refresh
	// (AERO-3) so the admin can see what churned per layer.
	Save(ctx context.Context, tenantID *int64, kind Kind, fc FeatureCollection, change ChangeSummary, fetchedAt time.Time) error
}

// Config configures the aeronautical Service.
type Config struct {
	// Enabled gates the whole feature. When false (e.g. no API key configured)
	// fetching does nothing and the endpoints serve empty collections.
	Enabled bool
	// BBox is the geographic window queried from OpenAIP.
	BBox BoundingBox
	// Store persists the fetched cache across redeploys (AERO-1, ADR 0018). nil =
	// in-memory only.
	Store CacheStore
	// TenantID selects which persisted cache row this Service owns (nil = the
	// global fallback row).
	TenantID *int64
}

// Service fetches aeronautical data from OpenAIP, caches the last good result per
// kind (in memory, backed by a persistent DB cache), and serves it as GeoJSON.
// Per ADR 0004 it is best-effort: failures keep the last-good cache and never
// surface as errors to the frontend or to readiness. Since AERO-1 (ADR 0018) it
// fetches once/on-demand rather than on a periodic ticker: the caller hydrates it
// from the persistent store on boot and triggers a fetch only when needed.
type Service struct {
	client *Client
	cfg    Config
	logger *slog.Logger
	cache  map[Kind]*atomic.Pointer[FeatureCollection]

	fetchSuccess    atomic.Int64
	fetchFailure    atomic.Int64
	lastSuccessUnix atomic.Int64
}

// NewService creates a Service. The cache starts empty; endpoints serve empty
// collections until Hydrate loads the persisted cache or the first fetch succeeds.
func NewService(client *Client, cfg Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Service{
		client: client,
		cfg:    cfg,
		logger: logger,
		cache:  make(map[Kind]*atomic.Pointer[FeatureCollection], len(allKinds)),
	}
	for _, k := range allKinds {
		s.cache[k] = &atomic.Pointer[FeatureCollection]{}
	}
	return s
}

// Hydrate loads the persisted cache (if any) into memory without any network
// call (AERO-1, ADR 0018), so the map is warm immediately after a redeploy. A nil
// store or an empty cache is a no-op. Best-effort: a load error is logged and
// skipped, never fatal.
func (s *Service) Hydrate(ctx context.Context) {
	if s.cfg.Store == nil {
		return
	}
	for _, kind := range allKinds {
		fc, fetchedAt, ok, err := s.cfg.Store.Load(ctx, s.cfg.TenantID, kind)
		if err != nil {
			s.logger.Warn("aeronautical hydrate failed; skipping",
				slog.String("kind", string(kind)), slog.String("error", err.Error()))
			continue
		}
		if !ok {
			continue
		}
		stored := fc
		s.cache[kind].Store(&stored)
		if u := fetchedAt.Unix(); u > s.lastSuccessUnix.Load() {
			s.lastSuccessUnix.Store(u)
		}
	}
}

// HasData reports whether any kind has a cached collection (from a hydrate or a
// fetch). Used to decide whether a first-time fetch is needed on boot.
func (s *Service) HasData() bool {
	for _, kind := range allKinds {
		if s.cache[kind].Load() != nil {
			return true
		}
	}
	return false
}

// BootstrapOnce hydrates from the persistent store and, only if the feature is
// enabled and nothing was hydrated (a fresh install, no persisted data yet), does
// a single fetch to populate the cache. It never runs a background loop — the
// AERO-1 fetch model is fetch-once/on-demand (ADR 0018).
func (s *Service) BootstrapOnce(ctx context.Context) {
	if !s.cfg.Enabled {
		s.logger.Warn("aeronautical layers disabled (no OpenAIP API key); " +
			"map will show no airspace/navaid/waypoint overlays")
		return
	}
	s.Hydrate(ctx)
	if !s.HasData() {
		s.refreshAll(ctx)
	}
}

// RefreshNow performs a single fetch of every kind and persists the result (a
// forced refresh, e.g. an admin key change or the AERO-2 refresh buttons). It is
// a no-op when the feature is disabled.
func (s *Service) RefreshNow(ctx context.Context) {
	if !s.cfg.Enabled {
		return
	}
	s.refreshAll(ctx)
}

// refreshAll fetches every kind, keeping the last-good cache on failure and
// persisting each success to the store (AERO-1) so it survives a redeploy.
func (s *Service) refreshAll(ctx context.Context) {
	for _, kind := range allKinds {
		fc, err := s.client.Fetch(ctx, kind, s.cfg.BBox)
		if err != nil {
			s.fetchFailure.Add(1)
			s.logger.Warn("aeronautical refresh failed; keeping last-good cache",
				slog.String("kind", string(kind)), slog.String("error", err.Error()))
			continue
		}
		// Change-impact (AERO-3): diff the fresh data against what we had before
		// overwriting the in-memory cache, so the admin sees what churned per layer.
		change := diffCollections(s.cache[kind].Load(), fc)
		stored := fc
		s.cache[kind].Store(&stored)
		s.fetchSuccess.Add(1)
		now := time.Now()
		s.lastSuccessUnix.Store(now.Unix())
		if s.cfg.Store != nil {
			if err := s.cfg.Store.Save(ctx, s.cfg.TenantID, kind, fc, change, now); err != nil {
				s.logger.Warn("aeronautical persist failed; cache kept in memory only",
					slog.String("kind", string(kind)), slog.String("error", err.Error()))
			}
		}
		s.logger.Debug("aeronautical refresh ok",
			slog.String("kind", string(kind)), slog.Int("features", len(fc.Features)))
	}
}

// Register wires the GeoJSON endpoints onto the given mux.
func (s *Service) Register(mux *http.ServeMux) {
	for _, kind := range allKinds {
		mux.HandleFunc(endpointPaths[kind], s.handler(kind))
	}
}

// Serve returns the cached collection for one kind, or an empty collection if
// nothing has been cached yet (graceful degradation, ADR 0004). It is the read
// side shared by the single-tenant HTTP handler and the per-tenant Registry
// (ONB-6), which serves a tenant's own cache and falls back to this one.
func (s *Service) Serve(kind Kind) FeatureCollection {
	if c := s.cache[kind]; c != nil {
		if ptr := c.Load(); ptr != nil {
			return *ptr
		}
	}
	return EmptyCollection()
}

// handler serves the cached collection for one kind (single-tenant path).
func (s *Service) handler(kind Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeFeatureCollection(w, s.Serve(kind))
	}
}

// writeFeatureCollection encodes a FeatureCollection as JSON. Shared by the
// single-tenant Service handler and the per-tenant Registry handlers (ONB-6).
func writeFeatureCollection(w http.ResponseWriter, fc FeatureCollection) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fc)
}

// FetchSuccessCount returns the number of successful per-kind fetches.
func (s *Service) FetchSuccessCount() int64 { return s.fetchSuccess.Load() }

// FetchFailureCount returns the number of failed per-kind fetches.
func (s *Service) FetchFailureCount() int64 { return s.fetchFailure.Load() }

// CacheAgeSeconds returns how long ago (seconds) the last successful fetch
// happened, or -1 if there has never been one. Makes a staling cache observable
// (ADR 0004: the feature stays available but ages visibly on a long outage).
func (s *Service) CacheAgeSeconds(now time.Time) int64 {
	last := s.lastSuccessUnix.Load()
	if last == 0 {
		return -1
	}
	return now.Unix() - last
}
