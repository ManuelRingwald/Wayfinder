package instance

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int       { return &i }

func TestSpecFromFeed(t *testing.T) {
	cov := &store.BBox{MinLat: 48, MinLon: 7, MaxLat: 50, MaxLon: 9}
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: cov, CredRef: ptrStr("secret/sky")},
		{Type: store.SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)},
		{Type: store.SourceFLARMAPRS, BBox: cov, CredRef: ptrStr("secret/ogn")},
	}
	f := Feed{ID: 3, Name: "speyer", Group: "239.255.0.70", Port: 8700}

	spec := SpecFromFeed(f, sources, cov)

	if spec.FeedID != 3 || spec.FeedName != "speyer" || spec.Group != "239.255.0.70" || spec.Port != 8700 {
		t.Fatalf("spec identity/endpoint wrong: %+v", spec)
	}
	if spec.Coverage != cov || len(spec.Sources) != 3 {
		t.Fatalf("spec coverage/sources wrong: %+v", spec)
	}
	// Secret refs are distinct and sorted; values never appear, only the handles.
	want := []string{"secret/ogn", "secret/sky"}
	if len(spec.SecretRefs) != len(want) {
		t.Fatalf("secret refs = %v, want %v", spec.SecretRefs, want)
	}
	for i, r := range want {
		if spec.SecretRefs[i] != r {
			t.Fatalf("secret refs = %v, want %v (sorted, deduped)", spec.SecretRefs, want)
		}
	}
}

func TestSpecFromFeedNoSecretsNoCoverage(t *testing.T) {
	sources := store.SourceConfig{{Type: store.SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)}}
	spec := SpecFromFeed(Feed{ID: 1, Name: "ffm", Group: "239.0.0.1", Port: 8600}, sources, nil)
	if spec.SecretRefs != nil {
		t.Errorf("SecretRefs = %v, want nil", spec.SecretRefs)
	}
	if spec.Coverage != nil {
		t.Errorf("Coverage = %+v, want nil", spec.Coverage)
	}
}

func TestSpecFromFeedDedupesSecretRefs(t *testing.T) {
	bb := &store.BBox{MinLat: 1, MinLon: 1, MaxLat: 2, MaxLon: 2}
	sources := store.SourceConfig{
		{Type: store.SourceADSBOpenSky, BBox: bb, CredRef: ptrStr("secret/shared")},
		{Type: store.SourceFLARMAPRS, BBox: bb, CredRef: ptrStr("secret/shared")},
	}
	spec := SpecFromFeed(Feed{ID: 1, Name: "x", Group: "239.0.0.1", Port: 8600}, sources, nil)
	if len(spec.SecretRefs) != 1 || spec.SecretRefs[0] != "secret/shared" {
		t.Fatalf("SecretRefs = %v, want one [secret/shared]", spec.SecretRefs)
	}
}

func TestSpecValidate(t *testing.T) {
	valid := Spec{FeedID: 1, FeedName: "x", Group: "239.0.0.1", Port: 8600}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	bad := []Spec{
		{FeedID: 0, Group: "239.0.0.1", Port: 8600},  // no feed id
		{FeedID: 1, Group: "", Port: 8600},           // no group
		{FeedID: 1, Group: "239.0.0.1", Port: 0},     // bad port
		{FeedID: 1, Group: "239.0.0.1", Port: 70000}, // bad port
	}
	for i, s := range bad {
		if err := s.Validate(); err == nil {
			t.Errorf("bad spec[%d] = %+v accepted, want error", i, s)
		}
	}
}

func TestSpecEndpoint(t *testing.T) {
	s := Spec{Group: "239.255.0.70", Port: 8700}
	if s.Endpoint() != "239.255.0.70:8700" {
		t.Fatalf("Endpoint = %q", s.Endpoint())
	}
}

func TestMemoryBackendLifecycle(t *testing.T) {
	ctx := context.Background()
	b := NewMemoryBackend()
	spec := Spec{FeedID: 3, FeedName: "speyer", Group: "239.255.0.70", Port: 8700}

	// Unknown feed is stopped.
	if st, _ := b.Status(ctx, 3); st != StatusStopped {
		t.Fatalf("initial status = %q, want stopped", st)
	}

	if err := b.Start(ctx, spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	if st, _ := b.Status(ctx, 3); st != StatusRunning {
		t.Fatalf("status after start = %q, want running", st)
	}
	got, ok := b.RunningSpec(3)
	if !ok || got.Group != "239.255.0.70" {
		t.Fatalf("running spec = %+v, ok=%v", got, ok)
	}

	// Idempotent: starting the same spec again keeps it running.
	if err := b.Start(ctx, spec); err != nil {
		t.Fatalf("re-start: %v", err)
	}
	if feeds := b.RunningFeeds(); len(feeds) != 1 {
		t.Fatalf("running feeds = %v, want exactly one", feeds)
	}

	// A changed spec replaces the running one.
	spec2 := spec
	spec2.Port = 8701
	if err := b.Start(ctx, spec2); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if got, _ := b.RunningSpec(3); got.Port != 8701 {
		t.Fatalf("spec not replaced: %+v", got)
	}

	if err := b.Stop(ctx, 3); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if st, _ := b.Status(ctx, 3); st != StatusStopped {
		t.Fatalf("status after stop = %q, want stopped", st)
	}
	// Stopping an unknown feed is a no-op.
	if err := b.Stop(ctx, 999); err != nil {
		t.Fatalf("stop unknown: %v", err)
	}
}

func TestMemoryBackendRejectsInvalidSpec(t *testing.T) {
	b := NewMemoryBackend()
	if err := b.Start(context.Background(), Spec{FeedID: 0}); err == nil {
		t.Fatal("Start accepted an invalid spec")
	}
	if feeds := b.RunningFeeds(); len(feeds) != 0 {
		t.Fatalf("an invalid spec must not be recorded as running: %v", feeds)
	}
}

func TestMemoryBackendStartHookFailure(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("backend unavailable")
	b := NewMemoryBackend().WithStartHook(func(Spec) error { return boom })
	spec := Spec{FeedID: 5, FeedName: "x", Group: "239.0.0.1", Port: 8600}

	if err := b.Start(ctx, spec); !errors.Is(err, boom) {
		t.Fatalf("Start error = %v, want boom", err)
	}
	if st, _ := b.Status(ctx, 5); st != StatusFailed {
		t.Fatalf("status after failed start = %q, want failed", st)
	}
	if _, ok := b.RunningSpec(5); ok {
		t.Fatal("a failed instance must not be recorded as running")
	}

	// Clearing the hook and re-starting recovers to running (clears failed mark).
	b.WithStartHook(nil)
	if err := b.Start(ctx, spec); err != nil {
		t.Fatalf("recovery start: %v", err)
	}
	if st, _ := b.Status(ctx, 5); st != StatusRunning {
		t.Fatalf("status after recovery = %q, want running", st)
	}
}

// TestMemoryBackendConcurrent exercises concurrent Start/Stop/Status under -race.
func TestMemoryBackendConcurrent(t *testing.T) {
	ctx := context.Background()
	b := NewMemoryBackend()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			spec := Spec{FeedID: id, FeedName: "f", Group: "239.0.0.1", Port: 8600}
			_ = b.Start(ctx, spec)
			_, _ = b.Status(ctx, id)
			_ = b.Stop(ctx, id)
		}(int64(i%10 + 1))
	}
	wg.Wait()
}
