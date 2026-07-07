package tenant

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

// #208 (ADR 0022): while must_change_password is set, the operational data paths
// are refused fail-closed with the stable marker the SPA keys on — the seed
// credential can reach nothing but the password-change flow.
func TestRequirePasswordChanged(t *testing.T) {
	cases := map[string]struct {
		identity *Identity // nil = no identity in context (gate used without Middleware)
		want     int
	}{
		"cleared flag allowed":    {&Identity{Role: store.RoleUser}, http.StatusOK},
		"flagged admin refused":   {&Identity{Role: store.RoleAdmin, MustChangePassword: true}, http.StatusForbidden},
		"flagged user refused":    {&Identity{Role: store.RoleUser, MustChangePassword: true}, http.StatusForbidden},
		"no identity fail-closed": {nil, http.StatusForbidden},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			reached := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tc.identity != nil {
				req = req.WithContext(WithIdentity(req.Context(), *tc.identity))
			}
			rec := httptest.NewRecorder()
			RequirePasswordChanged(next).ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if reached != (tc.want == http.StatusOK) {
				t.Fatalf("next reached = %v, want %v", reached, tc.want == http.StatusOK)
			}
			// The refusal for a flagged principal carries the SPA's stable marker.
			if tc.identity != nil && tc.identity.MustChangePassword &&
				!strings.Contains(rec.Body.String(), "password_change_required") {
				t.Fatalf("body = %q, want password_change_required marker", rec.Body.String())
			}
		})
	}
}
