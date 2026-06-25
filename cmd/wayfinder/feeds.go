package main

import (
	"fmt"
	"log/slog"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
	"github.com/manuelringwald/wayfinder/pkg/cat063"
	"github.com/manuelringwald/wayfinder/pkg/cat065"
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

// buildReceivers creates one receiver per feed, each stamping its feed_id onto
// decoded tracks (WF2-20.1). statusHandler receives (feedID, status) so the
// per-feed health registry (AP4) knows which feed each CAT065 heartbeat belongs
// to. sensorStatusHandler receives (feedID, statuses) for CAT063 per-sensor
// status updates (ADR 0022). It does not open sockets — call Listen on each.
// An invalid feed (e.g. a malformed multicast group) is a hard error naming the
// offending feed.
func buildReceivers(feeds []feedConfig, logger *slog.Logger,
	trackHandler func(int64, []cat062.DecodedTrack) error,
	statusHandler func(int64, cat065.ServiceStatus) error,
	sensorStatusHandler func(int64, []cat063.SensorStatus) error) ([]*receiver.Receiver, error) {
	recvs := make([]*receiver.Receiver, 0, len(feeds))
	for _, f := range feeds {
		fid := f.ID // capture for closure
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
		})
		if err != nil {
			return nil, fmt.Errorf("feed %q (id=%d): %w", f.Name, f.ID, err)
		}
		recvs = append(recvs, r)
	}
	return recvs, nil
}
