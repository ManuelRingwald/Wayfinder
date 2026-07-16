package correlationapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/fireflycmd"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

type fakeCommander struct {
	err          error
	correlated   bool
	uncorrelated bool
	cleared      bool
	lastFeed     int64
	lastTrack    uint16
	lastCallsign string
}

func (f *fakeCommander) Correlate(_ context.Context, feedID int64, trackNum uint16, callsign string) error {
	f.correlated, f.lastFeed, f.lastTrack, f.lastCallsign = true, feedID, trackNum, callsign
	return f.err
}

func (f *fakeCommander) SetUncorrelated(_ context.Context, feedID int64, trackNum uint16) error {
	f.uncorrelated, f.lastFeed, f.lastTrack = true, feedID, trackNum
	return f.err
}

func (f *fakeCommander) ClearOverride(_ context.Context, feedID int64, trackNum uint16) error {
	f.cleared, f.lastFeed, f.lastTrack = true, feedID, trackNum
	return f.err
}

func (f *fakeCommander) called() bool { return f.correlated || f.uncorrelated || f.cleared }

type fakeSubs struct {
	subscribed bool
	err        error
}

func (f fakeSubs) IsSubscribed(context.Context, int64, int64) (bool, error) {
	return f.subscribed, f.err
}

// tenantUser is an ordinary authenticated tenant user (the controller).
func tenantUser() tenant.Identity {
	return tenant.Identity{TenantID: 5, UserID: 9, Subject: "ctrl", Role: store.RoleUser}
}

// req builds a POST/DELETE request carrying an identity (and optionally an active
// read-only impersonation grant), as tenantMW would in production.
func req(method, target, body string, id *tenant.Identity, impersonating bool) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := r.Context()
	if id != nil {
		ctx = tenant.WithIdentity(ctx, *id)
	}
	if impersonating {
		ctx = tenant.WithReadTenant(ctx, 999) // admin viewing as tenant 999
	}
	return r.WithContext(ctx)
}

func svc(cmd Commander, subs SubscriptionChecker, enabled bool) *Service {
	return New(cmd, subs, enabled, nil)
}

// --- Happy path -----------------------------------------------------------

func TestSetCorrelatePinsPlan(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	id := tenantUser()
	svc(cmd, fakeSubs{subscribed: true}, true).SetHandler()(
		rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42,"callsign":"DLH123"}`, &id, false))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", rec.Code, rec.Body)
	}
	if !cmd.correlated || cmd.lastFeed != 7 || cmd.lastTrack != 42 || cmd.lastCallsign != "DLH123" {
		t.Errorf("commander = %+v, want Correlate(feed 7, track 42, DLH123)", cmd)
	}
}

func TestSetUncorrelatedWhenCallsignOmitted(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	id := tenantUser()
	svc(cmd, fakeSubs{subscribed: true}, true).SetHandler()(
		rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42}`, &id, false))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if !cmd.uncorrelated || cmd.correlated {
		t.Errorf("commander = %+v, want SetUncorrelated only", cmd)
	}
}

func TestClearOverrideDeletes(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	id := tenantUser()
	r := req(http.MethodDelete, "/api/correlation/7/42", "", &id, false)
	r.SetPathValue("feedID", "7")
	r.SetPathValue("trackNumber", "42")
	svc(cmd, fakeSubs{subscribed: true}, true).ClearHandler()(rec, r)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if !cmd.cleared || cmd.lastFeed != 7 || cmd.lastTrack != 42 {
		t.Errorf("commander = %+v, want ClearOverride(7, 42)", cmd)
	}
}

// --- Authorization gates (the heart of Häppchen 2) ------------------------

func TestUnauthenticatedIs401(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	svc(cmd, fakeSubs{subscribed: true}, true).SetHandler()(
		rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42,"callsign":"X"}`, nil, false))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if cmd.called() {
		t.Errorf("commander was called for an unauthenticated request")
	}
}

func TestNotSubscribedIs403(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	id := tenantUser()
	svc(cmd, fakeSubs{subscribed: false}, true).SetHandler()(
		rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42,"callsign":"X"}`, &id, false))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if cmd.called() {
		t.Errorf("commander was called for a non-subscribed tenant")
	}
}

