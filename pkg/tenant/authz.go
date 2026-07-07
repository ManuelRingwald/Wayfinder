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

// RequirePasswordChanged refuses any request whose Identity still carries the
// must_change_password flag (ONB-1, ADR 0011; hardened by #208/ADR 0022). The
// admin API already enforces this via its own allowlist (pkg/adminapi); this
// gate extends the rule to the operational data paths (/ws, overlays, weather),
// so a principal on the well-known seed credential can reach NOTHING but the
// password-change flow — regardless of which URL it logs in through. The 403
// body carries the same stable marker the SPA keys on. Fail-closed: a request
// with no Identity in context is denied, like RequireRole.
func RequirePasswordChanged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := FromContext(r.Context())
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if id.MustChangePassword {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"password_change_required"}` + "\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
