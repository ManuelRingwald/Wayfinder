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

	"github.com/manuelringwald/wayfinder/pkg/feature"
	"github.com/manuelringwald/wayfinder/pkg/sensorclass"
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

// EntitlementService is the per-tenant feature surface the admin API needs
// (satisfied by *feature.Service). Effective lists the full catalog with each
// key's state (default-deny); Set persists one flag, rejecting unknown keys
// (WF2-50). Setting entitlements is the billing/provisioning boundary, so the
// routes using it are super_admin-only.
type EntitlementService interface {
	Effective(ctx context.Context, tenantID int64) (map[feature.Key]bool, error)
	Set(ctx context.Context, tenantID int64, key feature.Key, enabled bool) error
	// HasFeature is the fail-closed gate used to enforce feed entitlements at the
	// grant edge (WF2-41): a tenant without multi_feed may hold at most one feed.
	HasFeature(ctx context.Context, tenantID int64, key feature.Key) bool
}

// RescopeFunc is invoked after a mutation that changes what a tenant's connected
// clients may see (a view edit, or a feed grant/revoke), so their live /ws streams
// are re-scoped in place without a reconnect (WF2-33). It is wired in main.go to
// the broadcaster; nil disables live-apply (clients then pick the change up on
// their next connect). It must not block on the request path beyond a quick
// resolve + enqueue.
type RescopeFunc func(ctx context.Context, tenantID int64)

// Handler routes the /api/admin/* endpoints.
type Handler struct {
	views   ViewStore
	subs    SubscriptionStore
	feeds   FeedStore
	tenants TenantStore
	feats   EntitlementService
	rescope RescopeFunc
	logger  *slog.Logger
	mux     *http.ServeMux
}

// New builds the admin API handler. Method+path patterns give automatic 405s for
// the wrong method. The cross-tenant provisioning routes (/api/admin/tenants/…)
// are additionally restricted to super_admin (requireSuper). rescope (may be nil)
// re-scopes a tenant's live streams after a mutation (WF2-33).
func New(views ViewStore, subs SubscriptionStore, feeds FeedStore, tenants TenantStore, feats EntitlementService, logger *slog.Logger, rescope RescopeFunc) *Handler {
	h := &Handler{views: views, subs: subs, feeds: feeds, tenants: tenants, feats: feats, rescope: rescope, logger: logger}
	mux := http.NewServeMux()
	// whoami: the SPA's role probe (WF2-32). It sits behind the same admin gate, so
	// a 200 here both confirms access and tells the client which panels to render.
	mux.HandleFunc("GET /api/admin/whoami", h.whoami)
	// tenant_admin self-service (tenant from the Identity).
	mux.HandleFunc("GET /api/admin/view", h.getView)
	mux.HandleFunc("PUT /api/admin/view", h.putView)
	mux.HandleFunc("GET /api/admin/subscriptions", h.getSubscriptions)
	mux.HandleFunc("GET /api/admin/feeds", h.getFeeds)
	// Read-only reference: the sensor-class catalogue (WF2-41), for the SPA to
	// render class pickers/legends. Any admin role may read it.
	mux.HandleFunc("GET /api/admin/sensor-classes", h.getSensorClasses)
	// super_admin provisioning (target tenant from the path, cross-tenant).
	mux.HandleFunc("GET /api/admin/tenants", h.requireSuper(h.listTenants))
	mux.HandleFunc("GET /api/admin/tenants/{tenantID}/subscriptions", h.requireSuper(h.listTenantSubscriptions))
	mux.HandleFunc("POST /api/admin/tenants/{tenantID}/subscriptions", h.requireSuper(h.grantSubscription))
	mux.HandleFunc("DELETE /api/admin/tenants/{tenantID}/subscriptions/{feedID}", h.requireSuper(h.revokeSubscription))
	// super_admin feature entitlements (WF2-50): list the full catalogue with the
	// target tenant's state, and set one flag. The billing/provisioning boundary.
	mux.HandleFunc("GET /api/admin/tenants/{tenantID}/entitlements", h.requireSuper(h.listTenantEntitlements))
	mux.HandleFunc("PUT /api/admin/tenants/{tenantID}/entitlements/{key}", h.requireSuper(h.setTenantEntitlement))
	h.mux = mux
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.mux.ServeHTTP(w, r) }

// whoamiDTO is the identity the SPA reads on entering /admin: it confirms access
// (the route is behind the admin gate), reports the role so the client knows
// whether to render the super_admin provisioning panel, and carries the tenant's
// effective feature flags (WF2-50) so the SPA can hide entitlement-gated UI. Both
// the role and the feature gating in the UI are cosmetic — the server enforces
// them independently (requireSuper → 403; feature gates checked server-side).
type whoamiDTO struct {
	Subject  string          `json:"subject"`
	TenantID int64           `json:"tenant_id"`
	UserID   int64           `json:"user_id"`
	Role     store.Role      `json:"role"`
	Features map[string]bool `json:"features"`
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
		Features: h.effectiveFeatures(r.Context(), id.TenantID),
	})
}

