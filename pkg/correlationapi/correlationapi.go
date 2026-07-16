// Package correlationapi is the browser-facing endpoint for manual flight-plan
// correlation (ADR 0024 §E1/§E3, Issue #245 Teil B, Häppchen 2). A controller
// overrides Firefly's automatic correlation from the ASD; the request lands here,
// is authorised, and — only then — is issued to the feed's Firefly instance via
// the command client (pkg/fireflycmd).
//
// This is the FIRST authenticated, tenant-scoped, feed-WRITE action in Wayfinder
// (everything else a tenant user does is read-only or scoped to their own
// account/view). The authorisation gate is therefore the heart of this package
// (authorize): the actor must be authenticated, NOT viewing as another tenant
// (read-only impersonation, ADR 0008 — a write under it is forbidden), and
// subscribed to the target feed (which also fails the scope-less admin, ADR 0022).
// The write always keys on the caller's own Identity.TenantID, never the
// impersonated read tenant.
package correlationapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/fireflycmd"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// maxBodyBytes caps the request body (defensive). A correlation command is a few
// bytes of JSON; 64 KiB is a generous ceiling.
const maxBodyBytes = 64 << 10

// Commander issues correlation commands to a feed's Firefly instance — the write
// subset of *fireflycmd.Client, behind an interface so the handler is unit-testable
// without a network.
type Commander interface {
	Correlate(ctx context.Context, feedID int64, trackNum uint16, callsign string) error
	SetUncorrelated(ctx context.Context, feedID int64, trackNum uint16) error
	ClearOverride(ctx context.Context, feedID int64, trackNum uint16) error
}

// SubscriptionChecker reports whether a tenant is subscribed to a feed — the
// authorisation predicate (ADR 0024 §E3). *store.SubscriptionRepo satisfies it.
type SubscriptionChecker interface {
	IsSubscribed(ctx context.Context, tenantID, feedID int64) (bool, error)
}

// Service serves the correlation command endpoint. When enabled is false (no
// command token configured, ADR 0024 §E2) the endpoint answers 503 rather than
// silently issuing token-less commands.
type Service struct {
	cmd     Commander
	subs    SubscriptionChecker
	log     *slog.Logger
	enabled bool
}

// New builds the service. enabled should reflect whether a command token is
// configured (empty token ⇒ feature off at the server edge).
func New(cmd Commander, subs SubscriptionChecker, enabled bool, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{cmd: cmd, subs: subs, log: log, enabled: enabled}
}

// setRequest is the POST /api/correlation body. A present callsign pins that plan;
// an absent callsign (nil) pins the track uncorrelated.
type setRequest struct {
	FeedID      int64   `json:"feed_id"`
	TrackNumber uint16  `json:"track_number"`
	Callsign    *string `json:"callsign"`
}

// SetHandler handles POST /api/correlation. Behind tenantMW + pwGate (any
// authenticated, non-locked user); the authorize gate does the feed-scope check.
func (s *Service) SetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.enabled {
			writeErr(w, http.StatusServiceUnavailable, "manual correlation is not enabled")
			return
		}
		var req setRequest
		if err := decodeBody(w, r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.FeedID <= 0 {
			writeErr(w, http.StatusBadRequest, "feed_id is required")
			return
		}
		tenantID, ok := s.authorize(w, r, req.FeedID)
		if !ok {
			return
		}
		var err error
		if req.Callsign != nil {
			cs := strings.TrimSpace(*req.Callsign)
			if cs == "" {
				writeErr(w, http.StatusBadRequest, "callsign must be non-empty (omit it to uncorrelate)")
				return
			}
			err = s.cmd.Correlate(r.Context(), req.FeedID, req.TrackNumber, cs)
			s.audit(r.Context(), tenantID, req.FeedID, req.TrackNumber, cs, err)
		} else {
			err = s.cmd.SetUncorrelated(r.Context(), req.FeedID, req.TrackNumber)
			s.audit(r.Context(), tenantID, req.FeedID, req.TrackNumber, "<uncorrelated>", err)
		}
		s.reply(w, err)
	}
}

