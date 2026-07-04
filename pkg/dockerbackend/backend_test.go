package dockerbackend

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/instance"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeClient is an in-memory ContainerClient. It assigns sequential ids and
// records the lifecycle calls so tests can assert behaviour without a daemon.
type fakeClient struct {
	containers map[string]ContainerInfo // id → info
	nextID     int
	creates    int
	removes    int
	starts     int
	stops      int
}

func newFakeClient() *fakeClient {
	return &fakeClient{containers: map[string]ContainerInfo{}}
}

func (f *fakeClient) List(_ context.Context) ([]ContainerInfo, error) {
	out := make([]ContainerInfo, 0, len(f.containers))
	for _, c := range f.containers {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeClient) Create(_ context.Context, opts CreateOptions) (string, error) {
	f.nextID++
	id := "c" + strconv.Itoa(f.nextID)
	feedID, _ := strconv.ParseInt(opts.Labels[labelFeedID], 10, 64)
	f.containers[id] = ContainerInfo{
		ID:       id,
		FeedID:   feedID,
		Running:  false, // created, not started
		SpecHash: opts.Labels[labelSpecHash],
	}
	f.creates++
	return id, nil
}

func (f *fakeClient) Start(_ context.Context, id string) error {
	c := f.containers[id]
	c.Running = true
	f.containers[id] = c
	f.starts++
	return nil
}

func (f *fakeClient) Stop(_ context.Context, id string) error {
	c := f.containers[id]
	c.Running = false
	f.containers[id] = c
	f.stops++
	return nil
}

func (f *fakeClient) Remove(_ context.Context, id string) error {
	delete(f.containers, id)
	f.removes++
	return nil
}

func spec(id int64, port int) instance.Spec {
	return instance.Spec{FeedID: id, FeedName: "feed", Group: "239.0.0.1", Port: port}
}

func newBackend(c ContainerClient) *Backend {
	return New(c, "firefly:test", "host", discardLogger())
}

func TestStartCreatesAndRunsContainer(t *testing.T) {
	ctx := context.Background()
	fc := newFakeClient()
	b := newBackend(fc)

	if err := b.Start(ctx, spec(1, 8600)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if fc.creates != 1 || fc.starts != 1 {
		t.Fatalf("creates=%d starts=%d, want 1/1", fc.creates, fc.starts)
	}
	if st, _ := b.Status(ctx, 1); st != instance.StatusRunning {
		t.Fatalf("status = %q, want running", st)
	}
}

func TestStartIsIdempotentOnSameSpec(t *testing.T) {
	ctx := context.Background()
	fc := newFakeClient()
	b := newBackend(fc)

	_ = b.Start(ctx, spec(1, 8600))
	// Re-applying the identical spec must NOT create or start anything new.
	if err := b.Start(ctx, spec(1, 8600)); err != nil {
		t.Fatalf("re-start: %v", err)
	}
	if fc.creates != 1 {
		t.Fatalf("creates = %d, want 1 (idempotent)", fc.creates)
	}
	if fc.starts != 1 {
		t.Fatalf("starts = %d, want 1 (no redundant start)", fc.starts)
	}
}

func TestStartRestartsStoppedContainer(t *testing.T) {
	ctx := context.Background()
	fc := newFakeClient()
	b := newBackend(fc)

	_ = b.Start(ctx, spec(1, 8600))
	// Container exits out of band (crash); same spec hash, but not running.
	for id, c := range fc.containers {
		c.Running = false
		fc.containers[id] = c
	}
	if err := b.Start(ctx, spec(1, 8600)); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if fc.creates != 1 {
		t.Fatalf("creates = %d, want 1 (no recreate on matching hash)", fc.creates)
	}
	if fc.starts != 2 {
		t.Fatalf("starts = %d, want 2 (restarted the stopped container)", fc.starts)
	}
	if st, _ := b.Status(ctx, 1); st != instance.StatusRunning {
		t.Fatalf("status = %q, want running", st)
	}
}

func TestStartReplacesDriftedContainer(t *testing.T) {
	ctx := context.Background()
	fc := newFakeClient()
	b := newBackend(fc)

	_ = b.Start(ctx, spec(1, 8600))
	// A changed spec (different port → different env → different hash) must
	// replace the container: stop + remove old, create + start new.
	if err := b.Start(ctx, spec(1, 8700)); err != nil {
		t.Fatalf("re-apply changed spec: %v", err)
	}
	if fc.creates != 2 || fc.removes != 1 {
		t.Fatalf("creates=%d removes=%d, want 2/1 (replaced)", fc.creates, fc.removes)
	}
	// Exactly one container remains, with the new hash.
	if len(fc.containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(fc.containers))
	}
}

func TestStopRemovesContainer(t *testing.T) {
	ctx := context.Background()
	fc := newFakeClient()
	b := newBackend(fc)

	_ = b.Start(ctx, spec(1, 8600))
	if err := b.Stop(ctx, 1); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(fc.containers) != 0 || fc.removes != 1 {
		t.Fatalf("containers=%d removes=%d, want 0/1", len(fc.containers), fc.removes)
	}
	if st, _ := b.Status(ctx, 1); st != instance.StatusStopped {
		t.Fatalf("status = %q, want stopped", st)
	}
}

func TestStopUnknownFeedIsNoop(t *testing.T) {
	fc := newFakeClient()
	if err := newBackend(fc).Stop(context.Background(), 999); err != nil {
		t.Fatalf("stop unknown: %v", err)
	}
	if fc.removes != 0 {
		t.Fatalf("removes = %d, want 0", fc.removes)
	}
}

func TestStatusFailedForExitedContainer(t *testing.T) {
	ctx := context.Background()
	fc := newFakeClient()
	b := newBackend(fc)
	_ = b.Start(ctx, spec(1, 8600))
	// Mark the container as failed (exited non-zero).
	for id, c := range fc.containers {
		c.Running = false
		c.Failed = true
		fc.containers[id] = c
	}
	if st, _ := b.Status(ctx, 1); st != instance.StatusFailed {
		t.Fatalf("status = %q, want failed", st)
	}
}

func TestRunningFeedsListsManaged(t *testing.T) {
	ctx := context.Background()
	fc := newFakeClient()
	b := newBackend(fc)
	_ = b.Start(ctx, spec(1, 8600))
	_ = b.Start(ctx, spec(2, 8601))

	feeds, err := b.RunningFeeds(ctx)
	if err != nil {
		t.Fatalf("running feeds: %v", err)
	}
	if len(feeds) != 2 {
		t.Fatalf("running feeds = %v, want 2", feeds)
	}
}

func TestStartRejectsInvalidSpec(t *testing.T) {
	fc := newFakeClient()
	if err := newBackend(fc).Start(context.Background(), instance.Spec{FeedID: 0}); err == nil {
		t.Fatal("Start accepted an invalid spec")
	}
	if fc.creates != 0 {
		t.Fatalf("an invalid spec must not create a container: creates=%d", fc.creates)
	}
}

func TestFireflyEnvMapsSpec(t *testing.T) {
	b := New(newFakeClient(), "firefly:test", "host", discardLogger())
	s := instance.Spec{
		FeedID: 1, FeedName: "f", Group: "239.0.0.5", Port: 8600,
		Coverage: &store.BBox{MinLat: 48, MinLon: 7, MaxLat: 50, MaxLon: 9},
	}
	env := b.fireflyEnv(s)
	want := map[string]bool{
		"FIREFLY_CAT062_GROUP=239.0.0.5":  false,
		"FIREFLY_CAT062_PORT=8600":        false,
		"FIREFLY_COVERAGE_BBOX=48,7,50,9": false,
		// A feed without sources carries the EXPLICIT empty source list: the
		// spawned Firefly idles with an empty sky + CAT065 heartbeat instead of
		// replaying a placeholder scene (Firefly ADR 0030).
		"FIREFLY_SOURCES=[]": false,
		// The multicast sender must be explicitly enabled — Firefly's default is off
		// (Issue #104). Without it the spawned tracker never emits the feed.
		"FIREFLY_CAT062_ENABLED=true": false,
		// A per-feed HTTP port clear of Wayfinder's 8080/8081, so the host-networked
		// tracker can bind (feed 1 → base+1 = 18081).
		"FIREFLY_PORT=18081": false,
	}
	for _, e := range env {
		if _, ok := want[e]; ok {
			want[e] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("env missing %q (got %v)", k, env)
		}
	}

	// No coverage → group/port + the always-on ENABLED, the per-feed
	// FIREFLY_PORT and the explicit empty FIREFLY_SOURCES, deterministic.
	b2 := New(newFakeClient(), "firefly:test", "host", discardLogger())
	env2 := b2.fireflyEnv(instance.Spec{FeedID: 1, Group: "239.0.0.5", Port: 8600})
	if len(env2) != 5 {
		t.Fatalf("env without coverage = %v, want 5 entries", env2)
	}
}

// TestFireflyHTTPPortIsCollisionFree pins the Issue #104 invariants: the spawned
// Firefly's HTTP port is distinct per feed and never clashes with Wayfinder's own
// ports (8081 UI / 8080 probe) — host networking makes that port process-global.
func TestFireflyHTTPPortIsCollisionFree(t *testing.T) {
	seen := map[int]int64{}
	for _, id := range []int64{1, 2, 3, 42, 254, 18000, 39999, 40000} {
		p := fireflyHTTPPort(id)
		if p < 1024 || p > 65535 {
			t.Errorf("feed %d → port %d out of range", id, p)
		}
		if p == 8080 || p == 8081 {
			t.Errorf("feed %d → port %d collides with a Wayfinder port", id, p)
		}
		if prev, dup := seen[p]; dup {
			t.Errorf("feed %d and feed %d map to the same port %d", prev, id, p)
		}
		seen[p] = id
	}
}
