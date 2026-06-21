package broadcast

import (
	"context"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// This file is the cross-tenant isolation gate (WF2-22, NFR-SEC-003): property
// and fuzz tests that go beyond the point negative tests in 21.1/21.2 to assert,
// over many generated tenant/subscription/view constellations and track streams,
// the two invariants of the scoped fan-out:
//
//   - Isolation (no false positive across the boundary): a client never receives
//     a track from a feed it does not subscribe to, nor one outside its view.
//   - Safety/completeness (no false negative): a track in an allowed feed and
//     inside the view — or with no flight level (fail-open) — is delivered.
//
// The reference oracle below is an independent, deliberately simple restatement
// of the intended predicate; the tests assert the real delivery path agrees with
// it (differential testing).

// viewAdmitsOracle is the reference view predicate: inclusive AOI bounds, and a
// fail-open flight-level band (a track with no measured level is always admitted).
// Independent of ViewFilter.admits so a logic change in one is caught against the
// other.
func viewAdmitsOracle(v *ViewFilter, lat, lon float64, fl *float64) bool {
	if v == nil {
		return true
	}
	if v.AOI != nil {
		if !(lat >= v.AOI.MinLat && lat <= v.AOI.MaxLat && lon >= v.AOI.MinLon && lon <= v.AOI.MaxLon) {
			return false
		}
	}
	if fl != nil {
		if v.FLMinFt != nil && *fl < *v.FLMinFt {
			return false
		}
		if v.FLMaxFt != nil && *fl > *v.FLMaxFt {
			return false
		}
	}
	return true
}

func sorted2(a, b float64) (float64, float64) {
	if a <= b {
		return a, b
	}
	return b, a
}

var feedPool = []int64{1, 2, 3, 4, 5, 6}

// randomScope builds a random feed allow-set and an optional view filter, with
// ranges that overlap randomTrackParams so the filter actually discriminates.
func randomScope(rng *rand.Rand) *Scope {
	var allowed []int64
	for _, id := range feedPool {
		if rng.Float64() < 0.5 {
			allowed = append(allowed, id)
		}
	}
	if rng.Float64() < 0.3 {
		return NewScope(allowed) // ~30%: no view restriction
	}
	view := &ViewFilter{}
	if rng.Float64() < 0.85 {
		minLat, maxLat := sorted2(45+rng.Float64()*10, 45+rng.Float64()*10)
		minLon, maxLon := sorted2(5+rng.Float64()*10, 5+rng.Float64()*10)
		view.AOI = &BBox{MinLat: minLat, MinLon: minLon, MaxLat: maxLat, MaxLon: maxLon}
	}
	if rng.Float64() < 0.6 {
		v := rng.Float64() * 40000
		view.FLMinFt = &v
	}
	if rng.Float64() < 0.6 {
		v := 10000 + rng.Float64()*40000
		view.FLMaxFt = &v
	}
	return NewScopeWithView(allowed, view)
}

// randomTrackParams generates a position (overlapping the AOI region) and a
// flight level that is absent ~25% of the time (to exercise the fail-open path).
func randomTrackParams(rng *rand.Rand) (lat, lon float64, fl *float64) {
	lat = 44 + rng.Float64()*12 // 44..56
	lon = 4 + rng.Float64()*12  // 4..16
	if rng.Float64() < 0.75 {
		v := rng.Float64() * 45000
		fl = &v
	}
	return
}

// TestFilterViewMatchesOracle asserts, over many random scopes and batches, that
// Scope.filterView keeps exactly the tracks the oracle admits — both directions,
// so neither over- nor under-filtering slips through.
func TestFilterViewMatchesOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(20260620))
	for i := 0; i < 50000; i++ {
		scope := randomScope(rng)
		n := rng.Intn(6)
		tracks := make([]TrackMessage, n)
		for j := range tracks {
			lat, lon, fl := randomTrackParams(rng)
			tracks[j] = TrackMessage{TrackNum: uint16(j + 1), Latitude: lat, Longitude: lon, FlightLevelFt: fl}
		}

		out := scope.filterView(Message{Tracks: tracks})
		kept := make(map[uint16]bool, len(out.Tracks))
		for _, tm := range out.Tracks {
			kept[tm.TrackNum] = true
		}
		for _, tm := range tracks {
			want := viewAdmitsOracle(scope.view, tm.Latitude, tm.Longitude, tm.FlightLevelFt)
			if kept[tm.TrackNum] != want {
				t.Fatalf("iter %d: track %+v kept=%v want=%v view=%+v", i, tm, kept[tm.TrackNum], want, scope.view)
			}
		}
	}
}

