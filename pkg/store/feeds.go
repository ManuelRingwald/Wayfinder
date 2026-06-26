package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/manuelringwald/wayfinder/pkg/sensorclass"
)

// Feed is one upstream CAT062/065 stream in the global catalogue (ADR 0005 §2).
// SensorMix is informational metadata (e.g. ["ADS-B"] or ["PSR","SSR","ADS-B"]),
// a property of the feed, not a per-track tag (ADR 0005 §8). Visibility is
// governed by subscriptions, not by ownership.
type Feed struct {
	ID             int64
	Name           string
	MulticastGroup string
	Port           int
	Region         *string
	SensorMix      []string
	CreatedAt      time.Time
}

const feedColumns = `id, name, multicast_group, port, region, sensor_mix, created_at`

// FeedRepo provides access to the feeds catalogue.
type FeedRepo struct {
	db *pgxpool.Pool
}

// NewFeedRepo returns a FeedRepo backed by the given pool.
func NewFeedRepo(db *pgxpool.Pool) *FeedRepo { return &FeedRepo{db: db} }

// Create inserts a feed. The sensor mix is canonicalised and validated against
// the controlled vocabulary (WF2-41): unknown classes are rejected so the
// catalogue never stores typo'd metadata; a nil/empty mix becomes an empty JSON
// array. The error wraps *sensorclass.UnknownClassError (errors.As-able).
func (r *FeedRepo) Create(ctx context.Context, name, multicastGroup string, port int, region *string, sensorMix []string) (Feed, error) {
	canonical, err := sensorclass.Canonicalize(sensorMix)
	if err != nil {
		return Feed{}, wrap("create feed: sensor_mix", err)
	}
	mix, err := toJSONB(canonical)
	if err != nil {
		return Feed{}, wrap("create feed: marshal sensor_mix", err)
	}
	const q = `INSERT INTO feeds (name, multicast_group, port, region, sensor_mix)
		VALUES ($1, $2, $3, $4, $5::jsonb) RETURNING ` + feedColumns
	f, err := scanFeed(r.db.QueryRow(ctx, q, name, multicastGroup, port, region, mix))
	if err != nil {
		return Feed{}, wrap("create feed", err)
	}
	return f, nil
}

// GetByID returns the feed with the given id, or ErrNotFound.
func (r *FeedRepo) GetByID(ctx context.Context, id int64) (Feed, error) {
	const q = `SELECT ` + feedColumns + ` FROM feeds WHERE id = $1`
	f, err := scanFeed(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return Feed{}, wrap("get feed by id", err)
	}
	return f, nil
}

// GetByName returns the feed with the given name, or ErrNotFound. Feed names are
// unique (migration 00008); the admin API uses this for a clean duplicate-name
// pre-check (409) before Create, the DB constraint remaining the race backstop.
func (r *FeedRepo) GetByName(ctx context.Context, name string) (Feed, error) {
	const q = `SELECT ` + feedColumns + ` FROM feeds WHERE name = $1`
	f, err := scanFeed(r.db.QueryRow(ctx, q, name))
	if err != nil {
		return Feed{}, wrap("get feed by name", err)
	}
	return f, nil
}

// Delete removes a feed from the catalogue. Subscriptions referencing it are
// removed by the ON DELETE CASCADE on subscriptions.feed_id (migration 00001),
// so the delete is atomic without an explicit transaction. A missing row yields
// ErrNotFound. The live receiver for this feed is stopped separately by the feed
// manager (ONB-5); this repo only owns the catalogue row.
func (r *FeedRepo) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM feeds WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return wrap("delete feed", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns all feeds ordered by id.
func (r *FeedRepo) List(ctx context.Context) ([]Feed, error) {
	const q = `SELECT ` + feedColumns + ` FROM feeds ORDER BY id`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, wrap("list feeds", err)
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, wrap("scan feed", err)
		}
		feeds = append(feeds, f)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate feeds", err)
	}
	return feeds, nil
}

// scanFeed reads a feed row, decoding the jsonb sensor_mix from raw bytes.
func scanFeed(row rowScanner) (Feed, error) {
	var (
		f   Feed
		mix []byte
	)
	if err := row.Scan(&f.ID, &f.Name, &f.MulticastGroup, &f.Port, &f.Region, &mix, &f.CreatedAt); err != nil {
		return Feed{}, err
	}
	if err := fromJSONB(mix, &f.SensorMix); err != nil {
		return Feed{}, err
	}
	return f, nil
}
