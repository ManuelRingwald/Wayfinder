package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxViewProfilesPerUser caps how many personal view profiles a user may keep
// (ADR 0023). Enforced server-side in Create so the UI cap cannot be bypassed
// via the API.
const MaxViewProfilesPerUser = 3

// ErrProfileLimit is returned by ViewProfileRepo.Create when the user already
// holds MaxViewProfilesPerUser profiles.
var ErrProfileLimit = errors.New("store: view profile limit reached")

// ViewProfile is one user's named ASD display preset (ADR 0023): a free-form
// JSON `Settings` blob (layer toggles, range-ring config, FL filter, base map, …)
// the backend stores VERBATIM and never interprets, plus a name and an optional
// "default on login" flag. Strictly per-user. Distinct from ViewConfig, which is
// the tenant/user MAP FRAMING (centre/zoom/AOI/AoR) — the two never mix.
type ViewProfile struct {
	ID        int64
	UserID    int64
	Name      string
	Settings  json.RawMessage
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

const viewProfileColumns = `id, user_id, name, settings, is_default, created_at, updated_at`

// ViewProfileRepo provides access to the user_view_profiles table.
type ViewProfileRepo struct {
	db *pgxpool.Pool
}

// NewViewProfileRepo returns a ViewProfileRepo backed by the given pool.
func NewViewProfileRepo(db *pgxpool.Pool) *ViewProfileRepo { return &ViewProfileRepo{db: db} }

// ListByUser returns the user's profiles in creation order (stable by id).
func (r *ViewProfileRepo) ListByUser(ctx context.Context, userID int64) ([]ViewProfile, error) {
	const q = `SELECT ` + viewProfileColumns + ` FROM user_view_profiles WHERE user_id = $1 ORDER BY id`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, wrap("list view profiles", err)
	}
	defer rows.Close()
	var out []ViewProfile
	for rows.Next() {
		vp, err := scanViewProfile(rows)
		if err != nil {
			return nil, wrap("scan view profile", err)
		}
		out = append(out, vp)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate view profiles", err)
	}
	return out, nil
}

// Create inserts a new profile for the user. The MaxViewProfilesPerUser cap is
// enforced inside a transaction guarded by a per-user advisory lock, so two
// concurrent creates for the same user can never exceed the cap. When
// makeDefault is true the new profile becomes the login default, clearing any
// previous one. Returns ErrProfileLimit when the cap is already reached.
func (r *ViewProfileRepo) Create(ctx context.Context, userID int64, name string, settings json.RawMessage, makeDefault bool) (ViewProfile, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ViewProfile{}, wrap("create view profile: begin", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	// Serialise concurrent creates for THIS user (auto-released at tx end), so
	// the count-then-insert below cannot race past the cap.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
		return ViewProfile{}, wrap("create view profile: lock", err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM user_view_profiles WHERE user_id = $1`, userID).Scan(&count); err != nil {
		return ViewProfile{}, wrap("create view profile: count", err)
	}
	if count >= MaxViewProfilesPerUser {
		return ViewProfile{}, ErrProfileLimit
	}
	if makeDefault {
		if _, err := tx.Exec(ctx, `UPDATE user_view_profiles SET is_default = FALSE, updated_at = now() WHERE user_id = $1 AND is_default`, userID); err != nil {
			return ViewProfile{}, wrap("create view profile: clear default", err)
		}
	}
	vp, err := scanViewProfile(tx.QueryRow(ctx,
		`INSERT INTO user_view_profiles (user_id, name, settings, is_default)
		 VALUES ($1, $2, $3::jsonb, $4) RETURNING `+viewProfileColumns,
		userID, name, string(normalizeSettings(settings)), makeDefault,
	))
	if err != nil {
		return ViewProfile{}, wrap("create view profile", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ViewProfile{}, wrap("create view profile: commit", err)
	}
	return vp, nil
}

// Update renames and replaces the settings of the user's OWN profile. Scoped by
// (id AND user_id) so a foreign id cannot be touched (yields ErrNotFound).
func (r *ViewProfileRepo) Update(ctx context.Context, userID, id int64, name string, settings json.RawMessage) (ViewProfile, error) {
	vp, err := scanViewProfile(r.db.QueryRow(ctx,
		`UPDATE user_view_profiles SET name = $3, settings = $4::jsonb, updated_at = now()
		 WHERE id = $1 AND user_id = $2 RETURNING `+viewProfileColumns,
		id, userID, name, string(normalizeSettings(settings)),
	))
	if err != nil {
		return ViewProfile{}, wrap("update view profile", err)
	}
	return vp, nil
}

// Delete removes the user's OWN profile. Scoped by (id AND user_id); ErrNotFound
// when nothing matched.
func (r *ViewProfileRepo) Delete(ctx context.Context, userID, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM user_view_profiles WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return wrap("delete view profile", err)
	}
	if tag.RowsAffected() == 0 {
		return wrap("delete view profile", pgx.ErrNoRows)
	}
	return nil
}

// SetDefault marks the user's OWN profile as the login default, clearing any
// previous one, atomically. Scoped by (id AND user_id); ErrNotFound when the id
// is not the user's.
func (r *ViewProfileRepo) SetDefault(ctx context.Context, userID, id int64) (ViewProfile, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ViewProfile{}, wrap("set default view profile: begin", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE user_view_profiles SET is_default = FALSE, updated_at = now() WHERE user_id = $1 AND is_default`, userID); err != nil {
		return ViewProfile{}, wrap("set default view profile: clear", err)
	}
	vp, err := scanViewProfile(tx.QueryRow(ctx,
		`UPDATE user_view_profiles SET is_default = TRUE, updated_at = now()
		 WHERE id = $1 AND user_id = $2 RETURNING `+viewProfileColumns,
		id, userID,
	))
	if err != nil {
		return ViewProfile{}, wrap("set default view profile", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ViewProfile{}, wrap("set default view profile: commit", err)
	}
	return vp, nil
}

// GetDefault returns the user's default profile, or ErrNotFound when none is set.
func (r *ViewProfileRepo) GetDefault(ctx context.Context, userID int64) (ViewProfile, error) {
	vp, err := scanViewProfile(r.db.QueryRow(ctx,
		`SELECT `+viewProfileColumns+` FROM user_view_profiles WHERE user_id = $1 AND is_default`, userID,
	))
	if err != nil {
		return ViewProfile{}, wrap("get default view profile", err)
	}
	return vp, nil
}

// normalizeSettings guarantees a non-empty JSON object is stored ("{}" for a
// nil/empty blob), so the settings column is never SQL NULL.
func normalizeSettings(s json.RawMessage) json.RawMessage {
	if len(s) == 0 {
		return json.RawMessage(`{}`)
	}
	return s
}

// scanViewProfile reads one user_view_profiles row; the jsonb settings column is
// scanned as raw bytes and kept as an opaque RawMessage ("{}" if NULL/empty).
func scanViewProfile(row rowScanner) (ViewProfile, error) {
	var (
		vp       ViewProfile
		settings []byte
	)
	if err := row.Scan(&vp.ID, &vp.UserID, &vp.Name, &settings, &vp.IsDefault, &vp.CreatedAt, &vp.UpdatedAt); err != nil {
		return ViewProfile{}, err
	}
	if len(settings) == 0 {
		settings = []byte(`{}`)
	}
	vp.Settings = json.RawMessage(settings)
	return vp, nil
}