// ClearHandler handles DELETE /api/correlation/{feedID}/{trackNumber}: remove the
// manual override so the automatics resume. Idempotent.
func (s *Service) ClearHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.enabled {
			writeErr(w, http.StatusServiceUnavailable, "manual correlation is not enabled")
			return
		}
		feedID, err1 := strconv.ParseInt(r.PathValue("feedID"), 10, 64)
		trackNum, err2 := strconv.ParseUint(r.PathValue("trackNumber"), 10, 16)
		if err1 != nil || err2 != nil || feedID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid feed_id or track_number")
			return
		}
		tenantID, ok := s.authorize(w, r, feedID)
		if !ok {
			return
		}
		err := s.cmd.ClearOverride(r.Context(), feedID, uint16(trackNum))
		s.audit(r.Context(), tenantID, feedID, uint16(trackNum), "<cleared>", err)
		s.reply(w, err)
	}
}

// authorize enforces the three gates (ADR 0024 §E3). On failure it writes the
// status and returns ok=false; on success it returns the acting tenant id (always
// the caller's own Identity.TenantID, never an impersonated read tenant).
func (s *Service) authorize(w http.ResponseWriter, r *http.Request, feedID int64) (int64, bool) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return 0, false
	}
	// Read-only impersonation (ADR 0008) must never write Firefly state. Reject
	// outright rather than key the write on the admin's own tenant.
	if _, impersonating := tenant.ImpersonatedTenant(r.Context()); impersonating {
		writeErr(w, http.StatusForbidden, "correlation is not allowed while viewing as another tenant")
		return 0, false
	}
	subscribed, err := s.subs.IsSubscribed(r.Context(), id.TenantID, feedID)
	if err != nil {
		s.log.Error("correlation authz: subscription check failed",
			slog.Int64("tenant_id", id.TenantID), slog.Int64("feed_id", feedID), slog.Any("err", err))
		writeErr(w, http.StatusInternalServerError, "authorization check failed")
		return 0, false
	}
	// A non-subscribed tenant — and the scope-less admin, whose own tenant holds no
	// subscriptions (ADR 0022) — is refused here.
	if !subscribed {
		writeErr(w, http.StatusForbidden, "not subscribed to this feed")
		return 0, false
	}
	return id.TenantID, true
}

// reply maps a command result to an HTTP response: 204 on success, else the
// operator-facing status from the fireflycmd sentinel (ADR 0024 §E5).
func (s *Service) reply(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, fireflycmd.ErrUnknownCallsign):
		writeErr(w, http.StatusUnprocessableEntity, "no filed flight plan for that callsign")
	case errors.Is(err, fireflycmd.ErrNoFlightPlans):
		writeErr(w, http.StatusConflict, "this feed's tracker has no flight plans configured")
	case errors.Is(err, fireflycmd.ErrUnreachable):
		writeErr(w, http.StatusBadGateway, "tracker instance unreachable")
	case errors.Is(err, fireflycmd.ErrUnauthorized):
		// The server's command token is wrong/missing — a deployment misconfig, not
		// the operator's fault. Log it; never leak the cause to the browser.
		s.log.Error("correlation command rejected by firefly — command token misconfigured")
		writeErr(w, http.StatusBadGateway, "tracker command channel misconfigured")
	default:
		s.log.Error("correlation command failed", slog.Any("err", err))
		writeErr(w, http.StatusBadGateway, "correlation command failed")
	}
}

// audit records every attempted correlation command (a safety-relevant operator
// action) with who/what and the outcome — the traceability the certification
// posture wants (CLAUDE.md §7).
func (s *Service) audit(ctx context.Context, tenantID, feedID int64, trackNum uint16, callsign string, err error) {
	lvl := slog.LevelInfo
	if err != nil {
		lvl = slog.LevelWarn
	}
	s.log.Log(ctx, lvl, "manual correlation command",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("feed_id", feedID),
		slog.Int("track_number", int(trackNum)),
		slog.String("callsign", callsign),
		slog.Any("err", err))
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
