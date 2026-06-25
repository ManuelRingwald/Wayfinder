package tenant

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

func TestRequireRole(t *testing.T) {
	gate := RequireRole(store.RoleAdmin)

	cases := map[string]struct {
		identity *Identity // nil = no identity in context (gate used without Middleware)
		want     int
	}{
		"admin allowed":         {&Identity{Role: store.RoleAdmin}, http.StatusOK},
		"user forbidden":        {&Identity{Role: store.RoleUser}, http.StatusForbidden},
		"empty role forbidden":  {&Identity{}, http.StatusForbidden},
		"no identity forbidden": {nil, http.StatusForbidden},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			reached := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			if tc.identity != nil {
				req = req.WithContext(WithIdentity(req.Context(), *tc.identity))
			}
			rec := httptest.NewRecorder()
			gate(next).ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if reached != (tc.want == http.StatusOK) {
				t.Fatalf("next reached = %v, want %v", reached, tc.want == http.StatusOK)
			}
		})
	}
}
