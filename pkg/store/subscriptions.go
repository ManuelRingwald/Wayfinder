package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SubscriptionRepo manages which feeds a tenant may see (the subscriptions
// table). This is the data basis of cross-tenant isolation: a track only reaches
// a tenant whose subscription covers its feed (NFR-SEC-003). The *enforcement*
// of that predicate on the live stream lives in WF2-21; this repository provides
// the authoritative subscription data it reads.
type SubscriptionRepo struct {
	db *pgxpool.Pool
}

// NewSubscriptionRepo returns a SubscriptionRepo backed by the given pool.
func NewSubscriptionRepo(db *pgxpool.Pool) *SubscriptionRepo { return &SubscriptionRepo{db: db} }

// Subscribe grants a tenant access to a feed. It is idempotent: subscribing an
// existing pair is a no-op (no error).
func (r *SubscriptionRepo) Subscribe(ctx context.Context, tenantID, feedID int64) error {
	const q = `INSERT INTO subscriptions (tenant_id, feed_id) VALUES ($1, $2)
		ON CONFLICT (tenant_id, feed_id) DO NOTHING`
	if _, err := r.db.Exec(ctx, q, tenantID, feedID); err != nil {
		return wrap("subscribe", err)
	}
	return nil
}

// Unsubscribe revokes a tenant's access to a feed. Removing a non-existent pair
// is a no-op (no error).
func (r *SubscriptionRepo) Unsubscribe(ctx context.Context, tenantID, feedID int64) error {
	const q = `DELETE FROM subscriptions WHERE tenant_id = $1 AND feed_id = $2`
	if _, err := r.db.Exec(ctx, q, tenantID, feedID); err != nil {
		return wrap("unsubscribe", err)
	}
	return nil
}

// IsSubscribed reports whether the tenant is subscribed to the feed. This is the
// authorisation check the scoped fan-out (WF2-21) builds on.
func (r *SubscriptionRepo) IsSubscribed(ctx context.Context, tenantID, feedID int64) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM subscriptions WHERE tenant_id = $1 AND feed_id = $2)`
	var ok bool
	if err := r.db.QueryRow(ctx, q, tenantID, feedID).Scan(&ok); err != nil {
		return false, wrap("is subscribed", err)
	}
	return ok, nil
}

// ListFeedIDsByTenant returns the ids of feeds a tenant is subscribed to.
func (r *SubscriptionRepo) ListFeedIDsByTenant(ctx context.Context, tenantID int64) ([]int64, error) {
	const q = `SELECT feed_id FROM subscriptions WHERE tenant_id = $1 ORDER BY feed_id`
	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrap("list feed ids", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, wrap("scan feed id", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate feed ids", err)
	}
	return ids, nil
}

// ListFeedsByTenant returns the full feed rows a tenant is subscribed to. This is
// the query the scoped fan-out (WF2-21) uses to decide which feeds' tracks a
// tenant's clients may receive.
func (r *SubscriptionRepo) ListFeedsByTenant(ctx context.Context, tenantID int64) ([]Feed, error) {
	const q = `SELECT f.id, f.name, f.multicast_group, f.port, f.region, f.sensor_mix, f.created_at
		FROM feeds f
		JOIN subscriptions s ON s.feed_id = f.id
		WHERE s.tenant_id = $1
		ORDER BY f.id`
	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrap("list feeds by tenant", err)
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
		return nil, wrap("iterate feeds by tenant", err)
	}
	return feeds, nil
}
