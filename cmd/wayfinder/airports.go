package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/manuelringwald/wayfinder/pkg/airport"
	"github.com/manuelringwald/wayfinder/pkg/feature"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// airportsViewReader is the slice of the view-config repo the airports overlay
// needs: the effective view (tenant default or user override) whose AOI/centre
// scopes which aerodromes are returned.
type airportsViewReader interface {
	GetEffective(ctx context.Context, tenantID, userID int64) (store.ViewConfig, error)
}

// airportsFeatureGate reports whether a tenant is entitled to the airport
// overlay (the server-enforced boundary; the frontend toggle is cosmetic).
type airportsFeatureGate interface {
	HasFeature(ctx context.Context, tenantID int64, key feature.Key) bool
}

// airportMaxResults caps the returned marker count so a huge AOI (or a tenant
// without one, falling back to the radius box) cannot flood the map with
// thousands of aerodromes. The busiest sector-sized AOI holds far fewer.
const airportMaxResults = 400

// airportsHandler serves the "Flughafen" reference-point overlay (#192) as a
// GeoJSON FeatureCollection of Point markers, scoped to the caller's effective
// view AOI (its tenant default or user override), else a box around the view
// centre with the configured radius. Behind the tenant middleware; feature-gated
// per tenant (feature.Airport) — a tenant without the entitlement receives an
// empty collection, so the overlay never appears (server is the boundary, mirror
// of the aeronautical endpoints). Data is the embedded offline OurAirports
// directory (pkg/airport): no network, no key, air-gap friendly.
func airportsHandler(views airportsViewReader, gate airportsFeatureGate, radiusKM float64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		empty := func() { _ = json.NewEncoder(w).Encode(geojsonFC(nil)) }

		id, ok := tenant.FromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			empty()
			return
		}
		readTenant := tenant.ReadTenant(r.Context(), id.TenantID)
		// Server-enforced feature gate: no entitlement → empty collection.
		if gate != nil && !gate.HasFeature(r.Context(), readTenant, feature.Airport) {
			empty()
			return
		}

		// AOI: the effective view's explicit box, else a box around its centre with
		// the configured radius. Fail-soft: a backend error yields an empty overlay,
		// never a 500 (best-effort context layer).
		vc, err := views.GetEffective(r.Context(), readTenant, id.UserID)
		if err != nil {
			empty()
			return
		}
		bb := aeroBBoxFromView(vc, radiusKM)
		airports := airport.InBBox(bb.MinLat, bb.MinLon, bb.MaxLat, bb.MaxLon, airportMaxResults)

		features := make([]geojsonFeature, 0, len(airports))
		for _, a := range airports {
			features = append(features, geojsonFeature{
				Type:       "Feature",
				Geometry:   geojsonPoint{Type: "Point", Coordinates: [2]float64{a.Lon, a.Lat}},
				Properties: map[string]any{"icao": a.ICAO, "name": a.Name},
			})
		}
		_ = json.NewEncoder(w).Encode(geojsonFC(features))
	}
}

// Minimal GeoJSON encoders for the airport overlay (no external dependency).
type geojsonPoint struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

type geojsonFeature struct {
	Type       string         `json:"type"`
	Geometry   geojsonPoint   `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

type geojsonFeatureCollection struct {
	Type     string           `json:"type"`
	Features []geojsonFeature `json:"features"`
}

func geojsonFC(features []geojsonFeature) geojsonFeatureCollection {
	if features == nil {
		features = []geojsonFeature{}
	}
	return geojsonFeatureCollection{Type: "FeatureCollection", Features: features}
}
