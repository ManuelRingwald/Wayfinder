package mapconfig

import (
	"encoding/json"
	"net/http"
)

// Resource wires one non-secret Setting to a generic admin GET/PUT endpoint. K1
// mounts concrete instances (base-map style URL, DWD URLs, …) behind
// RequireRole(admin); this helper carries the shared read/validate/save/reload
// shape so each subsystem does not re-implement it.
type Resource struct {
	Setting  *Setting
	Registry *Registry
	Domain   string // reload domain triggered after a successful save
	// Validate optionally screens a non-empty new value before it is stored
	// (e.g. ValidateFetchURL for a fetched URL). Nil = accept any value. An
	// empty value is a reset and is never validated.
	Validate func(string) error
}

// state is the JSON shape returned by GET and by a successful PUT.
type state struct {
	Value       string `json:"value"`
	Overridden  bool   `json:"overridden"`
	Default     string `json:"default"`
	ReloadError string `json:"reload_error,omitempty"`
}

type putBody struct {
	Value string `json:"value"`
}

// Handler returns the GET/PUT http.HandlerFunc for this resource.
//   - GET  → the effective value, whether it is overridden, and the env default.
//   - PUT  → validate (non-empty), store the override (empty = reset to default),
//     then trigger the reload. A reload failure is reported in `reload_error`
//     with 200 (the value is stored but the service kept its last-good config —
//     honest, non-destructive), so the operator knows it did not take effect.
func (res *Resource) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			res.writeState(w, r, "")
		case http.MethodPut:
			res.put(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (res *Resource) put(w http.ResponseWriter, r *http.Request) {
	var body putBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if body.Value != "" && res.Validate != nil {
		if err := res.Validate(body.Value); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if err := res.Setting.Set(r.Context(), body.Value); err != nil {
		http.Error(w, "could not store setting", http.StatusInternalServerError)
		return
	}
	reloadErr := ""
	if res.Registry != nil {
		if err := res.Registry.Trigger(r.Context(), res.Domain); err != nil {
			reloadErr = err.Error()
		}
	}
	res.writeState(w, r, reloadErr)
}

// writeState reads the current effective/overridden state and writes it as JSON.
func (res *Resource) writeState(w http.ResponseWriter, r *http.Request, reloadErr string) {
	value, err := res.Setting.Effective(r.Context())
	if err != nil {
		http.Error(w, "could not read setting", http.StatusInternalServerError)
		return
	}
	overridden, err := res.Setting.Overridden(r.Context())
	if err != nil {
		http.Error(w, "could not read setting", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state{
		Value:       value,
		Overridden:  overridden,
		Default:     res.Setting.Default(),
		ReloadError: reloadErr,
	})
}
