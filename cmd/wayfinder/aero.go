package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/aeronautical"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// aeroCacheStore adapts store.AeroCacheRepo to aeronautical.CacheStore (AERO-1,
// ADR 0018). It marshals the FeatureCollection to/from the TEXT column so the
// store stays free of the GeoJSON types and the aeronautical package stays free of
// the DB. A nil tenantID addresses the global fallback row.
type aeroCacheStore struct {
	repo *store.AeroCacheRepo
}

func newAeroCacheStore(repo *store.AeroCacheRepo) aeroCacheStore {
	return aeroCacheStore{repo: repo}
}

func (a aeroCacheStore) Load(ctx context.Context, tenantID *int64, kind aeronautical.Kind) (aeronautical.FeatureCollection, time.Time, bool, error) {
	e, ok, err := a.repo.Load(ctx, tenantID, string(kind))
	if err != nil || !ok {
		return aeronautical.FeatureCollection{}, time.Time{}, false, err
	}
	var fc aeronautical.FeatureCollection
	if err := json.Unmarshal([]byte(e.GeoJSON), &fc); err != nil {
		// A corrupt persisted row is treated as a load error: Hydrate logs and skips
		// it, so a bad blob never crashes the boot — the next fetch overwrites it.
		return aeronautical.FeatureCollection{}, time.Time{}, false, err
	}
	return fc, e.FetchedAt, true, nil
}

func (a aeroCacheStore) Save(ctx context.Context, tenantID *int64, kind aeronautical.Kind, fc aeronautical.FeatureCollection, change aeronautical.ChangeSummary, fetchedAt time.Time) error {
	b, err := json.Marshal(fc)
	if err != nil {
		return err
	}
	// Map the change summary to nullable columns: nil on the first fetch (no prior).
	var prev, added, removed *int
	if change.HasPrev {
		p, a2, r := change.PrevFeatureCount, change.Added, change.Removed
		prev, added, removed = &p, &a2, &r
	}
	return a.repo.Save(ctx, tenantID, string(kind), string(b), len(fc.Features), prev, added, removed, fetchedAt)
}

// AeroCacheStatus satisfies adminapi.AeroCacheStatusReader: the tenant's persisted
// cache freshness for the admin status route (AERO-1, ADR 0018).
func (a aeroCacheStore) AeroCacheStatus(ctx context.Context, tenantID int64) (*time.Time, int, bool, error) {
	tid := tenantID
	st, ok, err := a.repo.Status(ctx, &tid)
	if err != nil || !ok {
		return nil, 0, false, err
	}
	fetchedAt := st.FetchedAt
	return &fetchedAt, st.FeatureCount, true, nil
}

// TenantAeroCacheChanges satisfies adminapi.AeroChangesReader: the per-kind
// change-impact of the tenant's last OpenAIP refresh (AERO-3).
func (a aeroCacheStore) TenantAeroCacheChanges(ctx context.Context, tenantID int64) ([]store.AeroCacheChange, error) {
	tid := tenantID
	return a.repo.Changes(ctx, &tid)
}

// OpenAIP per tenant (ONB-6, ADR 0011). This file wires the aeronautical Registry
// into the rest of the process: a client factory (one OpenAIP client per key) and
// an adapter that resolves a tenant's effective key + area of interest and drives
// the registry. It keeps the per-tenant OpenAIP detail out of main.go and out of
// the admin API (which talks only to the adminapi.TenantAeroLifecycle interface).

// tenantAeroStore reads a tenant's per-tenant OpenAIP key (nil = use global) and
// lists tenants (for the AERO-2 fetch-all).
type tenantAeroStore interface {
	GetOpenAIPKey(ctx context.Context, id int64) (*string, error)
	List(ctx context.Context) ([]store.Tenant, error)
}

// tenantViewReader reads a tenant's default view, whose AOI/centre sets the
// OpenAIP query window for that tenant.
type tenantViewReader interface {
	GetTenantDefault(ctx context.Context, tenantID int64) (store.ViewConfig, error)
}

// newAeroClientFactory returns a ClientFactory that builds an OpenAIP client for a
// given key, sharing the configured base URL and a sensible HTTP timeout.
func newAeroClientFactory(baseURL string) aeronautical.ClientFactory {
	return func(apiKey string) *aeronautical.Client {
		return aeronautical.NewClient(&http.Client{Timeout: 15 * time.Second}, baseURL, apiKey)
	}
}

