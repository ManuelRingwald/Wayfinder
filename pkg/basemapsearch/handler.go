package basemapsearch

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler serves GET /api/basemap/search?q=… .
//
// allowed decides the basemap entitlement for the request's (read-)tenant —
// enforced SERVER-side and fail-closed, unlike the cosmetic sidebar gate:
// index building consumes real server resources (thousands of tile fetches),
// so an unentitled tenant must not be able to trigger it. resolveBBox yields
// the tenant's search area (view AOI, else the 30 NM fallback box — W3).
func (s *Service) Handler(allowed func(*http.Request) bool, resolveBBox func(*http.Request) (BBox, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !allowed(r) {
			http.Error(w, `{"error":"basemap feature not enabled"}`, http.StatusForbidden)
			return
		}
		bbox, ok := resolveBBox(r)
		if !ok {
			http.Error(w, `{"error":"no search area resolvable"}`, http.StatusServiceUnavailable)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		res := s.Search(bbox, q)
		w.Header().Set("Content-Type", "application/json")
		if res.Status == "building" {
			w.WriteHeader(http.StatusAccepted)
		}
		_ = json.NewEncoder(w).Encode(res)
	}
}
