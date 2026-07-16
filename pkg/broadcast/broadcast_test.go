package broadcast

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// TestBroadcasterBasic tests basic track broadcasting.
func TestBroadcasterBasic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	// Register a client scoped to the (feed 0) batch below.
	sendChan := make(chan Message, 10)
	client := b.RegisterClient(sendChan, NewScope([]int64{0}))

	// Run broadcaster in background.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() { _ = b.Run(ctx) }()

	// Give broadcaster time to start.
	time.Sleep(10 * time.Millisecond)

	// Send a track.
	track := cat062.DecodedTrack{
		TrackNum: 42,
		Source:   cat062.DataSourceID{SAC: 0x19, SIC: 0x02},
		WGS84:    cat062.WGS84Position{Latitude: 45.0, Longitude: 11.25},
		Velocity: cat062.Velocity{Vx: 100.0, Vy: -50.0},
	}

	b.trackChan <- TrackBatch{Tracks: []cat062.DecodedTrack{track}}

	// Receive broadcast.
	select {
	case msg := <-sendChan:
		if len(msg.Tracks) != 1 {
			t.Errorf("expected 1 track, got %d", len(msg.Tracks))
		}
		if msg.Tracks[0].TrackNum != 42 {
			t.Errorf("track_num: expected 42, got %d", msg.Tracks[0].TrackNum)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast")
	}

	// Unregister client.
	b.UnregisterClient(client)

	// Give broadcaster time to unregister.
	time.Sleep(10 * time.Millisecond)

	if b.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", b.ClientCount())
	}
}