// tenantAeroLifecycle adapts the aeronautical Registry to
// adminapi.TenantAeroLifecycle. Apply resolves the tenant's effective key (its own
// key, else the global fallback) and area of interest (its view AOI, else a box
// around its view centre, else the global map-centre box) and (re)starts its
// Service; Stop drops it. The registry's Start is idempotent on unchanged inputs,
// so Apply is safe to call after every tenant view edit.
type tenantAeroLifecycle struct {
	reg     *aeronautical.Registry
	tenants tenantAeroStore
	views   tenantViewReader
	// globalKey returns the effective global fallback OpenAIP key (AERO-2, ADR 0018):
	// the runtime-set, sealed platform key when present, else the WAYFINDER_OPENAIP_API_KEY
	// env fallback. Read per resolve so a UI change takes effect without a restart.
	globalKey func(ctx context.Context) string
	radiusKM  float64
	fallback  aeronautical.BoundingBox // global map-centre box, used when a tenant has no view
	logger    *slog.Logger
}

// Apply (re)configures the tenant's per-tenant OpenAIP service with its effective
// key + area of interest. It does NOT force a fetch: the registry fetches only when
// the inputs changed (e.g. AOI moved) or nothing is persisted yet (AERO-1). Safe to
// call on every view edit and on boot (a redeploy just hydrates).
func (l tenantAeroLifecycle) Apply(ctx context.Context, tenantID int64) {
	key, bbox := l.resolve(ctx, tenantID)
	l.reg.Start(tenantID, key, bbox, false)
}

// Refresh forces a re-fetch of the tenant's OpenAIP data with its current key +
// AOI (AERO-1, ADR 0018) — the explicit "get fresh data now" path, used after a key
// change (and by the AERO-2 refresh buttons). A tenant without a key is a no-op
// (it falls back to the global cache).
func (l tenantAeroLifecycle) Refresh(ctx context.Context, tenantID int64) {
	key, bbox := l.resolve(ctx, tenantID)
	l.reg.Start(tenantID, key, bbox, true)
}

func (l tenantAeroLifecycle) Stop(tenantID int64) {
	l.reg.Stop(tenantID)
}

// RefreshAll forces a re-fetch for every tenant (AERO-2, ADR 0018) — used after the
// global key changes and by the "refresh all" admin button. Each tenant re-resolves
// its effective key (its own, else the new global) and force-fetches; the fetches
// run asynchronously in the registry, so this returns once they are all queued.
// Best-effort: a tenant-list error is logged, not fatal.
func (l tenantAeroLifecycle) RefreshAll(ctx context.Context) {
	tenants, err := l.tenants.List(ctx)
	if err != nil {
		l.logger.Warn("openaip refresh-all: list tenants failed", slog.String("error", err.Error()))
		return
	}
	for _, t := range tenants {
		l.Refresh(ctx, t.ID)
	}
	l.logger.Info("openaip refresh-all queued", slog.Int("tenants", len(tenants)))
}

// resolve computes the tenant's effective OpenAIP key (its own, else the global
// fallback) and query window (its view AOI, else a box around its view centre,
// else the global map-centre box).
func (l tenantAeroLifecycle) resolve(ctx context.Context, tenantID int64) (string, aeronautical.BoundingBox) {
	key := l.globalKey(ctx)
	if k, err := l.tenants.GetOpenAIPKey(ctx, tenantID); err != nil {
		l.logger.Warn("openaip resolve: read tenant key failed; using global key",
			slog.Int64("tenant_id", tenantID), slog.String("error", err.Error()))
	} else if k != nil && *k != "" {
		key = *k
	}

	bbox := l.fallback
	if vc, err := l.views.GetTenantDefault(ctx, tenantID); err == nil {
		bbox = aeroBBoxFromView(vc, l.radiusKM)
	}
	return key, bbox
}