func TestImpersonationIs403EvenWhenSubscribed(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	id := tenantUser()
	// subscribed=true would pass the feed gate — impersonation must still block the
	// write (ADR 0008: read-only). The check precedes the subscription check.
	svc(cmd, fakeSubs{subscribed: true}, true).SetHandler()(
		rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42,"callsign":"X"}`, &id, true))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 under impersonation", rec.Code)
	}
	if cmd.called() {
		t.Errorf("commander was called under read-only impersonation")
	}
}

func TestScopelessAdminIs403(t *testing.T) {
	// An admin without an impersonation grant has their own tenant, which holds no
	// subscriptions (ADR 0022) — the IsSubscribed gate refuses them. No special case.
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	admin := tenant.Identity{TenantID: 1, UserID: 1, Role: store.RoleAdmin}
	svc(cmd, fakeSubs{subscribed: false}, true).SetHandler()(
		rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42,"callsign":"X"}`, &admin, false))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a scope-less admin", rec.Code)
	}
	if cmd.called() {
		t.Errorf("commander was called for a scope-less admin")
	}
}

func TestSubscriptionCheckErrorIs500(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	id := tenantUser()
	svc(cmd, fakeSubs{err: errors.New("db down")}, true).SetHandler()(
		rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42,"callsign":"X"}`, &id, false))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if cmd.called() {
		t.Errorf("commander was called after an authz check failure")
	}
}

// --- Feature gate + input validation --------------------------------------

func TestFeatureDisabledIs503(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	id := tenantUser()
	svc(cmd, fakeSubs{subscribed: true}, false).SetHandler()( // enabled=false
		rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42,"callsign":"X"}`, &id, false))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when disabled", rec.Code)
	}
	if cmd.called() {
		t.Errorf("commander was called while the feature is disabled")
	}
}

func TestInputValidation(t *testing.T) {
	id := tenantUser()
	cases := []struct {
		name string
		body string
	}{
		{"malformed json", `{not json`},
		{"missing feed_id", `{"track_number":42,"callsign":"X"}`},
		{"empty callsign", `{"feed_id":7,"track_number":42,"callsign":"  "}`},
		{"unknown field", `{"feed_id":7,"track_number":42,"bogus":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &fakeCommander{}
			rec := httptest.NewRecorder()
			svc(cmd, fakeSubs{subscribed: true}, true).SetHandler()(
				rec, req(http.MethodPost, "/api/correlation", tc.body, &id, false))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if cmd.called() {
				t.Errorf("commander was called for an invalid body")
			}
		})
	}
}

// --- Firefly error mapping ------------------------------------------------

func TestCommandErrorMapping(t *testing.T) {
	id := tenantUser()
	cases := []struct {
		err  error
		want int
	}{
		{fireflycmd.ErrUnknownCallsign, http.StatusUnprocessableEntity}, // 422
		{fireflycmd.ErrNoFlightPlans, http.StatusConflict},              // 409
		{fireflycmd.ErrUnreachable, http.StatusBadGateway},              // 502
		{fireflycmd.ErrUnauthorized, http.StatusBadGateway},             // 502 (server misconfig)
		{errors.New("something else"), http.StatusBadGateway},           // 502 generic
	}
	for _, tc := range cases {
		cmd := &fakeCommander{err: tc.err}
		rec := httptest.NewRecorder()
		svc(cmd, fakeSubs{subscribed: true}, true).SetHandler()(
			rec, req(http.MethodPost, "/api/correlation", `{"feed_id":7,"track_number":42,"callsign":"X"}`, &id, false))
		if rec.Code != tc.want {
			t.Errorf("error %v → status %d, want %d", tc.err, rec.Code, tc.want)
		}
	}
}

func TestBadPathParamsIs400(t *testing.T) {
	cmd := &fakeCommander{}
	rec := httptest.NewRecorder()
	id := tenantUser()
	r := req(http.MethodDelete, "/api/correlation/x/y", "", &id, false)
	r.SetPathValue("feedID", "notanumber")
	r.SetPathValue("trackNumber", "42")
	svc(cmd, fakeSubs{subscribed: true}, true).ClearHandler()(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a bad feed id", rec.Code)
	}
	if cmd.called() {
		t.Errorf("commander was called with a bad path parameter")
	}
}

// *fireflycmd.Client must satisfy Commander (compile-time check).
var _ Commander = (*fireflycmd.Client)(nil)