// TestBroadcasterMultipleClients tests broadcasting to multiple clients.
func TestBroadcasterMultipleClients(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	// Register 3 clients.
	clients := make([]*Client, 3)
	sendChans := make([]chan Message, 3)
	for i := 0; i < 3; i++ {
		sendChans[i] = make(chan Message, 10)
		clients[i] = b.RegisterClient(sendChans[i], NewScope([]int64{0}))
	}

	// Run broadcaster.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go func() { _ = b.Run(ctx) }()

	time.Sleep(10 * time.Millisecond)

	if b.ClientCount() != 3 {
		t.Fatalf("expected 3 clients, got %d", b.ClientCount())
	}

	// Send a track.
	b.trackChan <- TrackBatch{Tracks: []cat062.DecodedTrack{{TrackNum: 1}}}

	// All clients should receive it.
	for i := 0; i < 3; i++ {
		select {
		case msg := <-sendChans[i]:
			if len(msg.Tracks) != 1 {
				t.Errorf("client %d: expected 1 track, got %d", i, len(msg.Tracks))
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("client %d: timeout waiting for broadcast", i)
		}
	}
}

// TestBroadcasterSend tests the Send() method.
func TestBroadcasterSend(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	sendChan := make(chan Message, 10)
	b.RegisterClient(sendChan, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go func() { _ = b.Run(ctx) }()

	time.Sleep(10 * time.Millisecond)

	// Send a message directly.
	msg := Message{Tracks: []TrackMessage{{TrackNum: 99}}}
	if err := b.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Receive it.
	select {
	case received := <-sendChan:
		if len(received.Tracks) != 1 || received.Tracks[0].TrackNum != 99 {
			t.Errorf("message mismatch")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout")
	}
}

// TestTracksToMessage tests track conversion to message format.
func TestTracksToMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	track := cat062.DecodedTrack{
		TrackNum:  42,
		Source:    cat062.DataSourceID{SAC: 0x19, SIC: 0x02},
		WGS84:     cat062.WGS84Position{Latitude: 45.0, Longitude: 11.25},
		Velocity:  cat062.Velocity{Vx: 100.0, Vy: -50.0},
		Cartesian: cat062.CartesianPosition{X: 1000.0, Y: -500.0},
	}

	msg := b.tracksToMessage(TrackBatch{FeedID: 7, Tracks: []cat062.DecodedTrack{track}})

	if len(msg.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(msg.Tracks))
	}

	tm := msg.Tracks[0]
	if tm.FeedID != 7 {
		t.Errorf("FeedID: got %d, want 7 (batch feed stamped onto track)", tm.FeedID)
	}
	if tm.TrackNum != 42 {
		t.Errorf("TrackNum: got %d, want 42", tm.TrackNum)
	}
	if tm.SAC != 0x19 {
		t.Errorf("SAC: got 0x%02X, want 0x19", tm.SAC)
	}
	if tm.SIC != 0x02 {
		t.Errorf("SIC: got 0x%02X, want 0x02", tm.SIC)
	}
	if tm.Latitude < 44.999 || tm.Latitude > 45.001 {
		t.Errorf("Latitude: got %f, want ~45.0", tm.Latitude)
	}
	if tm.CartX < 999.9 || tm.CartX > 1000.1 {
		t.Errorf("CartX: got %f, want ~1000.0", tm.CartX)
	}
}

// TestTracksToMessageMapsVerticalChain verifies the vertical chain (I062/130/
// 135/220, ICD 3.5.0, #241) is carried to the wire, and that the QNH-correction
// flag is emitted only alongside a barometric altitude — a track without a
// barometric solution must ship no "qnh_corrected" pointer (never a stray false).
func TestTracksToMessageMapsVerticalChain(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	geo := 10000.0
	baro := 3000.0
	rocd := -1200.0
	withVertical := cat062.DecodedTrack{
		TrackNum:                1,
		GeometricAltitudeFt:     &geo,
		BarometricAltitudeFt:    &baro,
		BaroQNHCorrected:        true,
		RateOfClimbDescentFtMin: &rocd,
	}
	withoutVertical := cat062.DecodedTrack{TrackNum: 2}

	msg := b.tracksToMessage(TrackBatch{FeedID: 7, Tracks: []cat062.DecodedTrack{withVertical, withoutVertical}})
	if len(msg.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(msg.Tracks))
	}

	tm := msg.Tracks[0]
	if tm.GeometricAltitudeFt == nil || *tm.GeometricAltitudeFt != 10000.0 {
		t.Errorf("GeometricAltitudeFt: got %v, want 10000", tm.GeometricAltitudeFt)
	}
	if tm.BarometricAltitudeFt == nil || *tm.BarometricAltitudeFt != 3000.0 {
		t.Errorf("BarometricAltitudeFt: got %v, want 3000", tm.BarometricAltitudeFt)
	}
	if tm.QNHCorrected == nil || *tm.QNHCorrected != true {
		t.Errorf("QNHCorrected: got %v, want true", tm.QNHCorrected)
	}
	if tm.RocdFtMin == nil || *tm.RocdFtMin != -1200.0 {
		t.Errorf("RocdFtMin: got %v, want -1200", tm.RocdFtMin)
	}

	// No barometric altitude ⇒ no QNH pointer at all.
	bare := msg.Tracks[1]
	if bare.GeometricAltitudeFt != nil || bare.BarometricAltitudeFt != nil || bare.RocdFtMin != nil {
		t.Errorf("bare track carried vertical items: %+v", bare)
	}
	if bare.QNHCorrected != nil {
		t.Errorf("QNHCorrected: got %v, want nil for a track without barometric altitude", *bare.QNHCorrected)
	}
}

