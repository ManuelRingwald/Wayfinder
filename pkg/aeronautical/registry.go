package aeronautical

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ClientFactory builds an OpenAIP Client for a given API key (ONB-6). It is
// injected so the Registry stays free of HTTP/transport detail: the caller
// (main.go) bakes in the shared http.Client timeout and base URL and varies only
// the per-tenant key.
type ClientFactory func(apiKey string) *Client

// TenantResolver extracts the tenant id from a request (set on the context by the
// tenant middleware). ok is false when there is no identity, in which case the
// per-tenant endpoints serve an empty collection (graceful, never an error).
type TenantResolver func(r *http.Request) (tenantID int64, ok bool)

// Registry runs one aeronautical Service per tenant (ONB-6, ADR 0011): each tenant
// fetches OpenAIP with its own API key (or the global fallback) against its own
// area of interest, and the read endpoints serve the cache of the requesting
// tenant. A tenant with no running service (e.g. no key configured) transparently
// falls back to the global Service, preserving the pre-ONB-6 single-cache
// behaviour. The lifecycle mirrors feedmanager: a mutex-guarded map of supervised
// goroutines, each driven by a per-tenant context derived from a shared base.
type Registry struct {
	base    context.Context
	global  *Service // fallback cache for tenants without their own running service
	factory ClientFactory
	refresh time.Duration
	logger  *slog.Logger

	mu       sync.Mutex
	services map[int64]*serviceHandle
}

// serviceHandle tracks one running per-tenant Service. apiKey and bbox are kept
// for idempotency: a Start with unchanged inputs is a no-op, so a rescope that
// did not move the AOI (e.g. a feed grant) does not needlessly restart the fetch.
type serviceHandle struct {
	svc    *Service
	apiKey string
	bbox   BoundingBox
	cancel context.CancelFunc
	done   chan struct{}
}

// NewRegistry creates a per-tenant Service registry. base is the parent context
// for every per-tenant refresh loop (cancelled on shutdown); global is the
// fallback Service (the existing process-global cache); factory builds a Client
// per key; refresh is the AIRAC-paced refresh interval applied to each service.
func NewRegistry(base context.Context, global *Service, factory ClientFactory, refresh time.Duration, logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		base:     base,
		global:   global,
		factory:  factory,
		refresh:  refresh,
		logger:   logger,
		services: make(map[int64]*serviceHandle),
	}
}

// Start (re)starts the per-tenant Service for tenantID with the given effective
// key and area of interest. It is idempotent on unchanged (apiKey, bbox): an
// identical call is a no-op, so it is safe to call from the live-apply path on
// every tenant mutation. An empty apiKey means the tenant has no own key and no
// global fallback exists — no service is run and any existing one is stopped, so
// the tenant transparently falls back to the global cache.
func (reg *Registry) Start(tenantID int64, apiKey string, bbox BoundingBox) {
	if apiKey == "" {
		reg.Stop(tenantID) // nothing to fetch with; fall back to global
		return
	}

	reg.mu.Lock()
	if h, ok := reg.services[tenantID]; ok {
		if h.apiKey == apiKey && h.bbox == bbox {
			reg.mu.Unlock()
			return // unchanged — idempotent no-op
		}
		// Inputs changed: stop the old loop, then start a fresh one below.
		delete(reg.services, tenantID)
		reg.mu.Unlock()
		h.cancel()
		<-h.done
		reg.mu.Lock()
	}

	svc := NewService(reg.factory(apiKey), Config{Enabled: true, BBox: bbox, Refresh: reg.refresh}, reg.logger)
	ctx, cancel := context.WithCancel(reg.base)
	done := make(chan struct{})
	reg.services[tenantID] = &serviceHandle{svc: svc, apiKey: apiKey, bbox: bbox, cancel: cancel, done: done}
	reg.mu.Unlock()

	go func() {
		defer close(done)
		svc.Run(ctx)
	}()
	reg.logger.Info("tenant aeronautical service started", slog.Int64("tenant_id", tenantID))
}

