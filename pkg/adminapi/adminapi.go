// Package adminapi serves the tenant-scoped admin REST API (WF2-31): an admin
// reads and edits configuration for their own tenant (and, via the cross-tenant
// provisioning routes, any other tenant). Every handler derives the tenant from
// the request Identity (set by the tenant middleware) — never from the path or
// body — so self-service routes are isolated by construction (NFR-SEC-003).
// The routes are mounted behind RequireRole(admin); this package assumes the
// caller is already authorised.
package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/feature"
	"github.com/manuelringwald/wayfinder/pkg/health"
	"github.com/manuelringwald/wayfinder/pkg/sensorclass"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// ViewStore, SubscriptionStore, FeedStore and TenantStore are the slices of the
// store repos the API needs (small interfaces so handlers are unit-testable with
// fakes).
type ViewStore interface {
	GetEffective(ctx context.Context, tenantID, userID int64) (store.ViewConfig, error)
	// GetTenantDefault reads the tenant's default view (no user override), used by
	// the cross-tenant admin view editor (AP3) where there is no acting user whose
	// override should apply.
	GetTenantDefault(ctx context.Context, tenantID int64) (store.ViewConfig, error)
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
	// GetByName backs the duplicate-name pre-check on create (ONB-5); ErrNotFound
	// means the name is free.
	GetByName(ctx context.Context, name string) (store.Feed, error)
	Create(ctx context.Context, name, multicastGroup string, port int, region *string, sensorMix []string) (store.Feed, error)
	// CreateAutoAllocated inserts a feed on the next free multicast endpoint from
	// the configured pool (ORCH-4) — used when the admin omits group/port. Returns
	// store.ErrPoolExhausted when the pool is full.
	CreateAutoAllocated(ctx context.Context, name string, region *string, sensorMix []string) (store.Feed, error)
	Delete(ctx context.Context, id int64) error
	// GetSourceConfig/SetSourceConfig back the feed's generic source configuration
	// and derived coverage bbox (ORCH-1, ADR 0012); a missing feed yields
	// ErrNotFound. SetSourceConfig validates the config (errors.As *InvalidSourceError).
	GetSourceConfig(ctx context.Context, id int64) (store.SourceConfig, *store.BBox, error)
	SetSourceConfig(ctx context.Context, id int64, sources store.SourceConfig, coverage *store.BBox) error
}

// SecretService is the write-only credential surface for a feed's source
// configuration (ORCH-2c 3a, ADR 0012 §6). The admin API accepts a plaintext
// credential for a cred_ref and hands it to SetSecret, which seals it with the
// deployment key before it is stored; the value is never read back to the browser
// (ListSecretRefs reports only which refs are configured). Satisfied in production
// by *orchestrator.SecretSealer. The interface keeps the admin API crypto-agnostic
// (it never holds the key). A nil service disables the secret routes (503) — the
// degenerate case when WAYFINDER_SECRET_KEY is unset (dev/single-tenant).
type SecretService interface {
	SetSecret(ctx context.Context, feedID int64, credRef, plaintext string) error
	DeleteSecret(ctx context.Context, feedID int64, credRef string) error
	ListSecretRefs(ctx context.Context, feedID int64) ([]string, error)
}

// FeedLifecycle starts and stops the live multicast receiver for a feed so a
// catalogue change takes effect without a process restart (ONB-5, ADR 0011).
// Satisfied in production by an adapter over the feed manager + health registry
// (main.go). It is intentionally decoupled from the concrete receiver/manager
// types (primitive params) so the admin API never imports the transport layer.
// A nil lifecycle disables live-apply: the catalogue still changes, but the
// running receiver set only follows on the next restart (single-tenant mode and
// unit tests that do not exercise the live path).
type FeedLifecycle interface {
	// Start joins the multicast group for a newly catalogued feed. An error means
	// the group could not be joined; the create handler then rolls the catalogue
	// row back so a feed is never left half-created (catalogued but not receiving).
	Start(id int64, name, group string, port int) error
	// Stop leaves the multicast group for a deleted feed and releases its derived
	// per-feed state (e.g. health). Returns whether a receiver was running.
	Stop(id int64) bool
}

