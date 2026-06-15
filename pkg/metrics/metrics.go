// Package metrics renders Wayfinder's operational counters as a Prometheus
// text-exposition endpoint (REQ NFR-OBS-002): track throughput, decode
// errors, and WebSocket client counts/drops.
package metrics

import (
	"fmt"
	"net/http"
)

// Metric describes one Prometheus sample to render.
type Metric struct {
	name  string
	help  string
	typ   string // "counter" or "gauge"
	value float64
}

// Handler returns an http.HandlerFunc serving the given metrics in
// Prometheus text exposition format (version 0.0.4).
func Handler(metrics ...Metric) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		for _, m := range metrics {
			fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
			fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.typ)
			fmt.Fprintf(w, "%s %v\n", m.name, m.value)
		}
	}
}

// Counter describes a monotonically increasing Prometheus counter sample.
func Counter(name, help string, value int64) Metric {
	return Metric{name: name, help: help, typ: "counter", value: float64(value)}
}

// Gauge describes a Prometheus gauge sample (a value that can go up or down).
func Gauge(name, help string, value int64) Metric {
	return Metric{name: name, help: help, typ: "gauge", value: float64(value)}
}
