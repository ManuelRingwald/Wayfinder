package fireflycmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/instance"
)

// stubAddresser points every feed at one fixed base URL (the test server), so the
// client's HTTP behaviour can be exercised without the real per-feed port scheme.
type stubAddresser struct{ base string }

func (s stubAddresser) BaseURL(int64) string { return s.base }

// capturingServer records the last request it saw and replies with a fixed status
// and body, so a test can assert on both what was sent and how a reply is mapped.
type capturingServer struct {
	srv        *httptest.Server
	method     string
	path       string
	authHeader string
	body       []byte
	status     int
	reply      string
}

func newCapturingServer(status int, reply string) *capturingServer {
	c := &capturingServer{status: status, reply: reply}
	c.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.method = r.Method
		c.path = r.URL.Path
		c.authHeader = r.Header.Get("Authorization")
		c.body, _ = readAll(r)
		w.WriteHeader(c.status)
		_, _ = w.Write([]byte(c.reply))
	}))
	return c
}

func readAll(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	if r.Body == nil {
		return nil, nil
	}
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}

func (c *capturingServer) client(token string) *Client {
	return New(c.srv.Client(), stubAddresser{base: c.srv.URL}, token)
}

func (c *capturingServer) close() { c.srv.Close() }

func TestCorrelateSendsPlanAndBearer(t *testing.T) {
	srv := newCapturingServer(http.StatusOK, `{"track_number":42,"callsign":"DLH123","mode":"manual"}`)
	defer srv.close()

	if err := srv.client("s3cr3t").Correlate(context.Background(), 7, 42, "DLH123"); err != nil {
		t.Fatalf("Correlate: %v", err)
	}
	if srv.method != http.MethodPost || srv.path != "/correlation" {
		t.Errorf("request = %s %s, want POST /correlation", srv.method, srv.path)
	}
	if srv.authHeader != "Bearer s3cr3t" {
		t.Errorf("Authorization = %q, want Bearer s3cr3t", srv.authHeader)
	}
	var got setCorrelationReq
	if err := json.Unmarshal(srv.body, &got); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, srv.body)
	}
	if got.TrackNumber != 42 || got.Callsign == nil || *got.Callsign != "DLH123" {
		t.Errorf("body = %+v, want track 42 callsign DLH123", got)
	}
}

func TestSetUncorrelatedOmitsCallsign(t *testing.T) {
	srv := newCapturingServer(http.StatusOK, `{"track_number":42,"callsign":null,"mode":"manual"}`)
	defer srv.close()

	if err := srv.client("t").SetUncorrelated(context.Background(), 7, 42); err != nil {
		t.Fatalf("SetUncorrelated: %v", err)
	}
	// The callsign field must be omitted (Firefly reads its absence as "pin
	// uncorrelated"), never sent as an empty string.
	if strings.Contains(string(srv.body), "callsign") {
		t.Errorf("uncorrelated body carried a callsign field: %s", srv.body)
	}
}

func TestNoTokenSendsNoAuthHeader(t *testing.T) {
	srv := newCapturingServer(http.StatusOK, `{}`)
	defer srv.close()

	if err := srv.client("").SetUncorrelated(context.Background(), 1, 5); err != nil {
		t.Fatalf("SetUncorrelated: %v", err)
	}
	if srv.authHeader != "" {
		t.Errorf("Authorization = %q, want empty for an unset token", srv.authHeader)
	}
}

func TestClearOverrideIsDelete(t *testing.T) {
	srv := newCapturingServer(http.StatusOK, `{"track_number":42,"removed":true}`)
	defer srv.close()

	if err := srv.client("t").ClearOverride(context.Background(), 7, 42); err != nil {
		t.Fatalf("ClearOverride: %v", err)
	}
	if srv.method != http.MethodDelete || srv.path != "/correlation/42" {
		t.Errorf("request = %s %s, want DELETE /correlation/42", srv.method, srv.path)
	}
}

func TestListOverridesDecodes(t *testing.T) {
	srv := newCapturingServer(http.StatusOK, `[{"track_number":1,"callsign":"DLH1"},{"track_number":2,"callsign":null}]`)
	defer srv.close()

	got, err := srv.client("t").ListOverrides(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListOverrides: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d overrides, want 2", len(got))
	}
	if got[0].TrackNumber != 1 || got[0].Callsign == nil || *got[0].Callsign != "DLH1" {
		t.Errorf("override[0] = %+v, want track 1 callsign DLH1", got[0])
	}
	if got[1].TrackNumber != 2 || got[1].Callsign != nil {
		t.Errorf("override[1] = %+v, want track 2 uncorrelated", got[1])
	}
}

func TestStatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusConflict, ErrNoFlightPlans},
		{http.StatusUnprocessableEntity, ErrUnknownCallsign},
	}
	for _, tc := range cases {
		srv := newCapturingServer(tc.status, "err")
		err := srv.client("t").Correlate(context.Background(), 7, 42, "DLH123")
		srv.close()
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d → %v, want %v", tc.status, err, tc.want)
		}
	}
}

func TestUnexpectedStatusIsError(t *testing.T) {
	srv := newCapturingServer(http.StatusInternalServerError, "boom")
	defer srv.close()

	err := srv.client("t").Correlate(context.Background(), 7, 42, "DLH123")
	if err == nil || errors.Is(err, ErrUnknownCallsign) || errors.Is(err, ErrNoFlightPlans) {
		t.Errorf("500 → %v, want a generic non-sentinel error", err)
	}
}

func TestUnreachableInstance(t *testing.T) {
	// A server that is immediately closed → the dial fails → ErrUnreachable.
	srv := newCapturingServer(http.StatusOK, `{}`)
	c := srv.client("t")
	srv.close()

	err := c.Correlate(context.Background(), 7, 42, "DLH123")
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("closed instance → %v, want ErrUnreachable", err)
	}
}

func TestContextCancellationIsUnreachable(t *testing.T) {
	srv := newCapturingServer(http.StatusOK, `{}`)
	defer srv.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the call
	err := srv.client("t").Correlate(ctx, 7, 42, "DLH123")
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("cancelled context → %v, want ErrUnreachable", err)
	}
}

func TestHostLoopbackAddresser(t *testing.T) {
	a := HostLoopbackAddresser{}
	want := "http://127.0.0.1:" + strconv.Itoa(instance.FireflyHTTPPort(42))
	if got := a.BaseURL(42); got != want {
		t.Errorf("BaseURL(42) → %q, want %q", got, want)
	}
	// A custom host is honoured (non-loopback single-host layout).
	custom := HostLoopbackAddresser{Host: "10.0.0.5"}
	if got := custom.BaseURL(1); !strings.HasPrefix(got, "http://10.0.0.5:") {
		t.Errorf("custom host BaseURL(1) → %q, want http://10.0.0.5:…", got)
	}
}
