package main

import (
	"fmt"
	"log/slog"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
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
// decoded tracks (WF2-20.1) and sharing the given track/status handlers. It does
// not open sockets — call Listen on each. An invalid feed (e.g. a malformed
// multicast group) is a hard error naming the offending feed.
func buildReceivers(feeds []feedConfig, logger *slog.Logger,
	trackHandler func(int64, []cat062.DecodedTrack) error,
	statusHandler func(cat065.ServiceStatus) error) ([]*receiver.Receiver, error) {
	recvs := make([]*receiver.Receiver, 0, len(feeds))
	for _, f := range feeds {
		r, err := receiver.New(receiver.Config{
			FeedID:        f.ID,
			Group:         f.Group,
			Port:          f.Port,
			Logger:        logger,
			Handler:       trackHandler,
			StatusHandler: statusHandler,
		})
		if err != nil {
			return nil, fmt.Errorf("feed %q (id=%d): %w", f.Name, f.ID, err)
		}
		recvs = append(recvs, r)
	}
	return recvs, nil
}
