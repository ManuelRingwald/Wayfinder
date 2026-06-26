package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/aeronautical"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

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

func (l tenantAeroLifecycle) Apply(ctx context.Context, tenantID int64) {
	key := l.globalKey
	if k, err := l.tenants.GetOpenAIPKey(ctx, tenantID); err != nil {
		l.logger.Warn("openaip apply: read tenant key failed; using global key",
			slog.Int64("tenant_id", tenantID), slog.String("error", err.Error()))
	} else if k != nil && *k != "" {
		key = *k
	}

	bbox := l.fallback
	if vc, err := l.views.GetTenantDefault(ctx, tenantID); err == nil {
		bbox = aeroBBoxFromView(vc, l.radiusKM)
	}

	l.reg.Start(tenantID, key, bbox)
}

func (l tenantAeroLifecycle) Stop(tenantID int64) {
	l.reg.Stop(tenantID)
}

// aeroBBoxFromView derives a tenant's OpenAIP query window: its explicit AOI box
// when set, otherwise a box around its view centre with the configured radius.
func aeroBBoxFromView(vc store.ViewConfig, radiusKM float64) aeronautical.BoundingBox {
	if a := vc.AOI; a != nil {
		return aeronautical.BoundingBox{MinLon: a.MinLon, MinLat: a.MinLat, MaxLon: a.MaxLon, MaxLat: a.MaxLat}
	}
	return aeronautical.BoundingBoxFromCenter(vc.CenterLat, vc.CenterLon, radiusKM)
}
