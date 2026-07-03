package adminapi

import (
	"net/http"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/airac"
)

// getAirac serves the current AIRAC cycle and the next effective date (AERO-3, ADR
// 0018). It is a pure, deterministic computation (28-day grid) — no external data —
// so the operator can schedule an OpenAIP refresh around the AIRAC change.
func (h *Handler) getAirac(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, airac.Cycle(time.Now()))
}
