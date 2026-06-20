package store

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned by repository lookups when no row matches. Callers use
// errors.Is(err, store.ErrNotFound) without needing to import pgx.
var ErrNotFound = errors.New("store: not found")

// rowScanner is satisfied by both pgx.Row (single-row queries) and pgx.Rows
// (iteration), so the per-table scan helpers serve both.
type rowScanner interface {
	Scan(dest ...any) error
}

// wrap annotates a repository error with its operation and maps pgx's "no rows"
// sentinel to ErrNotFound (preserved through the %w chain for errors.Is).
func wrap(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("store: %s: %w", op, ErrNotFound)
	}
	return fmt.Errorf("store: %s: %w", op, err)
}

// toJSONB marshals v to a JSON string for insertion into a jsonb column via a
// "$n::jsonb" cast. Passing the marshalled text (rather than a Go slice/map)
// keeps the encoding explicit and independent of pgx's default type mapping
// (which would otherwise treat e.g. []string as a Postgres text[] array).
func toJSONB(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// fromJSONB unmarshals a jsonb column (scanned as raw bytes) into dest. Empty or
// SQL-NULL values (len 0) leave dest untouched.
func fromJSONB(raw []byte, dest any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
