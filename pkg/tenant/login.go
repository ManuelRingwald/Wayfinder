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
	TTL        time.Duration // sliding idle window: session lifetime per (re)issue (default 12h)
	// MaxLifetime is an absolute cap on a session's total lifetime measured from
	// first login, enforced independently of activity: no matter how active a
	// console is, once MaxLifetime has elapsed the sliding renew stops and the
	// operator must log in again. 0 (the default) disables the cap — pure sliding.
	MaxLifetime time.Duration
	Secure      bool // set the cookie Secure flag (TLS deployments)
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

// expiry computes a session's expiry for a login/renew happening at `now`, given
// the session's first-login time `issuedAt`. It is `now + TTL` (the sliding idle
// window), clamped so the session never outlives `issuedAt + MaxLifetime` when an
// absolute maximum is configured. With MaxLifetime <= 0 the cap is off and this is
// just the sliding expiry. Because the cap lowers the expiry itself, the absolute
// maximum is enforced by the ordinary expiry check too — even a client that never
// calls renew self-expires at the cap; it does not rely on the renew tor alone.
func (c LoginConfig) expiry(now, issuedAt time.Time) time.Time {
	exp := now.Add(c.ttl())
	if c.MaxLifetime > 0 {
		if limit := issuedAt.Add(c.MaxLifetime); limit.Before(exp) {
			exp = limit
		}
	}
	return exp
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

		// First login: stamp the issued-at at now and cap the expiry at the absolute
		// maximum (if any). With a max shorter than the TTL the cookie already carries
		// the shortened lifetime, so an unrenewed session still self-expires at the cap.
		now := time.Now()
		exp := cfg.expiry(now, now)
		http.SetCookie(w, &http.Cookie{
			Name:     cfg.cookieName(),
			Value:    auth.MintSessionAt(u.Subject, now, exp, cfg.SessionKey),
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(time.Until(exp).Seconds()),
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

		// Recover the original first-login time so the sliding renew can honour an
		// absolute maximum lifetime. The middleware already validated the cookie; we
		// re-read it only for the issued-at claim (not carried on Identity). A legacy
		// cookie without issued-at is treated softly: anchor the cap at now (this
		// first renew) instead of bouncing everyone on the upgrade.
		now := time.Now()
		issuedAt := now
		if c, err := r.Cookie(cfg.cookieName()); err == nil {
			if claims, perr := auth.ParseSessionClaims(c.Value, cfg.SessionKey); perr == nil && claims.IssuedAt > 0 {
				issuedAt = time.Unix(claims.IssuedAt, 0)
			}
		}

		// Absolute maximum reached → stop sliding and force a fresh login. The capped
		// expiry is already at/before now here, so re-minting would hand out a dead
		// cookie; return the generic 401 (no cookie) instead.
		exp := cfg.expiry(now, issuedAt)
		if !exp.After(now) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     cfg.cookieName(),
			Value:    auth.MintSessionAt(id.Subject, issuedAt, exp, cfg.SessionKey),
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(time.Until(exp).Seconds()),
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
