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

// Config configures the aeronautical Service.
type Config struct {
	// Enabled gates the whole feature. When false (e.g. no API key configured)
	// the refresh loop does nothing and the endpoints serve empty collections.
	Enabled bool
	// BBox is the geographic window queried from OpenAIP.
	BBox BoundingBox
	// Refresh is the interval between background refreshes (AIRAC-paced;
	// default applied by the caller, e.g. 24 h).
	Refresh time.Duration
}

// Service periodically refreshes aeronautical data from OpenAIP, caches the
// last good result per kind, and serves it as GeoJSON. Per ADR 0004 it is
// best-effort: failures keep the last-good cache and never surface as errors to
// the frontend or to readiness.
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
// collections until the first successful refresh.
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

// Run performs an initial refresh and then refreshes on the configured
// interval until ctx is cancelled. It returns immediately (after logging) when
// the feature is disabled. Run is non-blocking with respect to readiness: it is
// meant to be launched in its own goroutine and never reports fatal errors.
func (s *Service) Run(ctx context.Context) {
	if !s.cfg.Enabled {
		s.logger.Warn("aeronautical layers disabled (no OpenAIP API key); " +
			"map will show no airspace/navaid/waypoint overlays")
		return
	}

	s.refreshAll(ctx)

	interval := s.cfg.Refresh
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshAll(ctx)
		}
	}
}

// refreshAll refreshes every kind, keeping the last-good cache on failure.
func (s *Service) refreshAll(ctx context.Context) {
	for _, kind := range allKinds {
		fc, err := s.client.Fetch(ctx, kind, s.cfg.BBox)
		if err != nil {
			s.fetchFailure.Add(1)
			s.logger.Warn("aeronautical refresh failed; keeping last-good cache",
				slog.String("kind", string(kind)), slog.String("error", err.Error()))
			continue
		}
		stored := fc
		s.cache[kind].Store(&stored)
		s.fetchSuccess.Add(1)
		s.lastSuccessUnix.Store(time.Now().Unix())
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

// handler serves the cached collection for one kind, or an empty collection if
// nothing has been cached yet (graceful degradation, ADR 0004).
func (s *Service) handler(kind Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fc := EmptyCollection()
		if ptr := s.cache[kind].Load(); ptr != nil {
			fc = *ptr
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fc)
	}
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
