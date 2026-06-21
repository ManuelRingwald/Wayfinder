// Package adminapi serves the tenant-scoped admin REST API (WF2-31): a tenant
// administrator reads and edits *their own* tenant's configuration. Every handler
// derives the tenant from the request Identity (set by the tenant middleware) —
// never from the path or body — so a tenant admin can only ever touch their own
// tenant (isolation by construction, NFR-SEC-003). The routes are mounted behind
// RequireRole(tenant_admin, super_admin); this package assumes the caller is
// already authorised.
package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// ViewStore, SubscriptionStore, FeedStore and TenantStore are the slices of the
// store repos the API needs (small interfaces so handlers are unit-testable with
// fakes).
type ViewStore interface {
	GetEffective(ctx context.Context, tenantID, userID int64) (store.ViewConfig, error)
	UpsertTenantDefault(ctx context.Context, tenantID int64, vc store.ViewConfig) (store.ViewConfig, error)
}

type SubscriptionStore interface {
	ListFeedsByTenant(ctx context.Context, tenantID int64) ([]store.Feed, error)
	Subscribe(ctx context.Context, tenantID, feedID int64) error
	Unsubscribe(ctx context.Context, tenantID, feedID int64) error
}

type FeedStore interface {
	List(ctx context.Context) ([]store.Feed, error)
	GetByID(ctx context.Context, id int64) (store.Feed, error)
}

type TenantStore interface {
	List(ctx context.Context) ([]store.Tenant, error)
	GetByID(ctx context.Context, id int64) (store.Tenant, error)
}

// Handler routes the /api/admin/* endpoints.
type Handler struct {
	views   ViewStore
	subs    SubscriptionStore
	feeds   FeedStore
	tenants TenantStore
	logger  *slog.Logger
	mux     *http.ServeMux
}

// New builds the admin API handler. Method+path patterns give automatic 405s for
// the wrong method. The cross-tenant provisioning routes (/api/admin/tenants/…)
// are additionally restricted to super_admin (requireSuper).
func New(views ViewStore, subs SubscriptionStore, feeds FeedStore, tenants TenantStore, logger *slog.Logger) *Handler {
	h := &Handler{views: views, subs: subs, feeds: feeds, tenants: tenants, logger: logger}
	mux := http.NewServeMux()
	// whoami: the SPA's role probe (WF2-32). It sits behind the same admin gate, so
	// a 200 here both confirms access and tells the client which panels to render.
	mux.HandleFunc("GET /api/admin/whoami", h.whoami)
	// tenant_admin self-service (tenant from the Identity).
	mux.HandleFunc("GET /api/admin/view", h.getView)
	mux.HandleFunc("PUT /api/admin/view", h.putView)
	mux.HandleFunc("GET /api/admin/subscriptions", h.getSubscriptions)
	mux.HandleFunc("GET /api/admin/feeds", h.getFeeds)
	// super_admin provisioning (target tenant from the path, cross-tenant).
	mux.HandleFunc("GET /api/admin/tenants", h.requireSuper(h.listTenants))
	mux.HandleFunc("GET /api/admin/tenants/{tenantID}/subscriptions", h.requireSuper(h.listTenantSubscriptions))
	mux.HandleFunc("POST /api/admin/tenants/{tenantID}/subscriptions", h.requireSuper(h.grantSubscription))
	mux.HandleFunc("DELETE /api/admin/tenants/{tenantID}/subscriptions/{feedID}", h.requireSuper(h.revokeSubscription))
	h.mux = mux
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.mux.ServeHTTP(w, r) }

// whoamiDTO is the identity the SPA reads on entering /admin: it confirms access
// (the route is behind the admin gate) and reports the role so the client knows
// whether to render the super_admin provisioning panel. The role gating in the UI
// is cosmetic — the server independently enforces it (requireSuper → 403).
type whoamiDTO struct {
	Subject  string     `json:"subject"`
	TenantID int64      `json:"tenant_id"`
	UserID   int64      `json:"user_id"`
	Role     store.Role `json:"role"`
}

func (h *Handler) whoami(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, whoamiDTO{
		Subject:  id.Subject,
		TenantID: id.TenantID,
		UserID:   id.UserID,
		Role:     id.Role,
	})
}

// viewDTO is the JSON shape of a tenant's view configuration.
type viewDTO struct {
	CenterLat float64         `json:"center_lat"`
	CenterLon float64         `json:"center_lon"`
	Zoom      float64         `json:"zoom"`
	AOI       *store.BBox     `json:"aoi,omitempty"`
	FLMin     *int            `json:"fl_min,omitempty"`
	FLMax     *int            `json:"fl_max,omitempty"`
	Layers    map[string]bool `json:"layers,omitempty"`
}

// feedDTO is the catalogue-facing shape of a feed (infra fields like the
// multicast group/port are intentionally omitted from the admin surface).
type feedDTO struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Region    *string  `json:"region,omitempty"`
	SensorMix []string `json:"sensor_mix"`
}

func (h *Handler) getView(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	vc, err := h.views.GetEffective(r.Context(), id.TenantID, id.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no view configured")
		return
	}
	if err != nil {
		h.internalError(w, "get view", err)
		return
	}
	writeJSON(w, http.StatusOK, toViewDTO(vc))
}

func (h *Handler) putView(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var d viewDTO
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateView(d); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.views.UpsertTenantDefault(r.Context(), id.TenantID, toViewConfig(d))
	if err != nil {
		h.internalError(w, "upsert view", err)
		return
	}
	writeJSON(w, http.StatusOK, toViewDTO(out))
}

