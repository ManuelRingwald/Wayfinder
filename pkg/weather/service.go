package weather

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const defaultRefresh = 15 * time.Minute
const defaultStaleAfter = 2 * time.Hour

// Config configures the QNH Service.
type Config struct {
	// Enabled gates the feature. When false (no stations configured) the endpoint
	// serves an empty station list and the poller never runs.
	Enabled bool
	// Stations are the ICAO aerodromes to poll, in priority order; the first is
	// the header "primary". Upper-cased by the caller/NewService.
	Stations []string
	// Refresh is the poll interval. METAR is issued ~every 30 min (plus SPECI);
	// ~15 min keeps the value fresh well under the AWC ~100 req/min limit.
	Refresh time.Duration
	// StaleAfter marks a QNH stale when its observation time is older than this.
	StaleAfter time.Duration
}

// QNH is one station's cached observation.
type QNH struct {
	ICAO        string
	Hpa         float64
	ObsTimeUnix int64
}

// Service polls the AWC for the configured stations, caches the last good QNH per
// station, and serves it as JSON. Best-effort (ADR 0016): a fetch failure keeps
// the last-good cache and never errors to the browser or readiness.
type Service struct {
	client     *Client
	stations   []string
	refresh    time.Duration
	staleAfter time.Duration
	enabled    bool
	logger     *slog.Logger
	now        func() time.Time // injectable clock for deterministic tests

	mu    sync.Mutex
	cache map[string]QNH

	fetchSuccess    atomic.Int64
	fetchFailure    atomic.Int64
	lastSuccessUnix atomic.Int64
}

// NewService builds a QNH Service. Stations are upper-cased and de-duplicated in
// order; empty/blank config disables the feature.
func NewService(client *Client, cfg Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	refresh := cfg.Refresh
	if refresh <= 0 {
		refresh = defaultRefresh
	}
	staleAfter := cfg.StaleAfter
	if staleAfter <= 0 {
		staleAfter = defaultStaleAfter
	}
	stations := normaliseStations(cfg.Stations)
	return &Service{
		client:     client,
		stations:   stations,
		refresh:    refresh,
		staleAfter: staleAfter,
		enabled:    cfg.Enabled && len(stations) > 0,
		logger:     logger,
		now:        time.Now,
		cache:      make(map[string]QNH),
	}
}

// normaliseStations upper-cases, trims and de-duplicates the station list,
// preserving order (the first station is the header primary).
func normaliseStations(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Run performs an initial poll then polls on the configured interval until ctx is
// cancelled. It returns immediately when disabled. Non-blocking wrt readiness.
func (s *Service) Run(ctx context.Context) {
	if !s.enabled {
		s.logger.Warn("QNH infobox disabled (no WAYFINDER_METAR_STATIONS); header shows no QNH")
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

// refreshOnce polls all stations, updating the per-station cache on success and
// keeping the last-good cache on failure.
func (s *Service) refreshOnce(ctx context.Context) {
	reports, err := s.client.FetchMETAR(ctx, s.stations)
	if err != nil {
		s.fetchFailure.Add(1)
		s.logger.Debug("QNH refresh failed; keeping last-good cache", slog.String("error", err.Error()))
		return
	}
	s.mu.Lock()
	for _, r := range reports {
		s.cache[r.ICAO] = QNH{ICAO: r.ICAO, Hpa: r.QNHHpa, ObsTimeUnix: r.ObsTimeUnix}
	}
	s.mu.Unlock()
	s.fetchSuccess.Add(1)
	s.lastSuccessUnix.Store(s.now().Unix())
}

// stationDTO is the wire shape of one station's QNH for the frontend.
type stationDTO struct {
	ICAO    string `json:"icao"`
	QNHHpa  int    `json:"qnh_hpa"` // rounded to whole hPa (cockpit/ATC convention)
	ObsTime int64  `json:"obs_time"`
	Stale   bool   `json:"stale"`
}

// qnhResponse is the /api/weather/qnh payload: the configured stations that have
// a reading (in priority order) plus the "primary" (first) for the header.
type qnhResponse struct {
	Stations []stationDTO `json:"stations"`
	Primary  *stationDTO  `json:"primary,omitempty"`
}

// Snapshot returns the current per-station QNH DTOs in configured priority order,
// omitting stations that have never been read. Exposed for the handler and tests.
func (s *Service) Snapshot() qnhResponse {
	now := s.now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := qnhResponse{Stations: make([]stationDTO, 0, len(s.stations))}
	for _, icao := range s.stations {
		q, ok := s.cache[icao]
		if !ok {
			continue
		}
		dto := stationDTO{
			ICAO:    q.ICAO,
			QNHHpa:  int(math.Round(q.Hpa)),
			ObsTime: q.ObsTimeUnix,
			Stale:   q.ObsTimeUnix != 0 && now-q.ObsTimeUnix > int64(s.staleAfter.Seconds()),
		}
		resp.Stations = append(resp.Stations, dto)
	}
	if len(resp.Stations) > 0 {
		p := resp.Stations[0]
		resp.Primary = &p
	}
	return resp
}

// Handler serves the QNH snapshot as JSON. Always 200 with a (possibly empty)
// station list — best-effort, never an error (ADR 0016).
func (s *Service) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		_ = json.NewEncoder(w).Encode(s.Snapshot())
	}
}

// FetchSuccessCount / FetchFailureCount / CacheAgeSeconds mirror the aeronautical
// and radar services for uniform /metrics wiring.
func (s *Service) FetchSuccessCount() int64 { return s.fetchSuccess.Load() }
func (s *Service) FetchFailureCount() int64 { return s.fetchFailure.Load() }
func (s *Service) CacheAgeSeconds(now time.Time) int64 {
	last := s.lastSuccessUnix.Load()
	if last == 0 {
		return -1
	}
	return now.Unix() - last
}
