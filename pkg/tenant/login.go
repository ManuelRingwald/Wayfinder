package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"net"
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

// SessionStore is the server-side session registry the login/renew/logout
// handlers use (AP7, ADR 0009 §5). *store.SessionRepo satisfies it; tests use a
// fake. When LoginConfig.Sessions is nil the handlers fall back to the pre-AP7
// stateless cookie (used e.g. by tests that do not exercise the registry).
type SessionStore interface {
	CreateSession(ctx context.Context, userID int64, createdAt, expiresAt time.Time, limit int, policy store.SessionLimitPolicy, meta store.SessionMeta) (token string, err error)
	ExtendSession(ctx context.Context, token string, ttl, maxLife time.Duration) (time.Time, error)
	DeleteSession(ctx context.Context, token string) error
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

	// Sessions is the server-side session registry (AP7). When set, login opens a
	// registry session (enforcing the concurrent-session limit) and hands out a
	// session-id cookie; renew extends the registry row; logout deletes it. Nil
	// keeps the pre-AP7 stateless cookie behaviour.
	Sessions SessionStore
	// SessionLimitDefault is the concurrent-session limit applied to an access that
	// has no per-access override (WAYFINDER_SESSION_LIMIT_DEFAULT). 0 == unlimited.
	SessionLimitDefault int
	// SessionLimitPolicy decides what happens at the limit (reject | evict_oldest).
	// An unset/invalid value is treated as reject (the ADR default).
	SessionLimitPolicy store.SessionLimitPolicy
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

// policy returns the configured limit-overflow policy, defaulting to reject.
func (c LoginConfig) policy() store.SessionLimitPolicy {
	if c.SessionLimitPolicy.Valid() {
		return c.SessionLimitPolicy
	}
	return store.SessionLimitReject
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

// effectiveLimit resolves the concurrent-session limit for a user: a per-access
// override (users.session_limit) wins over the deployment default.
func (c LoginConfig) effectiveLimit(u store.User) int {
	if u.SessionLimit != nil {
		return *u.SessionLimit
	}
	return c.SessionLimitDefault
}

// setSessionCookie writes the session cookie with the shared attributes
// (HttpOnly, Secure per deployment, Lax) and a MaxAge tracking the expiry.
func setSessionCookie(w http.ResponseWriter, cfg LoginConfig, value string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.cookieName(),
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(time.Until(exp).Seconds()),
	})
}

// clientMeta records best-effort, advisory-only client context for a session
// (never trusted for authorisation). The user agent is bounded so a hostile
// header cannot bloat the row.
func clientMeta(r *http.Request) store.SessionMeta {
	ua := r.UserAgent()
	if len(ua) > 256 {
		ua = ua[:256]
	}
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return store.SessionMeta{UserAgent: ua, IP: ip}
}

// dummyHash is verified against when the user or credential is not found, so a
// failed login takes roughly the same time whether or not the account exists
// (mitigates user-enumeration via timing). Computed once at startup.
var dummyHash, _ = auth.HashPassword("wayfinder-dummy-password")

type loginRequest struct {
	Subject  string `json:"subject"`
	Password string `json:"password"`
}

