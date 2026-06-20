package tenant

import (
	"net/http"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// RequireRole returns middleware that lets a request through only if its
// Identity (established upstream by Middleware) carries one of the allowed
// roles; otherwise it responds 403 and never calls next. It is fail-closed: a
// request with no Identity in context — i.e. RequireRole used without Middleware
// in front — is also denied. This is the authorisation gate behind /admin
// (WF2-13); the admin API/UI itself follows in WF2-31/32.
func RequireRole(allowed ...store.Role) func(http.Handler) http.Handler {
	set := make(map[store.Role]bool, len(allowed))
	for _, r := range allowed {
		set[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := FromContext(r.Context())
			if !ok || !set[id.Role] {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
