package tenant

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// CredentialLookup retrieves a user's stored password hash.
// *store.CredentialRepo satisfies it; tests use a fake.
type CredentialLookup interface {
	GetHash(ctx context.Context, userID int64) (string, error)
}

// TenantLookup retrieves a tenant by id so login can enforce a tenant-level
// pause (AP6): a paused tenant blocks login for all of its accounts.
// *store.TenantRepo satisfies it; tests use a fake.
type TenantLookup interface {
	GetByID(ctx context.Context, id int64) (store.Tenant, error)
}

// LoginConfig configures the builtin login/logout handlers.
type LoginConfig struct {
	SessionKey []byte        // HMAC key for signing the session cookie (required)
	CookieName string        // session cookie name (default "wf_session")
	TTL        time.Duration // session lifetime (default 12h)
	Secure     bool          // set the cookie Secure flag (TLS deployments)
}

func (c LoginConfig) cookieName() string {
	if c.CookieName == "" {
		return "wf_session"
	}
	return c.CookieName
}

func (c LoginConfig) ttl() time.Duration {
	if c.TTL <= 0 {
		return 12 * time.Hour
	}
	return c.TTL
}

// dummyHash is verified against when the user or credential is not found, so a
// failed login takes roughly the same time whether or not the account exists
// (mitigates user-enumeration via timing). Computed once at startup.
var dummyHash, _ = auth.HashPassword("wayfinder-dummy-password")

type loginRequest struct {
	Subject  string `json:"subject"`
	Password string `json:"password"`
}

// LoginHandler verifies a builtin-mode subject/password and, on success, sets a
// signed session cookie (later consumed by auth.BuiltinAuthenticator). Every
// failure returns the same 401 without revealing whether the subject exists, is
// paused, or simply gave the wrong password. tenants (may be nil) enables the
// tenant-pause cascade (AP6); when nil only the per-account status is enforced.
func LoginHandler(users UserLookup, creds CredentialLookup, tenants TenantLookup, cfg LoginConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req loginRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.Subject == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Always run a password verification (against a dummy hash if the user or
		// credential is missing) to keep timing uniform.
		hash := dummyHash
		suspended := false
		u, lookupErr := users.GetBySubject(r.Context(), req.Subject)
		if lookupErr == nil {
			if h, herr := creds.GetHash(r.Context(), u.ID); herr == nil {
				hash = h
			}
			// AP6: a paused account — or an account under a paused tenant — may not
			// log in even with correct credentials. Both checks are fail-closed: a
			// tenant lookup error is treated as suspended. The generic 401 below does
			// not reveal that this was the reason (no paused/active enumeration).
			// A platform admin has no tenant (TenantID 0, ONB-3) — there is no tenant
			// to cascade from, so the tenant-pause check is skipped for admins (only
			// their own account status gates them; otherwise GetByID(0) would fail and
			// fail-closed lock every admin out).
			suspended = u.Status == store.StatusPaused
			if !suspended && tenants != nil && u.TenantID != 0 {
				if t, terr := tenants.GetByID(r.Context(), u.TenantID); terr != nil || t.Status == store.StatusPaused {
					suspended = true
				}
			}
		}

		ok, verr := auth.VerifyPassword(hash, req.Password)
		if lookupErr != nil || verr != nil || !ok || suspended {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     cfg.cookieName(),
			Value:    auth.MintSession(u.Subject, cfg.ttl(), cfg.SessionKey),
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(cfg.ttl().Seconds()),
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// RenewHandler re-issues the session cookie with a fresh TTL for the
// already-authenticated principal — the sliding-session refresh (WF2-12.5). It
// sits BEHIND the tenant middleware (which sets the Identity from the current
// cookie); without a valid Identity it returns 401. The ASD calls it periodically
// while the live picture is open (and on WebSocket reconnect / tab focus), so an
// actively-used console is never logged out, while an abandoned session still
// lapses after the (then-unrenewed) TTL. builtin mode only — a proxy session
// lives in the upstream OIDC proxy, not in this cookie.
func RenewHandler(cfg LoginConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     cfg.cookieName(),
			Value:    auth.MintSession(id.Subject, cfg.ttl(), cfg.SessionKey),
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(cfg.ttl().Seconds()),
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// LogoutHandler clears the session cookie.
func LogoutHandler(cfg LoginConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     cfg.cookieName(),
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
