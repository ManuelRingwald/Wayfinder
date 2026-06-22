package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// feedCommand is the `wayfinder feed <add|list>` entry point. It manages the
// feed catalogue that drives the multi-feed receiver (WF2-20.2). Until the admin
// API exists (WF2-31), this CLI is how feeds get into the catalogue.
func feedCommand(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: wayfinder feed <add|list> [flags]")
	}
	switch args[0] {
	case "add":
		return feedAddCommand(args[1:], out)
	case "list":
		return feedListCommand(args[1:], out)
	default:
		return fmt.Errorf("unknown feed subcommand %q (want add|list)", args[0])
	}
}

// openCatalogue opens the database (WAYFINDER_DB_URL) and ensures the schema is
// migrated, so the feed CLI works against a fresh database too.
func openCatalogue(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("WAYFINDER_DB_URL")
	if dsn == "" {
		return nil, errors.New("WAYFINDER_DB_URL must be set to manage the feed catalogue")
	}
	pool, err := store.Open(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := store.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return pool, nil
}

func feedAddCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("feed add", flag.ContinueOnError)
	fs.SetOutput(out)
	var (
		name      string
		group     string
		port      int
		region    string
		sensorMix string
	)
	fs.StringVar(&name, "name", "", "feed name (required)")
	fs.StringVar(&group, "group", "", "multicast group, e.g. 239.255.0.62 (required)")
	fs.IntVar(&port, "port", 8600, "multicast port")
	fs.StringVar(&region, "region", "", "region label (optional)")
	fs.StringVar(&sensorMix, "sensor-mix", "", "comma-separated sensor mix, e.g. PSR,SSR,ADS-B (optional; validated against the sensor-class catalogue, common spellings normalised)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" || group == "" {
		return errors.New("feed add: -name and -group are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := openCatalogue(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	var regionPtr *string
	if region != "" {
		regionPtr = &region
	}
	var mix []string
	for _, s := range strings.Split(sensorMix, ",") {
		if s = strings.TrimSpace(s); s != "" {
			mix = append(mix, s)
		}
	}

	f, err := store.NewFeedRepo(pool).Create(ctx, name, group, port, regionPtr, mix)
	if err != nil {
		return fmt.Errorf("create feed: %w", err)
	}
	fmt.Fprintf(out, "created feed %q (id=%d) %s:%d\n", f.Name, f.ID, f.MulticastGroup, f.Port)
	return nil
}

func feedListCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("feed list", flag.ContinueOnError)
	fs.SetOutput(out)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := openCatalogue(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	feeds, err := store.NewFeedRepo(pool).List(ctx)
	if err != nil {
		return fmt.Errorf("list feeds: %w", err)
	}
	if len(feeds) == 0 {
		fmt.Fprintln(out, "no feeds in catalogue")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tGROUP\tPORT\tSENSOR_MIX")
	for _, f := range feeds {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\n", f.ID, f.Name, f.MulticastGroup, f.Port, strings.Join(f.SensorMix, ","))
	}
	return tw.Flush()
}