type TenantStore interface {
	List(ctx context.Context) ([]store.Tenant, error)
	GetByID(ctx context.Context, id int64) (store.Tenant, error)
	GetBySlug(ctx context.Context, slug string) (store.Tenant, error)
	Create(ctx context.Context, slug, name string) (store.Tenant, error)
	SetStatus(ctx context.Context, id int64, status store.Status) error
	Delete(ctx context.Context, id int64) error
	// GetOpenAIPKey/SetOpenAIPKey back the per-tenant OpenAIP key (ONB-6). The key
	// is read in isolation and never returned to the browser (only its presence is).
	GetOpenAIPKey(ctx context.Context, id int64) (*string, error)
	SetOpenAIPKey(ctx context.Context, id int64, key *string) error
}

// TenantAeroLifecycle applies a tenant's per-tenant OpenAIP aeronautical refresh
// when its key — or, via its view, its area of interest — changes, and tears it
// down when the tenant is deleted (ONB-6, ADR 0011). Satisfied in production by an
// adapter over the aeronautical Registry (main.go). Apply re-reads the tenant's
// effective key and AOI and (re)starts/stops its Service; it is decoupled from the
// concrete registry (the admin API never imports the OpenAIP transport). A nil
// lifecycle disables live-apply: the stored key still changes, but the running
// per-tenant fetch only follows on the next restart (single-tenant mode / tests).
type TenantAeroLifecycle interface {
	Apply(ctx context.Context, tenantID int64)
	Stop(tenantID int64)
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

// FeedHealthSource provides per-feed health snapshots for the admin dashboard
// (AP4). Satisfied by *health.Registry in production; a nil source returns an
// empty list for the /api/admin/feeds/health endpoint (graceful degradation when
// the health registry is unavailable, e.g. in integration tests that only test
// the DB paths).
type FeedHealthSource interface {
	Snapshot(feedID int64, now time.Time) health.FeedSnapshot
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
	views      ViewStore
	subs       SubscriptionStore
	feeds      FeedStore
	tenants    TenantStore
	users      UserStore
	creds      CredentialStore
	feats      EntitlementService
	feedHealth FeedHealthSource    // may be nil; AP4 health endpoint returns empty list
	feedLife   FeedLifecycle       // may be nil; disables live receiver join/leave (ONB-5)
	aeroLife   TenantAeroLifecycle // may be nil; disables live per-tenant OpenAIP apply (ONB-6)
	secrets    SecretService       // may be nil; disables the per-feed secret routes (503) when no key is configured (ORCH-2c 3a)
	rescope    RescopeFunc
	logger     *slog.Logger
	mux        *http.ServeMux
}

// New builds the admin API handler. Method+path patterns give automatic 405s for
// the wrong method. The cross-tenant provisioning routes (/api/admin/tenants/…)
// are additionally guarded by requireAdmin (defence-in-depth). rescope (may be
// nil) re-scopes a tenant's live streams after a mutation (WF2-33).
func New(views ViewStore, subs SubscriptionStore, feeds FeedStore, tenants TenantStore, users UserStore, creds CredentialStore, feats EntitlementService, feedHealth FeedHealthSource, feedLife FeedLifecycle, aeroLife TenantAeroLifecycle, secrets SecretService, logger *slog.Logger, rescope RescopeFunc) *Handler {
	h := &Handler{views: views, subs: subs, feeds: feeds, tenants: tenants, users: users, creds: creds, feats: feats, feedHealth: feedHealth, feedLife: feedLife, aeroLife: aeroLife, secrets: secrets, rescope: rescope, logger: logger}
	mux := http.NewServeMux()
	// whoami: the SPA's role probe (WF2-32). It sits behind the same admin gate, so
	// a 200 here both confirms access and tells the client which panels to render.
	mux.HandleFunc("GET /api/admin/whoami", h.whoami)
	// Account self-service (ONB-1, ADR 0011): the logged-in principal manages its
	// own account — independent of role (no requireAdmin). These three routes are
	// the only ones reachable while must_change_password is set (see the gate in
	// ServeHTTP); changing the password is what clears the flag and unlocks the rest.
	mux.HandleFunc("GET /api/admin/me", h.getMe)
	mux.HandleFunc("PUT /api/admin/me/password", h.putMePassword)
	mux.HandleFunc("DELETE /api/admin/me", h.deleteMe)
	// Admin self-service (tenant from the Identity).
	mux.HandleFunc("GET /api/admin/view", h.getView)
	mux.HandleFunc("PUT /api/admin/view", h.putView)
	mux.HandleFunc("GET /api/admin/subscriptions", h.getSubscriptions)
	mux.HandleFunc("GET /api/admin/feeds", h.getFeeds)
	// Feed lifecycle (ONB-5, ADR 0011): create and delete catalogue feeds from the
	// UI; the live multicast receiver joins/leaves at once (no restart). Both are
	// platform operations → requireAdmin. The more specific {feedID} delete pattern
	// is distinct from the GET /api/admin/feeds/health route registered below.
	mux.HandleFunc("POST /api/admin/feeds", h.requireAdmin(h.createFeed))
	mux.HandleFunc("DELETE /api/admin/feeds/{feedID}", h.requireAdmin(h.deleteFeed))
	// Feed source configuration (ORCH-1b, ADR 0012): read/write the generic source
	// list + derived coverage bbox that the orchestrator will turn into a Firefly
	// instance. Platform operation → requireAdmin. The {feedID}/sources pattern is
	// distinct from the {feedID} delete and the feeds/health route.
	mux.HandleFunc("GET /api/admin/feeds/{feedID}/sources", h.requireAdmin(h.getFeedSources))
	mux.HandleFunc("PUT /api/admin/feeds/{feedID}/sources", h.requireAdmin(h.putFeedSources))
	// Per-feed source credentials (ORCH-2c 3a, ADR 0012 §6): write-only secret
	// management. The value referenced by a source's cred_ref is set/cleared here
	// and sealed at rest; the GET reports only which refs are configured, never a
	// value (mirrors the OpenAIP key isolation, ONB-6). The {ref...} trailing
	// wildcard lets a cred_ref carry slashes (e.g. "secret/opensky").
	mux.HandleFunc("GET /api/admin/feeds/{feedID}/secrets", h.requireAdmin(h.getFeedSecrets))
	mux.HandleFunc("PUT /api/admin/feeds/{feedID}/secrets/{ref...}", h.requireAdmin(h.putFeedSecret))
	mux.HandleFunc("DELETE /api/admin/feeds/{feedID}/secrets/{ref...}", h.requireAdmin(h.deleteFeedSecret))
	// Read-only reference: the sensor-class catalogue (WF2-41).
	mux.HandleFunc("GET /api/admin/sensor-classes", h.getSensorClasses)
	// Cross-tenant provisioning (target tenant from the path).
	mux.HandleFunc("GET /api/admin/tenants", h.requireAdmin(h.listTenants))
	// Tenant lifecycle (ONB-4, ADR 0011): create and delete tenants from the UI.
	// Delete cascades to the tenant's dependents (users, subscriptions,
	// entitlements, views) but is refused (409) while the tenant still has
	// accounts — the destructive cascade must be a conscious two-step.
	mux.HandleFunc("POST /api/admin/tenants", h.requireAdmin(h.createTenant))
	mux.HandleFunc("DELETE /api/admin/tenants/{tenantID}", h.requireAdmin(h.deleteTenant))
	// AP3: tenant-centric admin dashboard. The overview aggregates each tenant's
	// status, enabled features, subscribed feeds and account count in one call
	// (avoids N+1 fetches when rendering the tenant table). Per-tenant view
	// get/put let an admin edit *any* tenant's default view (the self-service
	// /api/admin/view routes only touch the caller's own tenant).
	mux.HandleFunc("GET /api/admin/overview", h.requireAdmin(h.getOverview))
	// AP4: per-feed health state (heartbeat staleness + track presence) for the
	// admin dashboard's feed-health colour chips.
	mux.HandleFunc("GET /api/admin/feeds/health", h.requireAdmin(h.getFeedsHealth))
	mux.HandleFunc("GET /api/admin/tenants/{tenantID}/view", h.requireAdmin(h.getTenantView))
	mux.HandleFunc("PUT /api/admin/tenants/{tenantID}/view", h.requireAdmin(h.putTenantView))
	mux.HandleFunc("GET /api/admin/tenants/{tenantID}/subscriptions", h.requireAdmin(h.listTenantSubscriptions))
	mux.HandleFunc("POST /api/admin/tenants/{tenantID}/subscriptions", h.requireAdmin(h.grantSubscription))
	mux.HandleFunc("DELETE /api/admin/tenants/{tenantID}/subscriptions/{feedID}", h.requireAdmin(h.revokeSubscription))
	// Feature entitlements (WF2-50): list the full catalogue with the target
	// tenant's state, and set one flag. The billing/provisioning boundary.
	mux.HandleFunc("GET /api/admin/tenants/{tenantID}/entitlements", h.requireAdmin(h.listTenantEntitlements))
	mux.HandleFunc("PUT /api/admin/tenants/{tenantID}/entitlements/{key}", h.requireAdmin(h.setTenantEntitlement))
	// OpenAIP per tenant (ONB-6, ADR 0011): read whether a per-tenant key is set
	// (never the key itself) and set/clear it. Setting the key (re)starts the
	// tenant's per-tenant OpenAIP refresh live (no restart).
	mux.HandleFunc("GET /api/admin/tenants/{tenantID}/openaip", h.requireAdmin(h.getTenantOpenAIP))
	mux.HandleFunc("PUT /api/admin/tenants/{tenantID}/openaip", h.requireAdmin(h.setTenantOpenAIP))
	// Platform-admin management (ONB-3, ADR 0011): admins are global (no tenant)
	// and managed through a dedicated surface, strictly separated from the
	// per-tenant user routes below. The "last active admin" guard (409) lives in
	// the delete/pause handlers. Cross-cutting platform operation → requireAdmin.
	mux.HandleFunc("GET /api/admin/admins", h.requireAdmin(h.listAdmins))
	mux.HandleFunc("POST /api/admin/admins", h.requireAdmin(h.createAdmin))
	mux.HandleFunc("PATCH /api/admin/admins/{adminID}", h.requireAdmin(h.setAdminStatus))
	mux.HandleFunc("DELETE /api/admin/admins/{adminID}", h.requireAdmin(h.deleteAdmin))
	mux.HandleFunc("PUT /api/admin/admins/{adminID}/password", h.requireAdmin(h.setAdminPassword))
	// Access management (AP6): an admin provisions and suspends access accounts
	// per tenant, and pauses a whole tenant. Cross-tenant → requireAdmin.
	mux.HandleFunc("PATCH /api/admin/tenants/{tenantID}", h.requireAdmin(h.setTenantStatus))
	mux.HandleFunc("GET /api/admin/tenants/{tenantID}/users", h.requireAdmin(h.listUsers))
	mux.HandleFunc("POST /api/admin/tenants/{tenantID}/users", h.requireAdmin(h.createUser))
	mux.HandleFunc("PATCH /api/admin/tenants/{tenantID}/users/{userID}", h.requireAdmin(h.setUserStatus))
	mux.HandleFunc("DELETE /api/admin/tenants/{tenantID}/users/{userID}", h.requireAdmin(h.deleteUser))
	mux.HandleFunc("PUT /api/admin/tenants/{tenantID}/users/{userID}/password", h.requireAdmin(h.setUserPassword))
	h.mux = mux
	return h
}

// passwordChangeAllowlist is the exact set of (method, path) pairs reachable while
// a principal's must_change_password flag is set (ONB-1, ADR 0011). Everything
// else is refused fail-closed so the known default credential cannot be used for
// anything but its own replacement: the role probe (so the SPA can render the
// forced-change mask), reading one's own account, and the password change itself.
var passwordChangeAllowlist = map[string]string{
	"/api/admin/whoami":      http.MethodGet,
	"/api/admin/me":          http.MethodGet,
	"/api/admin/me/password": http.MethodPut,
}

// ServeHTTP applies the must_change_password gate before dispatching. The flag is
// carried on the Identity (resolved by the tenant middleware), so the gate needs
// no database lookup. A gated request gets 403 with a stable marker the SPA keys
// on to show the forced-change mask.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if id, ok := tenant.FromContext(r.Context()); ok && id.MustChangePassword {
		if m, allowed := passwordChangeAllowlist[r.URL.Path]; !allowed || m != r.Method {
			writeError(w, http.StatusForbidden, "password_change_required")
			return
		}
	}
	h.mux.ServeHTTP(w, r)
}

