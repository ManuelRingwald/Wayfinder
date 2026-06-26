package main

import (
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

func TestBootstrapParamsValidate(t *testing.T) {
	cases := map[string]struct {
		p       bootstrapParams
		wantErr bool
	}{
		// A user is homed under a tenant — so -tenant is required for a user.
		"ok user":             {bootstrapParams{TenantSlug: "demo", Subject: "pilot", Role: store.RoleUser}, false},
		"missing tenant user": {bootstrapParams{Subject: "pilot", Role: store.RoleUser}, true},
		// A platform admin has no tenant (ONB-3) — -tenant is not required (ignored).
		"ok admin no tenant":   {bootstrapParams{Subject: "admin", Role: store.RoleAdmin}, false},
		"admin tenant ignored": {bootstrapParams{TenantSlug: "demo", Subject: "admin", Role: store.RoleAdmin}, false},
		"missing subject":      {bootstrapParams{TenantSlug: "demo", Role: store.RoleAdmin}, true},
		"invalid role":         {bootstrapParams{TenantSlug: "demo", Subject: "admin", Role: store.Role("root")}, true},
		"empty role":           {bootstrapParams{TenantSlug: "demo", Subject: "admin"}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := tc.p.validate(); (err != nil) != tc.wantErr {
				t.Fatalf("validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
