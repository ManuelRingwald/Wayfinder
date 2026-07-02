package weathertiles

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config configures the radar tile Service.
type Config struct {
	// Enabled gates the whole feature. When false (no WAYFINDER_DWD_WMS_URL) the
	// handler serves transparent tiles and never touches the network.
	Enabled bool
	// TTL is how long a fetched tile is served from cache before a refetch. DWD
	// radar composites update ~every 5 min, so ~5 min is the natural default.
	TTL time.Duration
	// MaxCacheEntries bounds the in-memory tile cache; when exceeded, stale
	// entries are swept. 0 applies a sensible default.
	MaxCacheEntries int
}

const defaultTTL = 5 * time.Minute
const defaultMaxCacheEntries = 4096

// tileEntry is one cached tile plus the wall-clock time it was fetched.
type tileEntry struct {
	data      []byte
	fetchedAt time.Time
}

// Service proxies and caches DWD radar tiles (WX-A, ADR 0016). It is best-effort:
// on any failure it serves the last-good tile if still cached, else a transparent
// tile — always HTTP 200 image/png, never an error to the browser and never a
// readiness concern.
type Service struct {
	client  *Client
	cfg     Config
	logger  *slog.Logger
	now     func() time.Time // injectable clock for deterministic tests
	ttl     time.Duration
	maxKeys int

	mu    sync.Mutex
	cache map[string]tileEntry

	fetchSuccess    atomic.Int64
	fetchFailure    atomic.Int64
	lastSuccessUnix atomic.Int64
}

// NewService builds a radar tile Service. A disabled Service (no URL) still
// serves transparent tiles so the frontend can request unconditionally.
func NewService(client *Client, cfg Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	maxKeys := cfg.MaxCacheEntries
	if maxKeys <= 0 {
		maxKeys = defaultMaxCacheEntries
	}
	return &Service{
		client:  client,
		cfg:     cfg,
		logger:  logger,
		now:     time.Now,
		ttl:     ttl,
		maxKeys: maxKeys,
		cache:   make(map[string]tileEntry),
	}
}

// TileHandler returns an http.HandlerFunc for GET
// /api/weather/radar/{z}/{x}/{y}. It reads the path wildcards, tolerates a
// trailing ".png" on {y}, and always responds with a PNG tile.
func (s *Service) TileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		z, zErr := strconv.Atoi(r.PathValue("z"))
		x, xErr := strconv.Atoi(r.PathValue("x"))
		// MapLibre requests ".../{y}.png"; the wildcard captures "y.png".
		y, yErr := strconv.Atoi(strings.TrimSuffix(r.PathValue("y"), ".png"))
		if zErr != nil || xErr != nil || yErr != nil {
			s.writeTransparent(w)
			return
		}
		s.serveTile(r.Context(), w, z, x, y)
	}
}

// serveTile is the cache-then-fetch core, separated from HTTP path parsing for
// direct testing.
func (s *Service) serveTile(ctx context.Context, w http.ResponseWriter, z, x, y int) {
	if !s.cfg.Enabled || !validTile(z, x, y) {
		s.writeTransparent(w)
		return
	}

	key := strconv.Itoa(z) + "/" + strconv.Itoa(x) + "/" + strconv.Itoa(y)

	// Fast path: a fresh cached tile.
	s.mu.Lock()
	if e, ok := s.cache[key]; ok && s.now().Sub(e.fetchedAt) < s.ttl {
		data := e.data
		s.mu.Unlock()
		s.writePNG(w, data)
		return
	}
	s.mu.Unlock()

	// Slow path: fetch upstream. On failure, fall back to the last-good cached
	// tile (even if stale) and finally to a transparent tile.
	data, err := s.client.FetchTile(ctx, z, x, y)
	if err != nil {
		s.fetchFailure.Add(1)
		s.logger.Debug("weather radar tile fetch failed; serving fallback",
			slog.Int("z", z), slog.Int("x", x), slog.Int("y", y), slog.String("error", err.Error()))
		s.mu.Lock()
		lastGood, ok := s.cache[key]
		s.mu.Unlock()
		if ok {
			s.writePNG(w, lastGood.data)
			return
		}
		s.writeTransparent(w)
		return
	}

	s.fetchSuccess.Add(1)
	s.lastSuccessUnix.Store(s.now().Unix())
	s.store(key, data)
	s.writePNG(w, data)
}

// store caches a tile and sweeps stale entries when the cache grows past its cap.
func (s *Service) store(key string, data []byte) {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = tileEntry{data: data, fetchedAt: now}
	if len(s.cache) > s.maxKeys {
		for k, e := range s.cache {
			if now.Sub(e.fetchedAt) >= s.ttl {
				delete(s.cache, k)
			}
		}
	}
}

func (s *Service) writePNG(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/png")
	// Let the browser cache a tile for the refresh window; radar updates ~5 min.
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(s.ttl.Seconds())))
	_, _ = w.Write(data)
}

// writeTransparent serves the 1×1 transparent fallback tile (still 200/image/png,
// but not cached long so a recovering upstream is picked up on the next request).
func (s *Service) writeTransparent(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(transparentTilePNG)
}

// FetchSuccessCount returns the number of successful upstream tile fetches.
func (s *Service) FetchSuccessCount() int64 { return s.fetchSuccess.Load() }

// FetchFailureCount returns the number of failed upstream tile fetches.
func (s *Service) FetchFailureCount() int64 { return s.fetchFailure.Load() }

// CacheAgeSeconds returns seconds since the last successful fetch, or -1 if there
// has never been one. Makes a staling radar overlay observable (ADR 0016).
func (s *Service) CacheAgeSeconds(now time.Time) int64 {
	last := s.lastSuccessUnix.Load()
	if last == 0 {
		return -1
	}
	return now.Unix() - last
}

// CachedTiles returns the current number of cached tiles (observability).
func (s *Service) CachedTiles() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cache)
}
