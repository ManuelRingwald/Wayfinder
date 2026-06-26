package main

import (
	"log/slog"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/cat063"
	"github.com/manuelringwald/wayfinder/pkg/cat065"
	"github.com/manuelringwald/wayfinder/pkg/feedmanager"
	"github.com/manuelringwald/wayfinder/pkg/health"
	"github.com/manuelringwald/wayfinder/pkg/receiver"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// feedConfig is one feed to receive from: its catalogue id (stamped onto every
// track for the scoped fan-out, WF2-21) and the multicast group/port to join.
type feedConfig struct {
	ID    int64
	Name  string
	Group string
	Port  int
}

// resolveFeeds decides which feeds the receivers consume. With a non-empty DB
// catalogue (multi-feed, WF2-20.2) it returns one feedConfig per row; otherwise
// — single-tenant, or a multi-tenant deployment whose catalogue is still empty —
// it falls back to the single ENV-configured feed, preserving the legacy
// single-feed behaviour so the ASD always starts with something to receive.
func resolveFeeds(catalogue []store.Feed, cfg Config) []feedConfig {
	if len(catalogue) == 0 {
		return []feedConfig{{
			ID:    cfg.FeedID,
			Name:  "default",
			Group: cfg.MulticastGroup,
			Port:  cfg.MulticastPort,
		}}
	}
	feeds := make([]feedConfig, len(catalogue))
	for i, f := range catalogue {
		feeds[i] = feedConfig{ID: f.ID, Name: f.Name, Group: f.MulticastGroup, Port: f.Port}
	}
	return feeds
}

// newReceiverFactory returns a feedmanager.Factory that builds one receiver per
// feed, each stamping its feed_id onto decoded tracks (WF2-20.1). statusHandler
// receives (feedID, status) so the per-feed health registry (AP4) knows which
// feed each CAT065 heartbeat belongs to; sensorStatusHandler receives
// (feedID, statuses) for CAT063 per-sensor status (ADR 0022). onDecodeError is
// the process-wide decode-error counter hook (ONB-5). The factory is used both at
// boot (existing catalogue) and for feeds added live from the admin API: it does
// not open sockets — the manager calls Listen/Run. An invalid feed (e.g. a
// malformed multicast group) surfaces as a build error naming the offending feed.
func newReceiverFactory(logger *slog.Logger,
	trackHandler func(int64, []cat062.DecodedTrack) error,
	statusHandler func(int64, cat065.ServiceStatus) error,
	sensorStatusHandler func(int64, []cat063.SensorStatus) error,
	onDecodeError func()) feedmanager.Factory {
	return func(f feedmanager.Feed) (feedmanager.Receiver, error) {
		fid := f.ID // capture for closures
		r, err := receiver.New(receiver.Config{
			FeedID:  fid,
			Group:   f.Group,
			Port:    f.Port,
			Logger:  logger,
			Handler: trackHandler,
			// Wrap so the signature-less handlers include the feedID via closure.
			StatusHandler: func(status cat065.ServiceStatus) error {
				return statusHandler(fid, status)
			},
			SensorStatusHandler: func(statuses []cat063.SensorStatus) error {
				return sensorStatusHandler(fid, statuses)
			},
			OnDecodeError: onDecodeError,
		})
		if err != nil {
			return nil, err
		}
		return r, nil
	}
}

// feedLifecycle adapts the feed manager + health registry to
// adminapi.FeedLifecycle (ONB-5): Start joins a newly catalogued feed's multicast
// group; Stop leaves a deleted feed's group and forgets its health so the
// dashboard stops reporting a phantom (forever-stale) feed.
type feedLifecycle struct {
	mgr      *feedmanager.Manager
	registry *health.Registry
}

func (l feedLifecycle) Start(id int64, name, group string, port int) error {
	return l.mgr.Start(feedmanager.Feed{ID: id, Name: name, Group: group, Port: port})
}

func (l feedLifecycle) Stop(id int64) bool {
	stopped := l.mgr.Stop(id)
	l.registry.Forget(id)
	return stopped
}
