package weatherwarnings

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

const defaultRefresh = 5 * time.Minute

// Config configures the warnings Service.
type Config struct {
	// Enabled gates the feature. When false (no WFS URL) the endpoint serves an
	// empty collection and the refresh loop does nothing.
	Enabled bool
	// Refresh is the poll interval. DWD warnings change on the order of minutes;
	// ~5 min is a sensible default.
	Refresh time.Duration
}

// Service periodically fetches DWD warnings, caches the last good collection, and
// serves it as GeoJSON. Best-effort (ADR 0016): failures keep the last-good cache
// and never surface as errors or affect readiness.
type Service struct {
	client  *Client
	enabled bool
	refresh time.Duration
	logger  *slog.Logger
	now     func() time.Time

	cache atomic.Pointer[FeatureCollection]

	fetchSuccess    atomic.Int64
	fetchFailure    atomic.Int64
	lastSuccessUnix atomic.Int64
}

// NewService builds a warnings Service. The cache starts empty; the endpoint
// serves an empty collection until the first successful fetch.
func NewService(client *Client, cfg Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	refresh := cfg.Refresh
	if refresh <= 0 {
		refresh = defaultRefresh
	}
	return &Service{
		client:  client,
		enabled: cfg.Enabled,
		refresh: refresh,
		logger:  logger,
		now:     time.Now,
	}
}

// Run performs an initial fetch then refreshes on the configured interval until
// ctx is cancelled. Returns immediately when disabled. Non-blocking wrt readiness.
func (s *Service) Run(ctx context.Context) {
	if !s.enabled {
		s.logger.Warn("weather warnings overlay disabled (no WAYFINDER_DWD_WARN_URL); map will show no warnings")
		return
	}
	s.refreshOnce(ctx)
	ticker := time.NewTicker(s.refresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshOnce(ctx)
		}
	}
}

// refreshOnce fetches once, keeping the last-good cache on failure.
func (s *Service) refreshOnce(ctx context.Context) {
	fc, err := s.client.Fetch(ctx)
	if err != nil {
		s.fetchFailure.Add(1)
		s.logger.Debug("weather warnings refresh failed; keeping last-good cache", slog.String("error", err.Error()))
		return
	}
	stored := fc
	s.cache.Store(&stored)
	s.fetchSuccess.Add(1)
	s.lastSuccessUnix.Store(s.now().Unix())
	s.logger.Debug("weather warnings refresh ok", slog.Int("features", len(fc.Features)))
}

// Serve returns the cached collection or an empty one (graceful degradation).
func (s *Service) Serve() FeatureCollection {
	if ptr := s.cache.Load(); ptr != nil {
		return *ptr
	}
	return EmptyCollection()
}

// Handler serves the cached warnings as GeoJSON. Always 200 with a valid
// (possibly empty) collection — best-effort, never an error (ADR 0016).
func (s *Service) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		w.Header().Set("Cache-Control", "no-cache")
		_ = json.NewEncoder(w).Encode(s.Serve())
	}
}

// FetchSuccessCount / FetchFailureCount / CacheAgeSeconds mirror the other weather
// services for uniform /metrics wiring.
func (s *Service) FetchSuccessCount() int64 { return s.fetchSuccess.Load() }
func (s *Service) FetchFailureCount() int64 { return s.fetchFailure.Load() }
func (s *Service) CacheAgeSeconds(now time.Time) int64 {
	last := s.lastSuccessUnix.Load()
	if last == 0 {
		return -1
	}
	return now.Unix() - last
}