func (h *Handler) getSubscriptions(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	feeds, err := h.subs.ListFeedsByTenant(r.Context(), id.TenantID)
	if err != nil {
		h.internalError(w, "list subscriptions", err)
		return
	}
	writeJSON(w, http.StatusOK, toFeedDTOs(feeds))
}

func (h *Handler) getFeeds(w http.ResponseWriter, r *http.Request) {
	if _, ok := tenant.FromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	feeds, err := h.feeds.List(r.Context())
	if err != nil {
		h.internalError(w, "list feeds", err)
		return
	}
	writeJSON(w, http.StatusOK, toFeedDTOs(feeds))
}

// requireSuper restricts a handler to super_admin. The outer gate already lets
// only tenant_admin/super_admin through; this is the extra step that gates the
// **cross-tenant** provisioning routes — super_admin is the only role allowed to
// grant/revoke another tenant's feed access (the billing/entitlement boundary).
func (h *Handler) requireSuper(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := tenant.FromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if id.Role != store.RoleSuperAdmin {
			writeError(w, http.StatusForbidden, "super_admin required")
			return
		}
		next(w, r)
	}
}

func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request) {
	ts, err := h.tenants.List(r.Context())
	if err != nil {
		h.internalError(w, "list tenants", err)
		return
	}
	writeJSON(w, http.StatusOK, toTenantDTOs(ts))
}

func (h *Handler) listTenantSubscriptions(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	feeds, err := h.subs.ListFeedsByTenant(r.Context(), tid)
	if err != nil {
		h.internalError(w, "list tenant subscriptions", err)
		return
	}
	writeJSON(w, http.StatusOK, toFeedDTOs(feeds))
}

func (h *Handler) grantSubscription(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	var body struct {
		FeedID int64 `json:"feed_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || body.FeedID == 0 {
		writeError(w, http.StatusBadRequest, "invalid body: expected {\"feed_id\": <id>}")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	if _, err := h.feeds.GetByID(r.Context(), body.FeedID); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "feed not found")
		return
	} else if err != nil {
		h.internalError(w, "get feed", err)
		return
	}
	if err := h.subs.Subscribe(r.Context(), tid, body.FeedID); err != nil { // idempotent
		h.internalError(w, "grant subscription", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) revokeSubscription(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	fid, err := pathInt(r, "feedID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feed id")
		return
	}
	if err := h.subs.Unsubscribe(r.Context(), tid, fid); err != nil { // idempotent
		h.internalError(w, "revoke subscription", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// tenantExists writes a 404 (or 500) and returns false if the target tenant does
// not exist; callers stop on false.
func (h *Handler) tenantExists(w http.ResponseWriter, r *http.Request, tenantID int64) bool {
	_, err := h.tenants.GetByID(r.Context(), tenantID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tenant not found")
		return false
	}
	if err != nil {
		h.internalError(w, "get tenant", err)
		return false
	}
	return true
}

func pathInt(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

// validateView enforces the geographic/flight-level invariants server-side so a
// client can never store an out-of-range or inverted view.
func validateView(d viewDTO) error {
	switch {
	case d.CenterLat < -90 || d.CenterLat > 90:
		return errors.New("center_lat out of range [-90,90]")
	case d.CenterLon < -180 || d.CenterLon > 180:
		return errors.New("center_lon out of range [-180,180]")
	case d.Zoom < 0 || d.Zoom > 24:
		return errors.New("zoom out of range [0,24]")
	}
	if a := d.AOI; a != nil {
		if a.MinLat < -90 || a.MaxLat > 90 || a.MinLon < -180 || a.MaxLon > 180 {
			return errors.New("aoi out of range")
		}
		if a.MinLat > a.MaxLat || a.MinLon > a.MaxLon {
			return errors.New("aoi min must be <= max")
		}
	}
	if d.FLMin != nil && *d.FLMin < 0 {
		return errors.New("fl_min must be >= 0")
	}
	if d.FLMax != nil && *d.FLMax < 0 {
		return errors.New("fl_max must be >= 0")
	}
	if d.FLMin != nil && d.FLMax != nil && *d.FLMin > *d.FLMax {
		return errors.New("fl_min must be <= fl_max")
	}
	return nil
}

func toViewConfig(d viewDTO) store.ViewConfig {
	return store.ViewConfig{
		CenterLat: d.CenterLat, CenterLon: d.CenterLon, Zoom: d.Zoom,
		AOI: d.AOI, FLMin: d.FLMin, FLMax: d.FLMax, Layers: d.Layers,
	}
}

func toViewDTO(vc store.ViewConfig) viewDTO {
	return viewDTO{
		CenterLat: vc.CenterLat, CenterLon: vc.CenterLon, Zoom: vc.Zoom,
		AOI: vc.AOI, FLMin: vc.FLMin, FLMax: vc.FLMax, Layers: vc.Layers,
	}
}

func toFeedDTOs(feeds []store.Feed) []feedDTO {
	out := make([]feedDTO, len(feeds))
	for i, f := range feeds {
		out[i] = feedDTO{ID: f.ID, Name: f.Name, Region: f.Region, SensorMix: f.SensorMix}
	}
	return out
}

// tenantDTO is the admin-facing shape of a tenant (super_admin provisioning view).
type tenantDTO struct {
	ID     int64  `json:"id"`
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func toTenantDTOs(tenants []store.Tenant) []tenantDTO {
	out := make([]tenantDTO, len(tenants))
	for i, t := range tenants {
		out[i] = tenantDTO{ID: t.ID, Slug: t.Slug, Name: t.Name, Status: t.Status}
	}
	return out
}

func (h *Handler) internalError(w http.ResponseWriter, op string, err error) {
	if h.logger != nil {
		h.logger.Error("admin api", slog.String("op", op), slog.String("error", err.Error()))
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