// Stop cancels and removes the per-tenant Service for tenantID, waiting for its
// goroutine to return. After Stop the tenant falls back to the global cache. It is
// a no-op (returns false) for a tenant that has no running service.
func (reg *Registry) Stop(tenantID int64) bool {
	reg.mu.Lock()
	h, ok := reg.services[tenantID]
	if ok {
		delete(reg.services, tenantID)
	}
	reg.mu.Unlock()
	if !ok {
		return false
	}
	h.cancel()
	<-h.done
	reg.logger.Info("tenant aeronautical service stopped", slog.Int64("tenant_id", tenantID))
	return true
}

// StopAll cancels every per-tenant Service and waits for all to return. Used on
// shutdown. Idempotent.
func (reg *Registry) StopAll() {
	reg.mu.Lock()
	handles := make([]*serviceHandle, 0, len(reg.services))
	for id, h := range reg.services {
		handles = append(handles, h)
		delete(reg.services, id)
	}
	reg.mu.Unlock()
	for _, h := range handles {
		h.cancel()
	}
	for _, h := range handles {
		<-h.done
	}
}

// IsRunning reports whether a per-tenant Service is currently running for
// tenantID (i.e. it has its own key and cache, not the global fallback).
func (reg *Registry) IsRunning(tenantID int64) bool {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	_, ok := reg.services[tenantID]
	return ok
}

// Serve returns the cached collection for one kind from the tenant's own Service,
// falling back to the global Service when the tenant has none running (no key
// configured, or not yet started). The fallback keeps the map populated for every
// authenticated tenant even before its first per-tenant refresh completes.
func (reg *Registry) Serve(tenantID int64, kind Kind) FeatureCollection {
	reg.mu.Lock()
	h := reg.services[tenantID]
	reg.mu.Unlock()
	if h != nil {
		return h.svc.Serve(kind)
	}
	if reg.global != nil {
		return reg.global.Serve(kind)
	}
	return EmptyCollection()
}

// Register mounts the per-tenant GeoJSON endpoints on mux, each wrapped by mw (the
// tenant middleware, so the handler sees an authenticated Identity) and resolving
// the tenant via tenantOf. Mirrors Service.Register but is tenant-aware (ONB-6).
func (reg *Registry) Register(mux *http.ServeMux, mw func(http.Handler) http.Handler, tenantOf TenantResolver) {
	for _, kind := range allKinds {
		mux.Handle(endpointPaths[kind], mw(reg.tenantHandler(kind, tenantOf)))
	}
}

// tenantHandler serves one kind for the requesting tenant. With no identity it
// returns an empty collection (graceful: the aeronautical layers are best-effort
// and must never surface as an error, ADR 0004).
func (reg *Registry) tenantHandler(kind Kind, tenantOf TenantResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tid, ok := tenantOf(r)
		if !ok {
			writeFeatureCollection(w, EmptyCollection())
			return
		}
		writeFeatureCollection(w, reg.Serve(tid, kind))
	}
}

// FetchSuccessCount aggregates successful fetches across every per-tenant Service
// and the global one, so the process-wide /metrics counter stays meaningful as
// tenants come and go (ONB-6). FetchFailureCount is the analogous sum.
func (reg *Registry) FetchSuccessCount() int64 {
	return reg.sum((*Service).FetchSuccessCount)
}

// FetchFailureCount aggregates failed fetches across all services (see above).
func (reg *Registry) FetchFailureCount() int64 {
	return reg.sum((*Service).FetchFailureCount)
}

func (reg *Registry) sum(get func(*Service) int64) int64 {
	var total int64
	if reg.global != nil {
		total += get(reg.global)
	}
	reg.mu.Lock()
	for _, h := range reg.services {
		total += get(h.svc)
	}
	reg.mu.Unlock()
	return total
}
