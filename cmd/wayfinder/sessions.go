package main

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// Session-registry metric counters (AP7, ADR 0009 §5). Process-wide and
// churn-stable, they back the wayfinder_sessions_* series in /metrics. The
// active-session count is a gauge read straight from the registry at scrape time
// (CountActiveSessions), not accumulated here.
var (
	sessionsOpened       atomic.Int64 // logins that opened a registry session
	sessionsRejected     atomic.Int64 // logins refused by the concurrent-session limit (reject policy)
	sessionsRevoked      atomic.Int64 // sessions killed by a pause/delete (admin action)
	sessionsExpiredSwept atomic.Int64 // expired sessions removed by the janitor
)

// sessionJanitorInterval is how often expired session rows are swept. Expiry is
// already enforced at resolve time; the sweep only stops dead rows accumulating,
// so a coarse cadence is fine.
const sessionJanitorInterval = 15 * time.Minute

// countingRevoker adapts *store.SessionRepo to adminapi.SessionRevoker while
// counting revoked sessions for the metric. The admin pause/delete handlers call
// it so a suspended access's live sessions die immediately (AP7).
type countingRevoker struct{ repo *store.SessionRepo }

func (c countingRevoker) DeleteUserSessions(ctx context.Context, userID int64) (int64, error) {
	n, err := c.repo.DeleteUserSessions(ctx, userID)
	if err == nil {
		sessionsRevoked.Add(n)
	}
	return n, err
}

func (c countingRevoker) DeleteTenantSessions(ctx context.Context, tenantID int64) (int64, error) {
	n, err := c.repo.DeleteTenantSessions(ctx, tenantID)
	if err == nil {
		sessionsRevoked.Add(n)
	}
	return n, err
}

// runSessionJanitor periodically deletes expired session rows until ctx is
// cancelled (shutdown). Each sweep runs under a short timeout so a slow database
// cannot wedge the loop; a sweep error is logged and the loop continues.
func runSessionJanitor(ctx context.Context, repo *store.SessionRepo, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			n, err := repo.DeleteExpiredSessions(sweepCtx)
			cancel()
			if err != nil {
				if logger != nil {
					logger.Warn("session janitor sweep failed", slog.String("error", err.Error()))
				}
				continue
			}
			if n > 0 {
				sessionsExpiredSwept.Add(n)
				if logger != nil {
					logger.Debug("session janitor swept expired sessions", slog.Int64("count", n))
				}
			}
		}
	}
}
