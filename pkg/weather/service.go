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
	// Enabled gates the NOAA/AWC METAR source (CBD-3, connected-by-default, ADR
	// 0017): true = the poller runs and the endpoint serves data; false
	// (WAYFINDER_QNH_ENABLED=false) = the poller never runs and the endpoint serves
	// an empty list. It no longer depends on any station being configured — a source
	// that is on but has nothing to poll simply serves nothing until a tenant sets
	// an aerodrome.
	Enabled bool
	// Stations is the optional GLOBAL fallback aerodrome list (deprecated
	// WAYFINDER_METAR_STATIONS), in priority order; the first is the fallback
	// header "primary" for a tenant that has not set its own aerodrome. Upper-cased
	// by NewService. The per-tenant aerodrome (StationsProvider) is the primary
	// mechanism (CBD-3).
	Stations []string
	// StationsProvider, when set, returns the dynamic per-tenant poll set (the union
	// of every tenant's configured aerodrome). It is read on each refresh so newly
	// configured aerodromes are picked up without a restart. Combined with (unioned
	// over) the static Stations fallback.
	StationsProvider func() []string
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
	static     []string        // global fallback aerodromes (deprecated env)
	provider   func() []string // dynamic per-tenant poll set (CBD-3)
	refresh    time.Duration
	staleAfter time.Duration
	enabled    bool
	logger     *slog.Logger
	now        func() time.Time // injectable clock for deterministic tests
	kick       chan struct{}    // non-blocking "refresh now" trigger

	mu    sync.Mutex
	cache map[string]QNH

	fetchSuccess    atomic.Int64
	fetchFailure    atomic.Int64
	lastSuccessUnix atomic.Int64
}

// NewService builds a QNH Service. The static fallback stations are upper-cased
// and de-duplicated in order; enabled reflects only whether the NOAA source is
// switched on (CBD-3) — a source that is on with no aerodrome to poll runs but
// serves nothing.
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
	return &Service{
		client:     client,
		static:     normaliseStations(cfg.Stations),
		provider:   cfg.StationsProvider,
		refresh:    refresh,
		staleAfter: staleAfter,
		enabled:    cfg.Enabled,
		logger:     logger,
		now:        time.Now,
		kick:       make(chan struct{}, 1),
		cache:      make(map[string]QNH),
	}
}

// currentStations is the poll set for this tick: the dynamic per-tenant union
// (provider) combined with the static global fallback, upper-cased and de-duped.
func (s *Service) currentStations() []string {
	var raw []string
	if s.provider != nil {
		raw = append(raw, s.provider()...)
	}
	raw = append(raw, s.static...)
	return normaliseStations(raw)
}

// Refresh asks the poll loop to fetch now (best-effort, non-blocking). Used after
// a tenant edits its aerodrome so a freshly configured station is polled promptly
// instead of waiting up to one refresh interval.
func (s *Service) Refresh() {
	select {
	case s.kick <- struct{}{}:
	default: // a refresh is already pending; coalesce.
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

// Run performs an initial poll then polls on the configured interval (or on a
// Refresh() kick) until ctx is cancelled. It returns immediately when the source
// is disabled. It keeps ticking even when no aerodrome is configured yet, so a
// tenant that sets one later is picked up. Non-blocking wrt readiness.
func (s *Service) Run(ctx context.Context) {
	if !s.enabled {
		s.logger.Warn("QNH source disabled (WAYFINDER_QNH_ENABLED=false); header shows no QNH")
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
		case <-s.kick:
			s.refreshOnce(ctx)
		}
	}
}

// refreshOnce polls the current station set, updating the per-station cache on
// success and keeping the last-good cache on failure. A no-op when nothing is
// configured — the source stays on, ready for the next tenant to add an aerodrome.
func (s *Service) refreshOnce(ctx context.Context) {
	stations := s.currentStations()
	if len(stations) == 0 {
		return
	}
	reports, err := s.client.FetchMETAR(ctx, stations)
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

// SnapshotFor returns the per-station QNH DTOs for the given aerodromes in the
// given priority order (the first with a reading is the header primary), omitting
// stations that have never been read. When icaos is empty it falls back to the
// static global list (deprecated WAYFINDER_METAR_STATIONS) so a tenant without its
// own aerodrome still sees the configured fallback. This is the tenant-scoped view
// (CBD-3): the poller keeps the union of all aerodromes warm; each tenant reads
// only its own.
func (s *Service) SnapshotFor(icaos []string) qnhResponse {
	icaos = normaliseStations(icaos)
	if len(icaos) == 0 {
		icaos = s.static
	}
	now := s.now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := qnhResponse{Stations: make([]stationDTO, 0, len(icaos))}
	for _, icao := range icaos {
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

// Snapshot returns the snapshot for the static fallback stations. Retained for the
// legacy global Handler and tests; the tenant-scoped path uses SnapshotFor.
func (s *Service) Snapshot() qnhResponse { return s.SnapshotFor(nil) }

// Handler serves the static/global QNH snapshot as JSON. Always 200 with a
// (possibly empty) station list — best-effort, never an error (ADR 0016). The
// per-tenant endpoint uses TenantHandler.
func (s *Service) Handler() http.HandlerFunc {
	return s.TenantHandler(nil)
}

// TenantHandler serves the QNH snapshot for the aerodrome(s) the resolve function
// returns for the request (CBD-3): the tenant's configured aerodrome, resolved by
// the caller from the request context. A nil resolver (or one returning nothing)
// falls back to the static global list. Always 200 — best-effort, never an error.
func (s *Service) TenantHandler(resolve func(r *http.Request) []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var icaos []string
		if resolve != nil {
			icaos = resolve(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		_ = json.NewEncoder(w).Encode(s.SnapshotFor(icaos))
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