// TestTracksToMessageMapsKinematics verifies the kinematics chain (I062/200/210,
// ICD 3.6.0, #242) is carried to the wire: the determined motion axes become their
// canonical strings, an undetermined axis ships no field, and the acceleration
// components pass through. A track without kinematics carries none of the fields.
func TestTracksToMessageMapsKinematics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	course := cat062.CourseRight
	vert := cat062.VerticalClimb // SpeedTrend deliberately left nil (undetermined)
	ax := 1.0
	ay := -0.5
	withKin := cat062.DecodedTrack{
		TrackNum:       1,
		MotionCourse:   &course,
		MotionVertical: &vert,
		AccelAxMS2:     &ax,
		AccelAyMS2:     &ay,
	}
	withoutKin := cat062.DecodedTrack{TrackNum: 2}

	msg := b.tracksToMessage(TrackBatch{FeedID: 7, Tracks: []cat062.DecodedTrack{withKin, withoutKin}})
	if len(msg.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(msg.Tracks))
	}

	tm := msg.Tracks[0]
	if tm.CourseTrend == nil || *tm.CourseTrend != "right" {
		t.Errorf("CourseTrend: got %v, want right", tm.CourseTrend)
	}
	if tm.SpeedTrend != nil {
		t.Errorf("SpeedTrend: got %v, want nil (undetermined axis ships no field)", *tm.SpeedTrend)
	}
	if tm.VerticalMotion == nil || *tm.VerticalMotion != "climb" {
		t.Errorf("VerticalMotion: got %v, want climb", tm.VerticalMotion)
	}
	if tm.AccelAxMs2 == nil || *tm.AccelAxMs2 != 1.0 {
		t.Errorf("AccelAxMs2: got %v, want 1.0", tm.AccelAxMs2)
	}
	if tm.AccelAyMs2 == nil || *tm.AccelAyMs2 != -0.5 {
		t.Errorf("AccelAyMs2: got %v, want -0.5", tm.AccelAyMs2)
	}

	bare := msg.Tracks[1]
	if bare.CourseTrend != nil || bare.SpeedTrend != nil || bare.VerticalMotion != nil ||
		bare.AccelAxMs2 != nil || bare.AccelAyMs2 != nil {
		t.Errorf("bare track carried kinematics fields: %+v", bare)
	}
}

// TestTracksToMessageMapsFlightPlan verifies the flight-plan correlation (I062/390,
// ICD 3.7.0, #245) is carried to the wire, that a callsign-only plan ships no
// route, and that an uncorrelated track carries none of the fields.
func TestTracksToMessageMapsFlightPlan(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	csn := "DLH123"
	dep := "EDDF"
	dst := "EDDM"
	csnOnly := "BAW22"
	full := cat062.DecodedTrack{TrackNum: 1, PlanCallsign: &csn, PlanDeparture: &dep, PlanDestination: &dst}
	partial := cat062.DecodedTrack{TrackNum: 2, PlanCallsign: &csnOnly}
	none := cat062.DecodedTrack{TrackNum: 3}

	msg := b.tracksToMessage(TrackBatch{FeedID: 7, Tracks: []cat062.DecodedTrack{full, partial, none}})
	if len(msg.Tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(msg.Tracks))
	}

	tm := msg.Tracks[0]
	if tm.PlanCallsign == nil || *tm.PlanCallsign != "DLH123" {
		t.Errorf("PlanCallsign: got %v, want DLH123", tm.PlanCallsign)
	}
	if tm.PlanDeparture == nil || *tm.PlanDeparture != "EDDF" {
		t.Errorf("PlanDeparture: got %v, want EDDF", tm.PlanDeparture)
	}
	if tm.PlanDestination == nil || *tm.PlanDestination != "EDDM" {
		t.Errorf("PlanDestination: got %v, want EDDM", tm.PlanDestination)
	}

	partialTM := msg.Tracks[1]
	if partialTM.PlanCallsign == nil || *partialTM.PlanCallsign != "BAW22" {
		t.Errorf("partial PlanCallsign: got %v, want BAW22", partialTM.PlanCallsign)
	}
	if partialTM.PlanDeparture != nil || partialTM.PlanDestination != nil {
		t.Errorf("callsign-only plan carried a route: dep=%v dst=%v", partialTM.PlanDeparture, partialTM.PlanDestination)
	}

	bare := msg.Tracks[2]
	if bare.PlanCallsign != nil || bare.PlanDeparture != nil || bare.PlanDestination != nil {
		t.Errorf("uncorrelated track carried plan fields: %+v", bare)
	}
}

