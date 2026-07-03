package main

import (
	"context"
	"encoding/json"
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

func (a aeroCacheStore) Save(ctx context.Context, tenantID *int64, kind aeronautical.Kind, fc aeronautical.FeatureCollection, fetchedAt time.Time) error {
	b, err := json.Marshal(fc)
	if err != nil {
		return err
	}
	return a.repo.Save(ctx, tenantID, string(kind), string(b), len(fc.Features), fetchedAt)
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

// OpenAIP per tenant (ONB-6, ADR 0011). This file wires the aeronautical Registry
// into the rest of the process: a client factory (one OpenAIP client per key) and
// an adapter that resolves a tenant's effective key + area of interest and drives
// the registry. It keeps the per-tenant OpenAIP detail out of main.go and out of
// the admin API (which talks only to the adminapi.TenantAeroLifecycle interface).

// tenantAeroKeyReader reads a tenant's per-tenant OpenAIP key (nil = use global).
type tenantAeroKeyReader interface {
	GetOpenAIPKey(ctx context.Context, id int64) (*string, error)
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
	reg       *aeronautical.Registry
	tenants   tenantAeroKeyReader
	views     tenantViewReader
	globalKey string
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

// resolve computes the tenant's effective OpenAIP key (its own, else the global
// fallback) and query window (its view AOI, else a box around its view centre,
// else the global map-centre box).
func (l tenantAeroLifecycle) resolve(ctx context.Context, tenantID int64) (string, aeronautical.BoundingBox) {
	key := l.globalKey
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