// TestBroadcasterIsolationProperty drives the real fan-out (Run + RegisterClient +
// trackChan) with many clients of random scope and many random batches, and
// asserts the isolation invariant end-to-end: every track a client receives lies
// within that client's scope (allowed feed AND inside its view).
func TestBroadcasterIsolationProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	const k = 8
	type client struct {
		ch    chan Message
		scope *Scope
	}
	clients := make([]client, k)
	for i := range clients {
		ch := make(chan Message, 16384) // large enough to avoid eviction during the run
		sc := randomScope(rng)
		b.RegisterClient(ch, sc)
		clients[i] = client{ch, sc}
	}
	for i := 0; i < 500 && b.ClientCount() != k; i++ {
		time.Sleep(time.Millisecond)
	}
	if b.ClientCount() != k {
		t.Fatalf("registered clients = %d, want %d", b.ClientCount(), k)
	}

	const m = 400
	for i := 0; i < m; i++ {
		feedID := feedPool[rng.Intn(len(feedPool))]
		n := 1 + rng.Intn(4)
		dts := make([]cat062.DecodedTrack, n)
		for j := range dts {
			lat, lon, fl := randomTrackParams(rng)
			dts[j] = cat062.DecodedTrack{
				TrackNum:      uint16(i*10 + j),
				WGS84:         cat062.WGS84Position{Latitude: lat, Longitude: lon},
				FlightLevelFt: fl,
			}
		}
		b.trackChan <- TrackBatch{FeedID: feedID, Tracks: dts}
	}
	time.Sleep(50 * time.Millisecond) // let the last fan-outs complete

	for ci, c := range clients {
		drained := false
		for !drained {
			select {
			case msg := <-c.ch:
				for _, tm := range msg.Tracks {
					if !c.scope.AllowsFeed(tm.FeedID) {
						t.Fatalf("ISOLATION BREACH: client %d got a track from non-subscribed feed %d", ci, tm.FeedID)
					}
					if !viewAdmitsOracle(c.scope.view, tm.Latitude, tm.Longitude, tm.FlightLevelFt) {
						t.Fatalf("ISOLATION BREACH: client %d got an out-of-view track %+v (view %+v)", ci, tm, c.scope.view)
					}
				}
			default:
				drained = true
			}
		}
	}
}

// FuzzScopeFilter fuzzes the scope predicate over the realistic finite numeric
// domain: filterView must agree with the oracle (view dimension) and AllowsFeed
// must be exact (feed dimension), and neither may panic.
func FuzzScopeFilter(f *testing.F) {
	f.Add(int64(1), int64(1), 49.0, 51.0, 8.0, 10.0, 50.0, 9.0, true, 20000.0)
	f.Add(int64(2), int64(3), 0.0, 0.0, 0.0, 0.0, 10.0, 20.0, false, 0.0)
	f.Fuzz(func(t *testing.T, allowed, trackFeed int64, b1, b2, b3, b4, lat, lon float64, hasFL bool, flv float64) {
		for _, v := range []float64{b1, b2, b3, b4, lat, lon, flv} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return // focus on the realistic finite domain (real positions/FL are finite)
			}
		}
		minLat, maxLat := sorted2(b1, b2)
		minLon, maxLon := sorted2(b3, b4)
		view := &ViewFilter{AOI: &BBox{MinLat: minLat, MinLon: minLon, MaxLat: maxLat, MaxLon: maxLon}}
		scope := NewScopeWithView([]int64{allowed}, view)
		var fl *float64
		if hasFL {
			fl = &flv
		}
		track := TrackMessage{FeedID: trackFeed, Latitude: lat, Longitude: lon, FlightLevelFt: fl}

		out := scope.filterView(Message{Tracks: []TrackMessage{track}})
		got := len(out.Tracks) == 1
		if want := viewAdmitsOracle(view, lat, lon, fl); got != want {
			t.Fatalf("filterView kept=%v, oracle=%v; track=%+v view=%+v", got, want, track, view)
		}
		if scope.AllowsFeed(trackFeed) != (trackFeed == allowed) {
			t.Fatalf("AllowsFeed(%d)=%v, allowed feed=%d", trackFeed, scope.AllowsFeed(trackFeed), allowed)
		}
	})
}