// LoginHandler verifies a builtin-mode subject/password and, on success, opens a
// session and sets a signed session cookie (later consumed by
// auth.BuiltinAuthenticator). Every credential failure returns the same 401
// without revealing whether the subject exists, is paused, or simply gave the
// wrong password. tenants (may be nil) enables the tenant-pause cascade (AP6);
// when nil only the per-account status is enforced.
//
// When cfg.Sessions is set (AP7) the session is recorded in the registry with the
// concurrent-session limit enforced: a login that exceeds the limit under the
// reject policy is refused with 429 — distinct from a 401 because the credentials
// were already proven correct, so it leaks nothing about other accounts.
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

		if cfg.Sessions == nil {
			// Pre-AP7 stateless cookie: no registry, no limit.
			setSessionCookie(w, cfg, auth.MintSessionAt(u.Subject, now, exp, cfg.SessionKey), exp)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// AP7: open a registry session, enforcing the per-access concurrent-session
		// limit. A limit breach under the reject policy is a 429 (the credentials were
		// valid — the operator must free a session), not the credential 401.
		token, cerr := cfg.Sessions.CreateSession(r.Context(), u.ID, now, exp, cfg.effectiveLimit(u), cfg.policy(), clientMeta(r))
		if cerr != nil {
			if errors.Is(cerr, store.ErrSessionLimit) {
				http.Error(w, "session limit reached", http.StatusTooManyRequests)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		setSessionCookie(w, cfg, auth.MintSessionID(token, cfg.SessionKey), exp)
		w.WriteHeader(http.StatusNoContent)
	}
}

// RenewHandler re-issues the session for the already-authenticated principal —
// the sliding-session refresh (WF2-12.5). It sits BEHIND the tenant middleware
// (which sets the Identity from the current cookie); without a valid Identity it
// returns 401. The ASD calls it periodically while the live picture is open (and
// on WebSocket reconnect / tab focus), so an actively-used console is never logged
// out, while an abandoned session still lapses after the (then-unrenewed) TTL.
// builtin mode only — a proxy session lives in the upstream OIDC proxy.
//
// With cfg.Sessions set (AP7) it extends the registry row (revoked/expired →
// 401); a legacy stateless cookie encountered here is converted into a registry
// session (sanfte Übernahme), anchoring the absolute-max clock at the original
// first-login time. Without Sessions it re-mints the stateless cookie (pre-AP7).
func RenewHandler(cfg LoginConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		now := time.Now()

		if cfg.Sessions == nil {
			renewStateless(w, r, cfg, id, now)
			return
		}

		// Registry session → slide its expiry in place (same token/cookie).
		if c, cerr := r.Cookie(cfg.cookieName()); cerr == nil {
			if token, perr := auth.ParseSessionID(c.Value, cfg.SessionKey); perr == nil {
				exp, eerr := cfg.Sessions.ExtendSession(r.Context(), token, cfg.ttl(), cfg.MaxLifetime)
				if eerr != nil {
					// Revoked/expired/unknown → force a fresh login (no cookie).
					if errors.Is(eerr, store.ErrNotFound) {
						http.Error(w, "unauthorized", http.StatusUnauthorized)
						return
					}
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				// Absolute maximum reached → the capped expiry is not after now; stop
				// sliding and force a fresh login rather than hand out a dead cookie.
				if !exp.After(now) {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				setSessionCookie(w, cfg, c.Value, exp)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		// No registry cookie (a legacy stateless cookie, or none) → adopt into a new
		// registry session, anchoring the absolute-max clock at the original
		// first-login time when the legacy cookie still carries one. The concurrent-
		// session limit is NOT enforced on conversion: this is an already-active
		// console, not a new login, and bouncing it mid-rollout would be user-hostile.
		issuedAt := now
		if c, cerr := r.Cookie(cfg.cookieName()); cerr == nil {
			if claims, perr := auth.ParseSessionClaims(c.Value, cfg.SessionKey); perr == nil && claims.IssuedAt > 0 {
				issuedAt = time.Unix(claims.IssuedAt, 0)
			}
		}
		exp := cfg.expiry(now, issuedAt)
		if !exp.After(now) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		token, cerr := cfg.Sessions.CreateSession(r.Context(), id.UserID, issuedAt, exp, 0, store.SessionLimitReject, clientMeta(r))
		if cerr != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		setSessionCookie(w, cfg, auth.MintSessionID(token, cfg.SessionKey), exp)
		w.WriteHeader(http.StatusNoContent)
	}
}

// renewStateless is the pre-AP7 sliding renew: re-mint the signed stateless
// cookie, preserving the original issued-at so the absolute maximum keeps
// counting. Kept for the Sessions-nil path (tests, and any non-registry mode).
func renewStateless(w http.ResponseWriter, r *http.Request, cfg LoginConfig, id Identity, now time.Time) {
	issuedAt := now
	if c, err := r.Cookie(cfg.cookieName()); err == nil {
		if claims, perr := auth.ParseSessionClaims(c.Value, cfg.SessionKey); perr == nil && claims.IssuedAt > 0 {
			issuedAt = time.Unix(claims.IssuedAt, 0)
		}
	}
	exp := cfg.expiry(now, issuedAt)
	if !exp.After(now) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	setSessionCookie(w, cfg, auth.MintSessionAt(id.Subject, issuedAt, exp, cfg.SessionKey), exp)
	w.WriteHeader(http.StatusNoContent)
}

// LogoutHandler clears the session cookie and, when the cookie names a registry
// session (AP7), deletes that session server-side — a real logout, not just a
// browser-side cookie clear. It is unauthenticated (like login): it acts only on
// the presented cookie and is idempotent.
func LogoutHandler(cfg LoginConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Sessions != nil {
			if c, err := r.Cookie(cfg.cookieName()); err == nil {
				if token, perr := auth.ParseSessionID(c.Value, cfg.SessionKey); perr == nil {
					// Best-effort: a failed delete still clears the cookie below.
					_ = cfg.Sessions.DeleteSession(r.Context(), token)
				}
			}
		}
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