// TestTracksToMessageMapsAdsbAge verifies the I062/290 ES age (ADS-B, ICD
// 2.4.0) is carried through to the wire as adsb_age_s, and that a radar-only
// track leaves it nil (so the frontend shows no ADS-B badge). AP9.9.
func TestTracksToMessageMapsAdsbAge(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	b := New(logger)

	esAge := 3.0
	adsbTrack := cat062.DecodedTrack{TrackNum: 1, UpdateAge: cat062.UpdateAge{PSRAge: 2.0, ESAge: &esAge}}
	radarTrack := cat062.DecodedTrack{TrackNum: 2, UpdateAge: cat062.UpdateAge{PSRAge: 2.0}}

	msg := b.tracksToMessage(TrackBatch{Tracks: []cat062.DecodedTrack{adsbTrack, radarTrack}})
	if len(msg.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(msg.Tracks))
	}

	if msg.Tracks[0].AdsbAgeS == nil {
		t.Fatalf("ADS-B track: AdsbAgeS got nil, want ~3.0")
	}
	if *msg.Tracks[0].AdsbAgeS < 2.99 || *msg.Tracks[0].AdsbAgeS > 3.01 {
		t.Errorf("ADS-B track: AdsbAgeS got %f, want ~3.0", *msg.Tracks[0].AdsbAgeS)
	}
	if msg.Tracks[1].AdsbAgeS != nil {
		t.Errorf("radar-only track: AdsbAgeS got %v, want nil", *msg.Tracks[1].AdsbAgeS)
	}
}

// TestScopeAllowsFeed covers the feed-level scope predicate: a nil scope sees
// nothing (fail-closed, ADR 0014 — production always scopes), a built scope sees
// only its feeds, and an empty scope sees nothing (fail-closed).
func TestScopeAllowsFeed(t *testing.T) {
	var unscoped *Scope // nil
	if unscoped.AllowsFeed(1) || unscoped.AllowsFeed(999) {
		t.Error("nil scope must be fail-closed (allow no feed)")
	}

	s := NewScope([]int64{1, 3})
	if !s.AllowsFeed(1) || !s.AllowsFeed(3) {
		t.Error("scope must allow its own feeds")
	}
	if s.AllowsFeed(2) {
		t.Error("scope must reject a feed it was not granted")
	}

	if NewScope(nil).AllowsFeed(1) {
		t.Error("empty scope must allow nothing (fail-closed)")
	}
}

