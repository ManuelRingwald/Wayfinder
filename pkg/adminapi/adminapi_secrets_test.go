package adminapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// fakeSecretService is an in-memory SecretService. It records plaintext values
// only so the tests can assert what was set; production seals before storage.
type fakeSecretService struct {
	values map[string]string // cred_ref -> plaintext (test-only)
	setErr error
}

func newFakeSecretService() *fakeSecretService {
	return &fakeSecretService{values: map[string]string{}}
}

func (f *fakeSecretService) SetSecret(_ context.Context, _ int64, credRef, plaintext string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.values[credRef] = plaintext
	return nil
}

func (f *fakeSecretService) DeleteSecret(_ context.Context, _ int64, credRef string) error {
	if _, ok := f.values[credRef]; !ok {
		return store.ErrNotFound
	}
	delete(f.values, credRef)
	return nil
}

func (f *fakeSecretService) ListSecretRefs(_ context.Context, _ int64) ([]string, error) {
	refs := make([]string, 0, len(f.values))
	for ref := range f.values {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs, nil
}

// feedWithID returns a fakeFeeds where the given id exists (so feedExists passes).
func feedWithID(id int64) fakeFeeds {
	return fakeFeeds{byID: map[int64]store.Feed{id: {ID: id}}}
}

func handlerForSecrets(ff fakeFeeds, svc SecretService) *Handler {
	return New(&fakeVS{}, &fakeVS{}, ff, fakeTenants{}, &fakeUserStore{}, &fakeCredStore{},
		&fakeEntitlements{}, nil, nil, nil, svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func TestGetFeedSecretsReportsConfiguredRefs(t *testing.T) {
	svc := newFakeSecretService()
	svc.values["secret/opensky"] = "v1"
	svc.values["secret/flarm"] = "v2"
	rec := httptest.NewRecorder()
	handlerForSecrets(feedWithID(5), svc).ServeHTTP(rec,
		adminReq(http.MethodGet, "/api/admin/feeds/5/secrets", "", 1, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got feedSecretsDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Secrets) != 2 || got.Secrets[0].Ref != "secret/flarm" || got.Secrets[1].Ref != "secret/opensky" {
		t.Fatalf("secrets = %+v, want sorted [flarm, opensky]", got.Secrets)
	}
	if !got.Secrets[0].Configured || !got.Secrets[1].Configured {
		t.Fatalf("configured flags = %+v, want all true", got.Secrets)
	}
}

func TestGetFeedSecretsNeverLeaksValue(t *testing.T) {
	svc := newFakeSecretService()
	svc.values["secret/opensky"] = "super-secret-value"
	rec := httptest.NewRecorder()
	handlerForSecrets(feedWithID(5), svc).ServeHTTP(rec,
		adminReq(http.MethodGet, "/api/admin/feeds/5/secrets", "", 1, store.RoleAdmin))
	if strings.Contains(rec.Body.String(), "super-secret-value") {
		t.Fatalf("response leaked the value: %s", rec.Body.String())
	}
}

func TestPutFeedSecretStoresAndDoesNotEcho(t *testing.T) {
	svc := newFakeSecretService()
	rec := httptest.NewRecorder()
	// A cred_ref with a slash must route via the {ref...} trailing wildcard.
	handlerForSecrets(feedWithID(5), svc).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/feeds/5/secrets/secret/opensky", `{"value":"sky-token"}`, 1, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if got := svc.values["secret/opensky"]; got != "sky-token" {
		t.Fatalf("stored value = %q, want sky-token (full ref with slash)", got)
	}
	if strings.Contains(rec.Body.String(), "sky-token") {
		t.Fatalf("PUT echoed the value: %s", rec.Body.String())
	}
}

func TestPutFeedSecretRequiresValue(t *testing.T) {
	svc := newFakeSecretService()
	rec := httptest.NewRecorder()
	handlerForSecrets(feedWithID(5), svc).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/feeds/5/secrets/secret/opensky", `{"value":""}`, 1, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty value status = %d, want 400", rec.Code)
	}
	if _, ok := svc.values["secret/opensky"]; ok {
		t.Fatal("an empty value must not store anything (use DELETE to clear)")
	}
}

func TestPutFeedSecretTooLong(t *testing.T) {
	svc := newFakeSecretService()
	long := `{"value":"` + strings.Repeat("x", maxSecretValueLen+1) + `"}`
	rec := httptest.NewRecorder()
	handlerForSecrets(feedWithID(5), svc).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/feeds/5/secrets/secret/opensky", long, 1, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized value status = %d, want 400", rec.Code)
	}
}

func TestDeleteFeedSecret(t *testing.T) {
	svc := newFakeSecretService()
	svc.values["secret/opensky"] = "v1"
	h := handlerForSecrets(feedWithID(5), svc)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodDelete, "/api/admin/feeds/5/secrets/secret/opensky", "", 1, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	if _, ok := svc.values["secret/opensky"]; ok {
		t.Fatal("secret should be gone after delete")
	}
	// Re-deleting a missing ref is a clean 404.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodDelete, "/api/admin/feeds/5/secrets/secret/opensky", "", 1, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("re-delete status = %d, want 404", rec.Code)
	}
}

func TestSecretRoutesUnknownFeedIs404(t *testing.T) {
	svc := newFakeSecretService()
	h := handlerForSecrets(fakeFeeds{byID: map[int64]store.Feed{}}, svc) // no feed 9
	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/admin/feeds/9/secrets", ""},
		{http.MethodPut, "/api/admin/feeds/9/secrets/secret/opensky", `{"value":"x"}`},
		{http.MethodDelete, "/api/admin/feeds/9/secrets/secret/opensky", ""},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, adminReq(tc.method, tc.path, tc.body, 1, store.RoleAdmin))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}

func TestSecretRoutesDisabledWithoutKey(t *testing.T) {
	// A nil SecretService (no WAYFINDER_SECRET_KEY) disables the routes with 503 —
	// the capability is off, never silently storing credentials unencrypted.
	h := handlerForSecrets(feedWithID(5), nil)
	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/admin/feeds/5/secrets", ""},
		{http.MethodPut, "/api/admin/feeds/5/secrets/secret/opensky", `{"value":"x"}`},
		{http.MethodDelete, "/api/admin/feeds/5/secrets/secret/opensky", ""},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, adminReq(tc.method, tc.path, tc.body, 1, store.RoleAdmin))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s status = %d, want 503", tc.method, tc.path, rec.Code)
		}
	}
}

func TestSecretRoutesForbidNonAdmin(t *testing.T) {
	svc := newFakeSecretService()
	svc.values["secret/opensky"] = "v1"
	h := handlerForSecrets(feedWithID(5), svc)
	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/admin/feeds/5/secrets", ""},
		{http.MethodPut, "/api/admin/feeds/5/secrets/secret/opensky", `{"value":"x"}`},
		{http.MethodDelete, "/api/admin/feeds/5/secrets/secret/opensky", ""},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, adminReq(tc.method, tc.path, tc.body, 7, store.RoleUser))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s as non-admin status = %d, want 403", tc.method, tc.path, rec.Code)
		}
	}
}
