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
		"ok":              {bootstrapParams{TenantSlug: "demo", Subject: "admin", Role: store.RoleAdmin}, false},
		"missing tenant":  {bootstrapParams{Subject: "admin", Role: store.RoleAdmin}, true},
		"missing subject": {bootstrapParams{TenantSlug: "demo", Role: store.RoleAdmin}, true},
		"invalid role":    {bootstrapParams{TenantSlug: "demo", Subject: "admin", Role: store.Role("root")}, true},
		"empty role":      {bootstrapParams{TenantSlug: "demo", Subject: "admin"}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := tc.p.validate(); (err != nil) != tc.wantErr {
				t.Fatalf("validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
