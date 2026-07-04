package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/impersonation"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// Cross-tenant read-only impersonation wiring (ADR 0008, WF2-34). An admin
// "View as Tenant X" mints a signed, time-boxed grant cookie; the /ws read path
// (newScopeResolver) honours it, resolving feed scope AND view against the target
// tenant. The authenticated Identity is never touched and no write path uses the
// impersonated tenant — impersonation is structurally read-only.

// impersonationSessions counts read-only impersonation /ws sessions that were
// started (exposed as wayfinder_impersonation_sessions_total). It is deliberately
// separate from the per-tenant billing/SLA series, which exclude impersonation
// entirely (the session's scope.TenantID is zeroed — ADR 0008 §6).
var impersonationSessions atomic.Int64

// impersonationCookieConfig carries what the mint/clear handlers need: the HMAC
// signing key (reused from the session key), the grant TTL, and whether the
// cookie must be Secure (TLS terminated here).
type impersonationCookieConfig struct {
	key    []byte
	ttl    time.Duration
	secure bool
}

// tenantExistsChecker adapts *store.TenantRepo to impersonation.TenantChecker:
// a target tenant must exist before it can be impersonated. ErrNotFound maps to
// (false, nil); any other error fails closed (propagated to the caller).
type tenantExistsChecker struct{ repo *store.TenantRepo }

func (a tenantExistsChecker) Exists(ctx context.Context, tenantID int64) (bool, error) {
	if _, err := a.repo.GetByID(ctx, tenantID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// impersonationGrantCookie returns the raw grant from the request's
// wf_impersonation cookie, or "" when absent (the common, non-impersonated case).
func impersonationGrantCookie(r *http.Request) string {
	c, err := r.Cookie(impersonation.CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// startImpersonationHandler mints a grant naming the target tenant and sets it as
// the HttpOnly wf_impersonation cookie. It is admin-only (enforced by the route's
// RequireRole gate); the target tenant must exist (404 otherwise).
func startImpersonationHandler(tenants impersonation.TenantChecker, cfg impersonationCookieConfig, audit *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := tenant.FromContext(r.Context())
		if !ok { // defensive — the gate already guarantees an identity
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		var body struct {
			TenantID int64 `json:"tenant_id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil || body.TenantID <= 0 {
			http.Error(w, `{"error":"tenant_id (positive integer) required"}`, http.StatusBadRequest)
			return
		}
		exists, err := tenants.Exists(r.Context(), body.TenantID)
		if err != nil {
			http.Error(w, `{"error":"tenant lookup failed"}`, http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, `{"error":"target tenant does not exist"}`, http.StatusNotFound)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     impersonation.CookieName,
			Value:    impersonation.MintGrant(body.TenantID, cfg.ttl, cfg.key),
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.secure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   int(cfg.ttl.Seconds()),
		})
		audit.Info("impersonation started",
			slog.String("event", "impersonation_start"),
			slog.Int64("actor_user_id", id.UserID),
			slog.String("actor_subject", id.Subject),
			slog.Int64("impersonated_tenant_id", body.TenantID),
			slog.String("remote", r.RemoteAddr))
		w.WriteHeader(http.StatusNoContent)
	}
}

// stopImpersonationHandler clears the grant cookie (the "Exit" action). It is safe
// for any admin to call: it only removes the caller's own cookie.
func stopImpersonationHandler(cfg impersonationCookieConfig, audit *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     impersonation.CookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.secure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
		})
		if id, ok := tenant.FromContext(r.Context()); ok {
			audit.Info("impersonation ended",
				slog.String("event", "impersonation_end"),
				slog.Int64("actor_user_id", id.UserID),
				slog.String("actor_subject", id.Subject),
				slog.String("remote", r.RemoteAddr))
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// impersonationStatusHandler reports the caller's current impersonation state so
// the SPA can restore the read-only banner after a reload (the grant cookie is
// HttpOnly and not readable by JS). It is advisory: any non-active outcome — no
// cookie, an expired grant, or one the caller may not use — is reported as
// inactive without error; the /ws path remains the enforcement point.
func impersonationStatusHandler(tenants impersonation.TenantChecker, cfg impersonationCookieConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := tenant.FromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		resp := struct {
			Active   bool  `json:"active"`
			TenantID int64 `json:"tenant_id,omitempty"`
		}{}
		if d, err := impersonation.Resolve(r.Context(), impersonationGrantCookie(r), id, cfg.key, tenants); err == nil && d.Active {
			resp.Active = true
			resp.TenantID = d.TargetTenantID
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// logImpersonationDenied records a refused impersonation attempt — a valid grant
// presented by a non-admin, or one naming a missing tenant (ADR 0008 §3,
// decision 4: spoofing/misuse attempts must be loud and auditable, never silent).
func logImpersonationDenied(audit *slog.Logger, r *http.Request, id tenant.Identity, reason error) {
	audit.Warn("impersonation denied",
		slog.String("event", "impersonation_denied"),
		slog.Int64("actor_user_id", id.UserID),
		slog.String("actor_subject", id.Subject),
		slog.String("actor_role", string(id.Role)),
		slog.String("reason", reason.Error()),
		slog.String("remote", r.RemoteAddr))
}

// impersonationReadMW extends the grant to the map's plain read-only REST
// endpoints (whoami, aeronautical overlays, QNH) with byte-identical /ws
// semantics — the same impersonation.Resolve decides. Without it the ASD showed
// a half-impersonated picture: the /ws stream switched to the target tenant but
// every REST-derived piece (features → layer toggles, overlays, weather, ICAO,
// FL band) still answered for the tenant-less admin, i.e. empty.
//
// Outcomes: absent/stale grant → the request passes through unchanged (the
// non-impersonated path stays byte-identical); a valid admin grant stamps the
// effective read tenant onto the context (tenant.WithReadTenant) for the
// wrapped handler; a valid grant from a non-admin, or naming a missing tenant,
// is rejected 403 and audited — never silently honoured or ignored. It sits
// INSIDE the tenant middleware (needs the Identity) and is applied only to
// read-only routes; write paths never see a read-tenant context.
func impersonationReadMW(tenants impersonation.TenantChecker, key []byte, audit *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := tenant.FromContext(r.Context())
			if !ok { // defensive — the tenant middleware runs outside this one
				next.ServeHTTP(w, r)
				return
			}
			d, err := impersonation.Resolve(r.Context(), impersonationGrantCookie(r), id, key, tenants)
			if err != nil {
				logImpersonationDenied(audit, r, id, err)
				status := http.StatusForbidden
				if !errors.Is(err, impersonation.ErrDenied) && !errors.Is(err, impersonation.ErrUnknownTenant) {
					status = http.StatusInternalServerError // e.g. tenant lookup failed → fail closed
				}
				http.Error(w, `{"error":"impersonation denied"}`, status)
				return
			}
			if d.Active {
				r = r.WithContext(tenant.WithReadTenant(r.Context(), d.TargetTenantID))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// readTenantOf resolves the tenant whose data a read endpoint serves: the
// impersonation target when impersonationReadMW stamped one, else the caller's
// own tenant. Used as the aeronautical Registry's TenantResolver.
func readTenantOf(r *http.Request) (int64, bool) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		return 0, false
	}
	return tenant.ReadTenant(r.Context(), id.TenantID), true
}
