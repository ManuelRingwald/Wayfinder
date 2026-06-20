package store

import (
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
