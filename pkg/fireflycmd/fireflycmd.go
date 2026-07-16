// Package fireflycmd is Wayfinder's command back-channel to a per-feed Firefly
// tracker instance (ADR 0024, Issue #245 Teil B). Wayfinder is otherwise a pure
// CAT062 multicast *consumer*; this is the only path that *writes* to Firefly —
// the manual flight-plan correlation command a controller issues to override
// Firefly's automatic correlation (Firefly ADR 0038/0039).
//
// The client is deliberately narrow and best-effort, mirroring pkg/weather: an
// injected, timed *http.Client; every call is context-bounded, its response body
// is size-limited, and it carries the deployment-wide Bearer token (ADR 0024 §E2).
// It never touches the track path or readiness — a correlation command that fails
// is surfaced to the operator, never a system fault.
//
// ADR 0024 §E1 places the command issuer at the browser-facing server (this
// package is imported there, not by the orchestrator): the command needs a
// synchronous 422/409 answer for the context menu, and issuing a data command to
// a *running* Firefly is not the container-lifecycle privilege ADR 0012 §6 fences
// off. The per-feed address comes from the SDK-free instance.FireflyHTTPPort
// helper (ADR 0024 §E4), so the server need not import the orchestrator-private
// dockerbackend package.
package fireflycmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/instance"
)

// TokenEnvVar names the deployment-wide command token (ADR 0024 §E2): the same
// value on the server (to send) and injected into every Firefly instance as
// FIREFLY_WS_TOKEN (to verify). An empty token disables the command feature at the
// server edge — the wiring (Häppchen 2) treats an unset value as "feature off".
const TokenEnvVar = "WAYFINDER_FIREFLY_COMMAND_TOKEN"

// defaultTimeout is the best-effort ceiling on a single command round-trip (the
// house default, matching pkg/weather). A correlation command is a tiny local
// request; a slow one means the instance is unhealthy, not that we should wait.
const defaultTimeout = 15 * time.Second

// maxResponseBytes caps the response body read into memory (defensive consumer).
// The correlation responses are a few bytes of JSON; 1 MiB is a generous ceiling.
const maxResponseBytes = 1 << 20

// Sentinel errors map Firefly's correlation-API responses (Firefly ADR 0039) to
// stable values the caller (Häppchen 2) translates into operator-facing messages.
// Use errors.Is to classify.
var (
	// ErrNoFlightPlans is Firefly's 409 — the instance has no flight plans
	// configured, so correlation is inoperative for that feed.
	ErrNoFlightPlans = errors.New("fireflycmd: instance has no flight plans configured")
	// ErrUnknownCallsign is Firefly's 422 — no filed plan matches the callsign.
	ErrUnknownCallsign = errors.New("fireflycmd: no filed plan for that callsign")
	// ErrUnauthorized is Firefly's 401 — the command token is missing or wrong.
	ErrUnauthorized = errors.New("fireflycmd: command rejected (bad or missing token)")
	// ErrUnreachable wraps a transport failure (connection refused, timeout, …):
	// the Firefly instance could not be reached at all.
	ErrUnreachable = errors.New("fireflycmd: firefly instance unreachable")
)

// Addresser resolves a feed id to the base URL of that feed's Firefly command API
// (ADR 0024 §E4). It is an interface so the (future) Kubernetes backend can supply
// a Service address instead of the host-loopback scheme, without touching callers.
type Addresser interface {
	BaseURL(feedID int64) string
}

// HostLoopbackAddresser addresses each feed's Firefly on the host loopback at the
// deterministic port instance.FireflyHTTPPort(feedID) — valid in the single-host,
// host-networked harness (docker-compose.orchestrated.yml). This is the honest
// limit of the first implementation (ADR 0024): a non-host-networked or Kubernetes
// deployment needs a different Addresser. Host defaults to 127.0.0.1.
type HostLoopbackAddresser struct {
	Host string
}

// BaseURL returns "http://<host>:<port>" for the given feed.
func (a HostLoopbackAddresser) BaseURL(feedID int64) string {
	host := a.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return "http://" + host + ":" + strconv.Itoa(instance.FireflyHTTPPort(feedID))
}