// aeroBBoxFromView derives a tenant's OpenAIP query window: its explicit AOI box
// when set, otherwise a box around its view centre with the configured radius.
func aeroBBoxFromView(vc store.ViewConfig, radiusKM float64) aeronautical.BoundingBox {
	if a := vc.AOI; a != nil {
		return aeronautical.BoundingBox{MinLon: a.MinLon, MinLat: a.MinLat, MaxLon: a.MaxLon, MaxLat: a.MaxLat}
	}
	return aeronautical.BoundingBoxFromCenter(vc.CenterLat, vc.CenterLon, radiusKM)
}

// openaipGlobalSettingKey is the platform_settings row that holds the sealed global
// OpenAIP API key (AERO-2, ADR 0018). openaipGlobalAAD binds the ciphertext to its
// purpose so it cannot be authenticated under a different context.
const openaipGlobalSettingKey = "openaip_global_key"

var openaipGlobalAAD = []byte("openaip:global")

// settingsStore is the slice of store.SettingsRepo the global-key adapter needs.
type settingsStore interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

// secretCipher is the seal/open surface (satisfied by *secret.Cipher). nil = no
// WAYFINDER_SECRET_KEY configured, so the UI-set global key is unavailable.
type secretCipher interface {
	Seal(plaintext string, aad []byte) (string, error)
	Open(blob string, aad []byte) (string, error)
}

// globalOpenAIP manages the runtime, sealed global OpenAIP key (AERO-2, ADR 0018).
// The key is stored encrypted in platform_settings (never plaintext); the env
// WAYFINDER_OPENAIP_API_KEY is the keyless-deployment fallback. It implements the
// admin API's global-key surface and provides the effective-key reader the
// per-tenant lifecycle uses. Without a cipher (no WAYFINDER_SECRET_KEY) the UI path
// is unavailable — set/status report that, and the env fallback still applies.
type globalOpenAIP struct {
	settings settingsStore
	cipher   secretCipher // nil = encryption unavailable
	envKey   string       // WAYFINDER_OPENAIP_API_KEY fallback
	logger   *slog.Logger
}

func newGlobalOpenAIP(settings settingsStore, cipher secretCipher, envKey string, logger *slog.Logger) *globalOpenAIP {
	if logger == nil {
		logger = slog.Default()
	}
	return &globalOpenAIP{settings: settings, cipher: cipher, envKey: envKey, logger: logger}
}

// Available reports whether the UI-set global key is usable (a cipher is
// configured). When false the admin PUT route returns 503.
func (g *globalOpenAIP) Available() bool { return g.cipher != nil }

// Configured reports whether a global key is set through the UI (a sealed row
// exists). The env fallback is not "configured" here — it is a deployment default,
// not a UI-managed value.
func (g *globalOpenAIP) Configured(ctx context.Context) (bool, error) {
	_, ok, err := g.settings.Get(ctx, openaipGlobalSettingKey)
	return ok, err
}

// SetKey seals and stores the global key, or clears it when key is empty. Requires
// a cipher (guarded by the caller via Available). Never stores plaintext.
func (g *globalOpenAIP) SetKey(ctx context.Context, key string) error {
	if key == "" {
		return g.settings.Delete(ctx, openaipGlobalSettingKey)
	}
	if g.cipher == nil {
		return errors.New("openaip global key: encryption unavailable (set WAYFINDER_SECRET_KEY)")
	}
	blob, err := g.cipher.Seal(key, openaipGlobalAAD)
	if err != nil {
		return err
	}
	return g.settings.Set(ctx, openaipGlobalSettingKey, blob)
}

// effectiveKey returns the decrypted UI-set global key when present and readable,
// else the env fallback. Read per resolve so a UI change takes effect live. A
// decrypt error (e.g. WAYFINDER_SECRET_KEY rotated) is logged and falls back to env
// rather than dropping OpenAIP entirely.
func (g *globalOpenAIP) effectiveKey(ctx context.Context) string {
	if g.cipher != nil {
		if blob, ok, err := g.settings.Get(ctx, openaipGlobalSettingKey); err != nil {
			g.logger.Warn("openaip global key: read failed; using env fallback", slog.String("error", err.Error()))
		} else if ok {
			if pt, err := g.cipher.Open(blob, openaipGlobalAAD); err != nil {
				g.logger.Warn("openaip global key: decrypt failed (key rotated?); using env fallback", slog.String("error", err.Error()))
			} else {
				return pt
			}
		}
	}
	return g.envKey
}
