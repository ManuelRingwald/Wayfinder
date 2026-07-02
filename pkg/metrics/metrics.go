// Package metrics renders Wayfinder's operational counters as a Prometheus
// text-exposition endpoint (REQ NFR-OBS-002): track throughput, decode
// errors, WebSocket client counts/drops, and per-tenant series (WF2-23.2).
package metrics

import (
	"fmt"
	"net/http"
	"strings"
)

// Label is a single metric label (dimension). Keep label sets bounded: high
// cardinality (per user/session) does NOT belong in metrics — it belongs in the
// structured/audit log. The per-tenant series use only the controlled `tenant`
// label (WF2-23.2).
type Label struct {
	Name  string
	Value string
}

// Metric describes one Prometheus sample (one series) to render.
type Metric struct {
	name   string
	help   string
	typ    string // "counter" or "gauge"
	value  float64
	labels []Label
}

// Handler returns an http.HandlerFunc serving the given metrics in Prometheus
// text exposition format (version 0.0.4). HELP/TYPE are emitted once per metric
// name, so several labelled series can share a name.
func Handler(metrics ...Metric) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		seen := make(map[string]bool, len(metrics))
		for _, m := range metrics {
			if !seen[m.name] {
				_, _ = fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
				_, _ = fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.typ)
				seen[m.name] = true
			}
			_, _ = fmt.Fprintf(w, "%s%s %v\n", m.name, renderLabels(m.labels), m.value)
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

// With returns a copy of the metric carrying the given labels (one series).
func (m Metric) With(labels ...Label) Metric {
	m.labels = append(append([]Label(nil), m.labels...), labels...)
	return m
}

// renderLabels formats `{k="v",…}` with values escaped per the Prometheus text
// exposition format; an empty set renders nothing.
func renderLabels(labels []Label) string {
	if len(labels) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, l := range labels {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(l.Name)
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(l.Value))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

// escapeLabelValue escapes a label value per the Prometheus text format
// (backslash, double-quote, newline). Order matters: backslash first.
func escapeLabelValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return v
}