// TestBroadcastFeedIsolation is the mandatory cross-tenant negative test (WF2-21,
// NFR-SEC-003): a client scoped to feed 1 must NEVER receive a track from feed 2,
// and vice versa. Two clients with disjoint scopes prove the boundary.
func TestBroadcastFeedIsolation(t *testing.T) {
	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() { _ = b.Run(ctx) }()

	chanA := make(chan Message, 10)
	chanB := make(chan Message, 10)
	b.RegisterClient(chanA, NewScope([]int64{1})) // tenant A: only feed 1
	b.RegisterClient(chanB, NewScope([]int64{2})) // tenant B: only feed 2
	for i := 0; i < 100 && b.ClientCount() != 2; i++ {
		time.Sleep(time.Millisecond)
	}

	// A track on feed 1 → only A.
	b.trackChan <- TrackBatch{FeedID: 1, Tracks: []cat062.DecodedTrack{{TrackNum: 11}}}
	select {
	case msg := <-chanA:
		if len(msg.Tracks) != 1 || msg.Tracks[0].FeedID != 1 {
			t.Fatalf("A: got %+v, want one feed-1 track", msg.Tracks)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("A: timeout — should have received its own feed's track")
	}
	select {
	case msg := <-chanB:
		t.Fatalf("ISOLATION BREACH: B received a feed-1 track: %+v", msg.Tracks)
	case <-time.After(100 * time.Millisecond):
		// expected: B sees nothing from feed 1
	}

	// A track on feed 2 → only B.
	b.trackChan <- TrackBatch{FeedID: 2, Tracks: []cat062.DecodedTrack{{TrackNum: 22}}}
	select {
	case msg := <-chanB:
		if len(msg.Tracks) != 1 || msg.Tracks[0].FeedID != 2 {
			t.Fatalf("B: got %+v, want one feed-2 track", msg.Tracks)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("B: timeout — should have received its own feed's track")
	}
	select {
	case msg := <-chanA:
		t.Fatalf("ISOLATION BREACH: A received a feed-2 track: %+v", msg.Tracks)
	case <-time.After(100 * time.Millisecond):
		// expected: A sees nothing from feed 2
	}
}

// fl returns a pointer to a flight level expressed in feet.
func flFt(ft float64) *float64 { return &ft }

// TestViewFilterAdmits covers the AOI + FL-band predicate, including the
// fail-open rule: a track without a measured flight level is always admitted.
func TestViewFilterAdmits(t *testing.T) {
	// AOI roughly around Frankfurt; FL band 100..300 (10000..30000 ft).
	view := &ViewFilter{
		AOI:     &BBox{MinLat: 49, MinLon: 8, MaxLat: 51, MaxLon: 10},
		FLMinFt: flFt(10000),
		FLMaxFt: flFt(30000),
	}
	inside := func(lat, lon float64, flightFt *float64) TrackMessage {
		return TrackMessage{Latitude: lat, Longitude: lon, FlightLevelFt: flightFt}
	}

	cases := map[string]struct {
		t    TrackMessage
		want bool
	}{
		"inside AOI + FL band":      {inside(50, 9, flFt(20000)), true},
		"outside AOI (north)":       {inside(52, 9, flFt(20000)), false},
		"outside AOI (east)":        {inside(50, 11, flFt(20000)), false},
		"below FL band":             {inside(50, 9, flFt(5000)), false},
		"above FL band":             {inside(50, 9, flFt(40000)), false},
		"in AOI, no FL (fail-open)": {inside(50, 9, nil), true},
		"on AOI edge (inclusive)":   {inside(49, 8, flFt(10000)), true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := view.admits(tc.t); got != tc.want {
				t.Errorf("admits = %v, want %v", got, tc.want)
			}
		})
	}

	// A nil view admits everything.
	var none *ViewFilter
	if !none.admits(inside(0, 0, nil)) {
		t.Error("nil view must admit every track")
	}
}

// TestBroadcastViewScoping verifies that within an allowed feed, a client only
// receives the tracks inside its AOI/FL view (WF2-21.2), while an unscoped client
// receives all of them.
func TestBroadcastViewScoping(t *testing.T) {
	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() { _ = b.Run(ctx) }()

	viewChan := make(chan Message, 10)
	allChan := make(chan Message, 10)
	view := &ViewFilter{AOI: &BBox{MinLat: 49, MinLon: 8, MaxLat: 51, MaxLon: 10}}
	b.RegisterClient(viewChan, NewScopeWithView([]int64{1}, view)) // AOI-scoped
	b.RegisterClient(allChan, NewScope([]int64{1}))                // same feed, no view
	for i := 0; i < 100 && b.ClientCount() != 2; i++ {
		time.Sleep(time.Millisecond)
	}

	// Two tracks on feed 1: one inside the AOI, one well outside.
	b.trackChan <- TrackBatch{FeedID: 1, Tracks: []cat062.DecodedTrack{
		{TrackNum: 1, WGS84: cat062.WGS84Position{Latitude: 50, Longitude: 9}},
		{TrackNum: 2, WGS84: cat062.WGS84Position{Latitude: 10, Longitude: 80}},
	}}

	select {
	case msg := <-viewChan:
		if len(msg.Tracks) != 1 || msg.Tracks[0].TrackNum != 1 {
			t.Fatalf("AOI client: got %+v, want only the in-AOI track 1", msg.Tracks)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("AOI client: timeout")
	}
	select {
	case msg := <-allChan:
		if len(msg.Tracks) != 2 {
			t.Fatalf("unscoped client: got %d tracks, want 2 (no view filter)", len(msg.Tracks))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("unscoped client: timeout")
	}
}

// TestBroadcasterTenantMetrics verifies the per-tenant counters (WF2-23.2):
// connected clients and delivered tracks are tallied per tenant, and unregister
// decrements the connected gauge.
func TestBroadcasterTenantMetrics(t *testing.T) {
	b := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = b.Run(ctx) }()

	s1 := NewScope([]int64{1})
	s1.TenantID = 1
	s2 := NewScope([]int64{2})
	s2.TenantID = 2
	ch1 := make(chan Message, 10)
	ch2 := make(chan Message, 10)
	c1 := b.RegisterClient(ch1, s1)
	b.RegisterClient(ch2, s2)
	for i := 0; i < 200 && b.ClientCount() != 2; i++ {
		time.Sleep(time.Millisecond)
	}

	// Tenant 1's feed → 2 tracks to tenant 1 only; tenant 2's feed → 3 to tenant 2.
	b.trackChan <- TrackBatch{FeedID: 1, Tracks: make([]cat062.DecodedTrack, 2)}
	b.trackChan <- TrackBatch{FeedID: 2, Tracks: make([]cat062.DecodedTrack, 3)}
	<-ch1
	<-ch2

	byTenant := func() map[int64]TenantMetric {
		m := map[int64]TenantMetric{}
		for _, tm := range b.TenantMetrics() {
			m[tm.TenantID] = tm
		}
		return m
	}
	waitFor := func(cond func(map[int64]TenantMetric) bool) map[int64]TenantMetric {
		for i := 0; i < 200; i++ {
			if m := byTenant(); cond(m) {
				return m
			}
			time.Sleep(time.Millisecond)
		}
		return byTenant()
	}

	m := waitFor(func(m map[int64]TenantMetric) bool { return m[1].Delivered == 2 && m[2].Delivered == 3 })
	if m[1].Connected != 1 || m[1].Delivered != 2 {
		t.Errorf("tenant 1 = %+v, want connected 1 delivered 2", m[1])
	}
	if m[2].Connected != 1 || m[2].Delivered != 3 {
		t.Errorf("tenant 2 = %+v, want connected 1 delivered 3", m[2])
	}

	// Unregister tenant 1's client → its connected gauge drops to 0 (delivered stays).
	b.UnregisterClient(c1)
	m = waitFor(func(m map[int64]TenantMetric) bool { return m[1].Connected == 0 })
	if m[1].Connected != 0 || m[1].Delivered != 2 {
		t.Errorf("tenant 1 after unregister = %+v, want connected 0 delivered 2", m[1])
	}
}

// TestBroadcastEvictsClientWithFullSendChannel verifies that a client whose
// send channel is full (i.e., not being drained) is evicted instead of
// blocking the broadcaster.
func TestBroadcastEvictsClientWithFullSendChannel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := New(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go func() { _ = b.Run(ctx) }()

	// Unbuffered channel that nobody reads from, so the first broadcast fills it.
	sendChan := make(chan Message)
	b.RegisterClient(sendChan, nil)

	// Wait for registration to be processed.
	for i := 0; i < 100 && b.ClientCount() != 1; i++ {
		time.Sleep(time.Millisecond)
	}
	if b.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", b.ClientCount())
	}

	if err := b.Send(Message{Tracks: []TrackMessage{{TrackNum: 1}}}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Wait for the broadcaster to evict the unresponsive client.
	for i := 0; i < 100 && b.ClientCount() != 0; i++ {
		time.Sleep(time.Millisecond)
	}
	if b.ClientCount() != 0 {
		t.Errorf("expected client to be evicted, got %d clients", b.ClientCount())
	}
	if got := b.EvictedCount(); got != 1 {
		t.Errorf("EvictedCount: got %d, want 1", got)
	}
}