// Client issues manual-correlation commands to per-feed Firefly instances.
type Client struct {
	http      *http.Client
	addresser Addresser
	token     string
}

// New builds a command client. A nil httpClient falls back to a timed client
// (defaultTimeout) — best-effort, never blocking. An empty token sends no
// Authorization header (a Firefly without a configured token accepts the command;
// used in dev/test). The addresser must be non-nil.
func New(httpClient *http.Client, addresser Addresser, token string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{http: httpClient, addresser: addresser, token: token}
}

// setCorrelationReq is the POST /correlation body. A nil Callsign is omitted,
// which Firefly (serde default) reads as "pin uncorrelated".
type setCorrelationReq struct {
	TrackNumber uint16  `json:"track_number"`
	Callsign    *string `json:"callsign,omitempty"`
}

// Correlate pins the filed plan identified by callsign onto wire track trackNum on
// feedID's Firefly (POST /correlation). Returns ErrUnknownCallsign when no such
// plan is filed, ErrNoFlightPlans when the instance carries no plans at all.
func (c *Client) Correlate(ctx context.Context, feedID int64, trackNum uint16, callsign string) error {
	return c.postCorrelation(ctx, feedID, setCorrelationReq{TrackNumber: trackNum, Callsign: &callsign})
}

// SetUncorrelated pins trackNum as uncorrelated on feedID's Firefly (POST
// /correlation with no callsign), locking the automatics out until cleared.
func (c *Client) SetUncorrelated(ctx context.Context, feedID int64, trackNum uint16) error {
	return c.postCorrelation(ctx, feedID, setCorrelationReq{TrackNumber: trackNum})
}

func (c *Client) postCorrelation(ctx context.Context, feedID int64, body setCorrelationReq) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("fireflycmd: marshal command: %w", err)
	}
	url := c.addresser.BaseURL(feedID) + "/correlation"
	_, err = c.do(ctx, http.MethodPost, url, bytes.NewReader(buf))
	return err
}

// ClearOverride removes any manual override for trackNum on feedID's Firefly
// (DELETE /correlation/{n}), so the automatics resume. Idempotent: clearing a
// track with no override is not an error.
func (c *Client) ClearOverride(ctx context.Context, feedID int64, trackNum uint16) error {
	url := c.addresser.BaseURL(feedID) + "/correlation/" + strconv.FormatUint(uint64(trackNum), 10)
	_, err := c.do(ctx, http.MethodDelete, url, nil)
	return err
}

// Override is one manual-override entry from ListOverrides. A nil Callsign means
// the track is pinned uncorrelated.
type Override struct {
	TrackNumber uint16  `json:"track_number"`
	Callsign    *string `json:"callsign"`
}

// ListOverrides returns the manual overrides currently in force on feedID's
// Firefly (GET /correlation). An instance without flight plans answers an empty
// list — a read is never ErrNoFlightPlans.
func (c *Client) ListOverrides(ctx context.Context, feedID int64) ([]Override, error) {
	url := c.addresser.BaseURL(feedID) + "/correlation"
	body, err := c.do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	var out []Override
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("fireflycmd: decode overrides: %w", err)
	}
	return out, nil
}

// do issues one request, mapping Firefly's status codes to the sentinel errors and
// returning the (size-limited) response body on success. A transport failure is
// wrapped as ErrUnreachable.
func (c *Client) do(ctx context.Context, method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("fireflycmd: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	// Bearer only (ADR 0024 §E2 / Firefly authorize_command): no query fallback for
	// a state-changing request, and no Origin header — a server-to-server client
	// passes Firefly's origin allowlist on the token alone.
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return respBody, nil
	case resp.StatusCode == http.StatusUnauthorized: // 401
		return nil, ErrUnauthorized
	case resp.StatusCode == http.StatusConflict: // 409
		return nil, ErrNoFlightPlans
	case resp.StatusCode == http.StatusUnprocessableEntity: // 422
		return nil, ErrUnknownCallsign
	default:
		return nil, fmt.Errorf("fireflycmd: unexpected status %d from firefly", resp.StatusCode)
	}
}
