package mapconfig

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// ReloadFunc re-reads the effective config for one subsystem and re-applies it to
// the running service. It MUST be defensive: on failure it keeps the service's
// last-good configuration and returns an error (the operator sees it), never
// panics and never tears the service down — operator input must not be able to
// break a running scope (CLAUDE §7).
type ReloadFunc func(ctx context.Context) error

// Registry dispatches "config changed" to the owning subsystem's ReloadFunc. A
// subsystem (weather, base map, coverage, …) registers under a domain name at
// wiring time; the admin PUT handler calls Trigger(domain) after a successful
// save. Decoupling keeps the admin API free of direct service references.
type Registry struct {
	logger *slog.Logger
	mu     sync.RWMutex
	fns    map[string]ReloadFunc
}

// NewRegistry builds an empty Registry. A nil logger falls back to slog.Default.
func NewRegistry(logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{logger: logger, fns: make(map[string]ReloadFunc)}
}

// Register binds a domain to its reload function. A later Register for the same
// domain replaces the earlier one (last wiring wins). A nil fn is ignored.
func (r *Registry) Register(domain string, fn ReloadFunc) {
	if fn == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fns[domain] = fn
}

// Trigger invokes the domain's reload function and returns its error (so the
// admin response can surface a bad new config). An unknown domain is a no-op with
// a logged warning — a config plane without a wired service should degrade, not
// fail the save. A reload error is logged and returned but never propagated as a
// panic; the service keeps its last-good config.
func (r *Registry) Trigger(ctx context.Context, domain string) error {
	r.mu.RLock()
	fn, ok := r.fns[domain]
	r.mu.RUnlock()
	if !ok {
		r.logger.Warn("mapconfig reload: no service registered for domain", slog.String("domain", domain))
		return nil
	}
	if err := fn(ctx); err != nil {
		r.logger.Error("mapconfig reload failed; keeping last-good config",
			slog.String("domain", domain), slog.String("error", err.Error()))
		return fmt.Errorf("reload %s: %w", domain, err)
	}
	r.logger.Info("mapconfig reload applied", slog.String("domain", domain))
	return nil
}

// Domains returns the registered domain names (diagnostics/tests). Unordered.
func (r *Registry) Domains() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.fns))
	for d := range r.fns {
		out = append(out, d)
	}
	return out
}