// whoamiDTO is the identity the SPA reads on entering /admin: it confirms access
// (the route is behind the admin gate), reports the role so the client can render
// the correct panels, and carries the tenant's effective feature flags (WF2-50)
// so the SPA can hide entitlement-gated UI. Both the role and the feature gating
// in the UI are cosmetic — the server enforces them independently.
type whoamiDTO struct {
	Subject            string          `json:"subject"`
	TenantID           int64           `json:"tenant_id"`
	UserID             int64           `json:"user_id"`
	Role               store.Role      `json:"role"`
	MustChangePassword bool            `json:"must_change_password"`
	Features           map[string]bool `json:"features"`
}

// WhoamiHandler exposes the identity probe for mounting OUTSIDE the requireAdmin
// gate — at GET /api/whoami, behind the tenant middleware only. It lets ANY
// authenticated principal (including a plain tenant user) resolve its own session
// and role, which the ASD map needs to decide between its login screen and the
// live picture. 401 when no Identity is present. The admin SPA keeps using the
// admin-gated GET /api/admin/whoami; both share the same handler/DTO.
func (h *Handler) WhoamiHandler() http.HandlerFunc { return h.whoami }

func (h *Handler) whoami(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, whoamiDTO{
		Subject:            id.Subject,
		TenantID:           id.TenantID,
		UserID:             id.UserID,
		Role:               id.Role,
		MustChangePassword: id.MustChangePassword,
		Features:           h.effectiveFeatures(r.Context(), id.TenantID),
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

// feedDTO is the catalogue-facing shape of a feed. The multicast group/port are
// the feed's wire coordinates; they are surfaced (ONB-5) because the feed-lifecycle
// UI manages exactly those, and every /api/admin route is platform-admin only —
// the coordinates are operational, not secret.
type feedDTO struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	MulticastGroup string   `json:"multicast_group"`
	Port           int      `json:"port"`
	Region         *string  `json:"region,omitempty"`
	SensorMix      []string `json:"sensor_mix"`
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
	h.triggerRescope(r.Context(), id.TenantID)   // live-apply the new view (WF2-33)
	h.triggerAeroApply(r.Context(), id.TenantID) // re-fetch OpenAIP for the new AOI (ONB-6)
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

// requireAdmin is a defence-in-depth guard for cross-tenant provisioning routes.
// The outer gate already ensures only an admin reaches this handler; this inner
// check ensures no future refactoring accidentally exposes these routes to
// non-admin identities.
func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := tenant.FromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if id.Role != store.RoleAdmin {
			writeError(w, http.StatusForbidden, "admin required")
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

// triggerAeroApply re-applies the tenant's per-tenant OpenAIP refresh after a
// change to its key or its view AOI (ONB-6). No-op when not wired (single-tenant
// mode / tests). Idempotent on unchanged inputs (the registry compares key+AOI),
// so calling it after every view edit is cheap.
func (h *Handler) triggerAeroApply(ctx context.Context, tenantID int64) {
	if h.aeroLife != nil {
		h.aeroLife.Apply(ctx, tenantID)
	}
}

// triggerAeroStop tears down a deleted tenant's per-tenant OpenAIP refresh (ONB-6).
func (h *Handler) triggerAeroStop(tenantID int64) {
	if h.aeroLife != nil {
		h.aeroLife.Stop(tenantID)
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

// tenantOverviewDTO is one row of the AP3 admin dashboard: a tenant plus the
// aggregated configuration an admin wants at a glance — its enabled feature keys,
// the feeds it is subscribed to, and how many access accounts it has.
type tenantOverviewDTO struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Features  []string  `json:"features"`
	Feeds     []feedDTO `json:"feeds"`
	UserCount int       `json:"user_count"`
}

// getOverview aggregates every tenant's status, enabled features, feeds and
// account count into a single response (AP3). It fans out per tenant; the tenant
// count is small (operator-scale), so a sequential gather is fine and keeps the
// failure mode simple — any backend error fails the whole call rather than
// returning a partially-populated dashboard.
func (h *Handler) getOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ts, err := h.tenants.List(ctx)
	if err != nil {
		h.internalError(w, "overview: list tenants", err)
		return
	}
	out := make([]tenantOverviewDTO, 0, len(ts))
	for _, t := range ts {
		feeds, err := h.subs.ListFeedsByTenant(ctx, t.ID)
		if err != nil {
			h.internalError(w, "overview: list feeds", err)
			return
		}
		us, err := h.users.ListByTenant(ctx, t.ID)
		if err != nil {
			h.internalError(w, "overview: list users", err)
			return
		}
		out = append(out, tenantOverviewDTO{
			ID:        t.ID,
			Slug:      t.Slug,
			Name:      t.Name,
			Status:    string(t.Status),
			Features:  h.enabledFeatures(ctx, t.ID),
			Feeds:     toFeedDTOs(feeds),
			UserCount: len(us),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// enabledFeatures returns the tenant's enabled feature keys in stable catalogue
// order. Fail-closed: on a backend error the service yields an all-deny map
// (already logged), so the dashboard shows no features rather than wrong ones.
func (h *Handler) enabledFeatures(ctx context.Context, tenantID int64) []string {
	out := []string{}
	if h.feats == nil {
		return out
	}
	eff, _ := h.feats.Effective(ctx, tenantID)
	for _, k := range feature.All() {
		if eff[k] {
			out = append(out, string(k))
		}
	}
	return out
}

// getTenantView reads any tenant's default view (AP3 cross-tenant editor). Unlike
// getView (caller's own tenant, honouring a user override), this always reads the
// tenant default — there is no acting user whose override should apply.
func (h *Handler) getTenantView(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	vc, err := h.views.GetTenantDefault(r.Context(), tid)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no view configured")
		return
	}
	if err != nil {
		h.internalError(w, "get tenant view", err)
		return
	}
	writeJSON(w, http.StatusOK, toViewDTO(vc))
}

// putTenantView writes any tenant's default view (AP3). Same server-side
// validation as putView; the target tenant comes from the path. A successful
// write re-scopes that tenant's live streams (WF2-33).
func (h *Handler) putTenantView(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
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
	if !h.tenantExists(w, r, tid) {
		return
	}
	out, err := h.views.UpsertTenantDefault(r.Context(), tid, toViewConfig(d))
	if err != nil {
		h.internalError(w, "upsert tenant view", err)
		return
	}
	h.triggerRescope(r.Context(), tid)   // live-apply the new view (WF2-33)
	h.triggerAeroApply(r.Context(), tid) // re-fetch OpenAIP for the new AOI (ONB-6)
	writeJSON(w, http.StatusOK, toViewDTO(out))
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

func toFeedDTO(f store.Feed) feedDTO {
	return feedDTO{
		ID:             f.ID,
		Name:           f.Name,
		MulticastGroup: f.MulticastGroup,
		Port:           f.Port,
		Region:         f.Region,
		SensorMix:      f.SensorMix,
	}
}

func toFeedDTOs(feeds []store.Feed) []feedDTO {
	out := make([]feedDTO, len(feeds))
	for i, f := range feeds {
		out[i] = toFeedDTO(f)
	}
	return out
}

// tenantDTO is the admin-facing shape of a tenant (cross-tenant provisioning).
type tenantDTO struct {
	ID     int64  `json:"id"`
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func toTenantDTOs(tenants []store.Tenant) []tenantDTO {
	out := make([]tenantDTO, len(tenants))
	for i, t := range tenants {
		out[i] = tenantDTO{ID: t.ID, Slug: t.Slug, Name: t.Name, Status: string(t.Status)}
	}
	return out
}

// feedHealthDTO is the admin-visible health state for one feed (AP4).
type feedHealthDTO struct {
	FeedID            int64   `json:"feed_id"`
	Color             string  `json:"color"` // "green" | "yellow" | "red"
	Stale             bool    `json:"stale"`
	EverSeen          bool    `json:"ever_seen"`
	LastHeartbeatAgoS float64 `json:"last_heartbeat_ago_s"` // negative if never seen
	TrackCountRecent  int64   `json:"track_count_recent"`
	// SensorsActive and SensorsTotal are 0 until CAT063 sensor-status messages
	// arrive (Firefly issue #32). A non-zero SensorsTotal with SensorsActive <
	// SensorsTotal drives Color "yellow" (degraded fusion).
	SensorsActive int `json:"sensors_active"`
	SensorsTotal  int `json:"sensors_total"`
}

// getFeedsHealth returns the current health state for every known feed.
// It calls the FeedHealthSource for each feed in the global catalogue. If the
// health source is nil (e.g. in tests that do not wire up the registry), it
// returns an empty list.
func (h *Handler) getFeedsHealth(w http.ResponseWriter, r *http.Request) {
	if h.feedHealth == nil {
		writeJSON(w, http.StatusOK, []feedHealthDTO{})
		return
	}
	feedList, err := h.feeds.List(r.Context())
	if err != nil {
		h.internalError(w, "getFeedsHealth list", err)
		return
	}
	now := time.Now()
	out := make([]feedHealthDTO, len(feedList))
	for i, f := range feedList {
		s := h.feedHealth.Snapshot(f.ID, now)
		out[i] = feedHealthDTO{
			FeedID:            f.ID,
			Color:             s.Color(),
			Stale:             s.Stale,
			EverSeen:          s.EverSeen,
			LastHeartbeatAgoS: s.LastHeartbeatAgoS,
			TrackCountRecent:  s.TrackCountRecent,
			SensorsActive:     s.SensorsActive,
			SensorsTotal:      s.SensorsTotal,
		}
	}
	writeJSON(w, http.StatusOK, out)
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