// effectiveFeatures returns the tenant's feature flags for the SPA to gate UI.
// Fail-closed: on a backend error the service returns an all-deny map (already
// logged), so the worst case is hiding a feature, never wrongly exposing one.
func (h *Handler) effectiveFeatures(ctx context.Context, tenantID int64) map[string]bool {
	if h.feats == nil {
		return map[string]bool{}
	}
	eff, _ := h.feats.Effective(ctx, tenantID)
	out := make(map[string]bool, len(eff))
	for k, v := range eff {
		out[string(k)] = v
	}
	return out
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
	h.triggerRescope(r.Context(), id.TenantID) // live-apply the new view (WF2-33)
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

// sensorClassDTO is one entry of the sensor-class catalogue (WF2-41).
type sensorClassDTO struct {
	Class       string `json:"class"`
	Description string `json:"description"`
}

func (h *Handler) getSensorClasses(w http.ResponseWriter, r *http.Request) {
	if _, ok := tenant.FromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	out := make([]sensorClassDTO, 0, len(sensorclass.All()))
	for _, c := range sensorclass.All() {
		out = append(out, sensorClassDTO{Class: string(c), Description: sensorclass.Describe(c)})
	}
	writeJSON(w, http.StatusOK, out)
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

// triggerRescope re-scopes the tenant's live /ws streams after a successful
// mutation (WF2-33). No-op when live-apply is not wired (rescope == nil), e.g.
// single-tenant mode or unit tests that don't exercise it.
func (h *Handler) triggerRescope(ctx context.Context, tenantID int64) {
	if h.rescope != nil {
		h.rescope(ctx, tenantID)
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
	// WF2-41: a tenant without the multi_feed entitlement may hold at most one
	// feed. Enforce the invariant here (fail-early) so the invalid state never
	// reaches the database — granting a *second distinct* feed needs the
	// entitlement. Re-granting a feed the tenant already has stays idempotent.
	existing, err := h.subs.ListFeedsByTenant(r.Context(), tid)
	if err != nil {
		h.internalError(w, "list subscriptions", err)
		return
	}
	alreadySubscribed := false
	for _, f := range existing {
		if f.ID == body.FeedID {
			alreadySubscribed = true
			break
		}
	}
	if !alreadySubscribed && len(existing) >= 1 && !h.feats.HasFeature(r.Context(), tid, feature.MultiFeed) {
		if h.logger != nil {
			h.logger.Warn("feed grant denied: multi_feed entitlement required",
				slog.Int64("tenant_id", tid), slog.Int64("feed_id", body.FeedID), slog.Int("current_feeds", len(existing)))
		}
		writeError(w, http.StatusConflict, "tenant lacks the multi_feed entitlement (at most one feed without it)")
		return
	}
	if err := h.subs.Subscribe(r.Context(), tid, body.FeedID); err != nil { // idempotent
		h.internalError(w, "grant subscription", err)
		return
	}
	h.triggerRescope(r.Context(), tid) // live-apply the new grant (WF2-33)
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
	h.triggerRescope(r.Context(), tid) // live-apply the revoke (WF2-33)
	w.WriteHeader(http.StatusNoContent)
}

// entitlementDTO is one feature flag in the admin entitlement view. The full
// catalogue is always returned so the UI can render every toggle, with enabled
// reflecting the tenant's state (default-deny for keys never set).
type entitlementDTO struct {
	Key         string `json:"key"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

func (h *Handler) listTenantEntitlements(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	eff, err := h.feats.Effective(r.Context(), tid)
	if err != nil {
		h.internalError(w, "list entitlements", err)
		return
	}
	// Present the whole catalogue in a stable order, not just stored rows, so the
	// UI shows every available feature with its (default-denied) state.
	out := make([]entitlementDTO, 0, len(feature.All()))
	for _, k := range feature.All() {
		out = append(out, entitlementDTO{Key: string(k), Enabled: eff[k], Description: feature.Describe(k)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setTenantEntitlement(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: expected {\"enabled\": <bool>}")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	// No live-apply rescope (unlike view/subscription edits): entitlements gate
	// feature availability, not the live track scope. The catalogue guard lives in
	// the service (Set → ErrUnknownFeature), which we surface as 400.
	switch err := h.feats.Set(r.Context(), tid, feature.Key(r.PathValue("key")), body.Enabled); {
	case errors.Is(err, feature.ErrUnknownFeature):
		writeError(w, http.StatusBadRequest, "unknown feature key")
	case err != nil:
		h.internalError(w, "set entitlement", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
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
