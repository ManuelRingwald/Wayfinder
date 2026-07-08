package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BBox is a geographic bounding box (the area of interest of a view).
type BBox struct {
	MinLat float64 `json:"min_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLat float64 `json:"max_lat"`
	MaxLon float64 `json:"max_lon"`
}

// ViewConfig is a tenant's (or a user's) map view: centre, zoom, optional
// area-of-interest and flight-level band, and a free-form layer toggle map. A
// row with UserID == nil is the tenant default; a row with a UserID is that
// user's override (ADR 0006 schema). FLMin/FLMax and AOI are optional.
type ViewConfig struct {
	ID        int64
	TenantID  int64
	UserID    *int64
	CenterLat float64
	CenterLon float64
	Zoom      float64
	AOI       *BBox
	FLMin     *int
	FLMax     *int
	Layers    map[string]bool
	// ICAO is an optional per-tenant location indicator shown in the ASD header
	// (e.g. "EDGG" / "EDGG·KTG"). nil = unset (the header omits it). It is display
	// config, not track data — CAT062 carries no sector identity.
	ICAO *string
	// QNHICAO is the optional per-tenant aerodrome ICAO whose QNH (altimeter
	// setting, hPa) is shown in the header infobox (CBD-3, ADR 0016). Unlike ICAO
	// this is a REAL location indicator (e.g. "EDDH") fed to the NOAA/AWC METAR
	// poller — not a free-form label. nil = unset (no QNH for this tenant).
	QNHICAO *string
	// AoRAirspaceIDs is the tenant's Area of Responsibility: the set of OpenAIP
	// airspace ids (stable `_id`, surfaced since ASD-014.1) that make up the
	// controlled volumes (CTR/TMA) the ASD highlights (ADR 0021, Ebene 2). The id
	// is the robust key — airspace names drift per AIRAC. nil/empty = no AoR set.
	// Display config, not track data (CAT062 carries no sector identity).
	AoRAirspaceIDs []string
	UpdatedAt      time.Time
}

const viewConfigColumns = `id, tenant_id, user_id, center_lat, center_lon, zoom, aoi, fl_min, fl_max, layers, icao, qnh_icao, aor_airspace_ids, updated_at`

// ViewConfigRepo provides access to the view_configs table.
type ViewConfigRepo struct {
	db *pgxpool.Pool
}

// NewViewConfigRepo returns a ViewConfigRepo backed by the given pool.
func NewViewConfigRepo(db *pgxpool.Pool) *ViewConfigRepo { return &ViewConfigRepo{db: db} }

// UpsertTenantDefault stores (or replaces) the tenant's default view — the row
// with no user override. Idempotent via the partial unique index on
// (tenant_id) WHERE user_id IS NULL.
func (r *ViewConfigRepo) UpsertTenantDefault(ctx context.Context, tenantID int64, vc ViewConfig) (ViewConfig, error) {
	aoi, aor, layers, err := viewJSONParams(vc)
	if err != nil {
		return ViewConfig{}, wrap("upsert tenant view: marshal", err)
	}
	const q = `INSERT INTO view_configs (tenant_id, user_id, center_lat, center_lon, zoom, aoi, fl_min, fl_max, layers, icao, qnh_icao, aor_airspace_ids)
		VALUES ($1, NULL, $2, $3, $4, $5::jsonb, $6, $7, $8::jsonb, $9, $10, $11::jsonb)
		ON CONFLICT (tenant_id) WHERE user_id IS NULL
		DO UPDATE SET center_lat = EXCLUDED.center_lat, center_lon = EXCLUDED.center_lon,
			zoom = EXCLUDED.zoom, aoi = EXCLUDED.aoi, fl_min = EXCLUDED.fl_min,
			fl_max = EXCLUDED.fl_max, layers = EXCLUDED.layers, icao = EXCLUDED.icao,
			qnh_icao = EXCLUDED.qnh_icao, aor_airspace_ids = EXCLUDED.aor_airspace_ids, updated_at = now()
		RETURNING ` + viewConfigColumns
	out, err := scanViewConfig(r.db.QueryRow(ctx, q, tenantID, vc.CenterLat, vc.CenterLon, vc.Zoom, aoi, vc.FLMin, vc.FLMax, layers, vc.ICAO, vc.QNHICAO, aor))
	if err != nil {
		return ViewConfig{}, wrap("upsert tenant view", err)
	}
	return out, nil
}

// UpsertUserOverride stores (or replaces) a user's view override. Idempotent via
// the partial unique index on (user_id) WHERE user_id IS NOT NULL (migration 2).
func (r *ViewConfigRepo) UpsertUserOverride(ctx context.Context, tenantID, userID int64, vc ViewConfig) (ViewConfig, error) {
	aoi, aor, layers, err := viewJSONParams(vc)
	if err != nil {
		return ViewConfig{}, wrap("upsert user view: marshal", err)
	}
	const q = `INSERT INTO view_configs (tenant_id, user_id, center_lat, center_lon, zoom, aoi, fl_min, fl_max, layers, icao, qnh_icao, aor_airspace_ids)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9::jsonb, $10, $11, $12::jsonb)
		ON CONFLICT (user_id) WHERE user_id IS NOT NULL
		DO UPDATE SET center_lat = EXCLUDED.center_lat, center_lon = EXCLUDED.center_lon,
			zoom = EXCLUDED.zoom, aoi = EXCLUDED.aoi, fl_min = EXCLUDED.fl_min,
			fl_max = EXCLUDED.fl_max, layers = EXCLUDED.layers, icao = EXCLUDED.icao,
			qnh_icao = EXCLUDED.qnh_icao, aor_airspace_ids = EXCLUDED.aor_airspace_ids, updated_at = now()
		RETURNING ` + viewConfigColumns
	out, err := scanViewConfig(r.db.QueryRow(ctx, q, tenantID, userID, vc.CenterLat, vc.CenterLon, vc.Zoom, aoi, vc.FLMin, vc.FLMax, layers, vc.ICAO, vc.QNHICAO, aor))
	if err != nil {
		return ViewConfig{}, wrap("upsert user view", err)
	}
	return out, nil
}

// GetTenantDefault returns the tenant's default view, or ErrNotFound.
func (r *ViewConfigRepo) GetTenantDefault(ctx context.Context, tenantID int64) (ViewConfig, error) {
	const q = `SELECT ` + viewConfigColumns + ` FROM view_configs WHERE tenant_id = $1 AND user_id IS NULL`
	vc, err := scanViewConfig(r.db.QueryRow(ctx, q, tenantID))
	if err != nil {
		return ViewConfig{}, wrap("get tenant view", err)
	}
	return vc, nil
}

// GetUserOverride returns a user's view override, or ErrNotFound.
func (r *ViewConfigRepo) GetUserOverride(ctx context.Context, userID int64) (ViewConfig, error) {
	const q = `SELECT ` + viewConfigColumns + ` FROM view_configs WHERE user_id = $1`
	vc, err := scanViewConfig(r.db.QueryRow(ctx, q, userID))
	if err != nil {
		return ViewConfig{}, wrap("get user view", err)
	}
	return vc, nil
}

// GetEffective returns the view a user should see: their override if present,
// otherwise the tenant default. ErrNotFound only if neither exists.
func (r *ViewConfigRepo) GetEffective(ctx context.Context, tenantID, userID int64) (ViewConfig, error) {
	vc, err := r.GetUserOverride(ctx, userID)
	if err == nil {
		return vc, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return ViewConfig{}, err
	}
	return r.GetTenantDefault(ctx, tenantID)
}

// DistinctQNHICAOs returns the set of aerodrome ICAOs configured across all view
// configs (tenant defaults and user overrides), de-duplicated, skipping NULL/empty
// (CBD-3). It is the poll set for the global QNH service: the union of every
// tenant's header aerodrome, so one background poller keeps all of them warm. The
// caller normalises casing; the poll set is small (one airport per tenant).
func (r *ViewConfigRepo) DistinctQNHICAOs(ctx context.Context) ([]string, error) {
	const q = `SELECT DISTINCT qnh_icao FROM view_configs WHERE qnh_icao IS NOT NULL AND qnh_icao <> ''`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, wrap("list qnh icaos", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, wrap("scan qnh icao", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate qnh icaos", err)
	}
	return out, nil
}

// viewJSONParams prepares the jsonb parameters: aoi and aor are nil (SQL NULL)
// when unset (no area of interest / no AoR airspaces), else their JSON; layers is
// always a JSON object ("{}" when empty/nil).
func viewJSONParams(vc ViewConfig) (aoi any, aor any, layers string, err error) {
	if vc.AOI != nil {
		s, e := toJSONB(vc.AOI)
		if e != nil {
			return nil, nil, "", e
		}
		aoi = s
	}
	if len(vc.AoRAirspaceIDs) > 0 {
		s, e := toJSONB(vc.AoRAirspaceIDs)
		if e != nil {
			return nil, nil, "", e
		}
		aor = s
	}
	lay := vc.Layers
	if lay == nil {
		lay = map[string]bool{}
	}
	layers, err = toJSONB(lay)
	return aoi, aor, layers, err
}

// scanViewConfig reads a view_configs row, decoding the jsonb aoi/layers columns.
func scanViewConfig(row rowScanner) (ViewConfig, error) {
	var (
		vc     ViewConfig
		aoi    []byte
		layers []byte
		aor    []byte
	)
	if err := row.Scan(&vc.ID, &vc.TenantID, &vc.UserID, &vc.CenterLat, &vc.CenterLon, &vc.Zoom,
		&aoi, &vc.FLMin, &vc.FLMax, &layers, &vc.ICAO, &vc.QNHICAO, &aor, &vc.UpdatedAt); err != nil {
		return ViewConfig{}, err
	}
	if err := fromJSONB(aoi, &vc.AOI); err != nil {
		return ViewConfig{}, err
	}
	vc.Layers = map[string]bool{}
	if err := fromJSONB(layers, &vc.Layers); err != nil {
		return ViewConfig{}, err
	}
	if err := fromJSONB(aor, &vc.AoRAirspaceIDs); err != nil {
		return ViewConfig{}, err
	}
	return vc, nil
}
