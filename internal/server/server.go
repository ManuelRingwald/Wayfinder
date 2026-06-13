// Package server provides the HTTP routes for Wayfinder's own
// cloud-native endpoints (health and readiness probes).
package server

import (
	"net/http"
	"sync/atomic"
)

// ReadinessCheck reports whether Wayfinder is ready to serve traffic.
//
// It is a function rather than a static value because readiness depends on
// runtime state (e.g. whether the Firefly WebSocket connection is up), which
// is wired in by later milestones.
type ReadinessCheck func() bool

// AlwaysReady is a [ReadinessCheck] that always reports ready. Used until a
// real check (e.g. the Firefly connection state) is wired in.
func AlwaysReady() bool {
	return true
}

// AtomicReadiness is a [ReadinessCheck] backed by an atomic flag, so it can
// be flipped from another goroutine (e.g. the Firefly client) without races.
type AtomicReadiness struct {
	ready atomic.Bool
}

// NewAtomicReadiness creates a readiness flag with the given initial value.
func NewAtomicReadiness(initial bool) *AtomicReadiness {
	a := &AtomicReadiness{}
	a.ready.Store(initial)
	return a
}

// Set updates the readiness state.
func (a *AtomicReadiness) Set(ready bool) {
	a.ready.Store(ready)
}

// Check implements [ReadinessCheck].
func (a *AtomicReadiness) Check() bool {
	return a.ready.Load()
}

// NewMux builds the HTTP routes for Wayfinder's own endpoints.
//
//   - /healthz: liveness probe — answers as long as the process is up.
//   - /readyz: readiness probe — delegates to ready, which later milestones
//     couple to the Firefly connection state.
func NewMux(ready ReadinessCheck) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	return mux
}
