// Package tenant bridges authentication (pkg/auth) and persistence (pkg/store)
// at the HTTP edge: it resolves an authenticated request to a tenant Identity
// and enforces it fail-closed. This is the cross-tenant isolation anchor — every
// scoped query and stream (WF2-21) is keyed on the Identity's TenantID
// (NFR-SEC-003/004).
package tenant

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// Identity is the authenticated principal behind a request: which user, which
// tenant, which role. MustChangePassword (ONB-1, ADR 0011) is carried here so the
// admin API can gate every route except the password-change endpoint fail-closed
// without a second database lookup — the middleware already resolved the user.
type Identity struct {
	TenantID           int64
	UserID             int64
	Subject            string
	Role               store.Role
	MustChangePassword bool
}

type ctxKey struct{}

// WithIdentity returns a copy of ctx carrying id.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the Identity stored in ctx by the middleware, if any.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// UserLookup resolves an authenticated subject to its stored user.
// *store.UserRepo satisfies it; tests use a fake.
type UserLookup interface {
	GetBySubject(ctx context.Context, subject string) (store.User, error)
}

// Middleware authenticates each request and resolves it to a tenant Identity,
// which it places in the request context for downstream handlers. It is
// fail-closed: a request with no valid identity, or whose subject maps to no
// user, is rejected with 401 and never reaches next. The detailed reason is not
// leaked to the client; unexpected lookup errors are logged.
func Middleware(authenticator auth.Authenticator, users UserLookup, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, err := authenticator.Authenticate(r)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			u, err := users.GetBySubject(r.Context(), subject)
			if err != nil {
				// Unknown subject => no tenant => deny. A non-ErrNotFound error
				// (e.g. the database is down) is an operational problem worth a log.
				if logger != nil && !errors.Is(err, store.ErrNotFound) {
					logger.Warn("tenant resolution failed", slog.String("error", err.Error()))
				}
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			id := Identity{TenantID: u.TenantID, UserID: u.ID, Subject: u.Subject, Role: u.Role, MustChangePassword: u.MustChangePassword}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}
