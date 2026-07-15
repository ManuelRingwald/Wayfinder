package health

import (
	"testing"
	"time"
)

var t0 = time.Unix(1000, 0)

func TestRegistrySnapshotUnknownFeed(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	s := r.Snapshot(42, t0)
	if s.EverSeen || s.Stale {
		t.Errorf("unknown feed: got %+v, want zero value", s)
	}
	if got := s.Color(); got != "red" {
		t.Errorf("unknown feed color: got %q, want %q", got, "red")
	}
}

// TestRegistryRecordSensorsStoresDetail verifies the per-sensor breakdown (#237)
// — identity, state and applied bias — survives RecordSensors → Snapshot, and
// that absence of a bias stays nil (never 0).
func TestRegistryRecordSensorsStoresDetail(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	rb, az := 144.6875, 0.302
	details := []SensorDetail{
		{SAC: 0, SIC: 1, Operational: true, RangeBiasM: &rb, AzimuthBiasDeg: &az},
		{SAC: 0, SIC: 2, Operational: false, Reason: "unreachable"},
	}
	r.RecordSensors(7, 1, 2, "unreachable", details)

	s := r.Snapshot(7, t0)
	if s.SensorsActive != 1 || s.SensorsTotal != 2 {
		t.Errorf("counts: got active=%d total=%d, want 1/2", s.SensorsActive, s.SensorsTotal)
	}
	if len(s.Sensors) != 2 {
		t.Fatalf("expected 2 sensors, got %d", len(s.Sensors))
	}
	if s.Sensors[0].SIC != 1 || s.Sensors[0].RangeBiasM == nil || *s.Sensors[0].RangeBiasM != rb {
		t.Errorf("sensor 0: got %+v, want SIC 1 with range bias %v", s.Sensors[0], rb)
	}
	if s.Sensors[1].SIC != 2 || s.Sensors[1].Operational || s.Sensors[1].Reason != "unreachable" {
		t.Errorf("sensor 1: got %+v, want SIC 2 degraded/unreachable", s.Sensors[1])
	}
	if s.Sensors[1].RangeBiasM != nil {
		t.Errorf("sensor 1: expected nil bias (no correction), got %v", *s.Sensors[1].RangeBiasM)
	}
}

func TestRegistryHeartbeatMakesFeedGreen(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)
	r.RecordTracks(1, 5)

	s := r.Snapshot(1, t0.Add(1*time.Second))
	if !s.EverSeen || s.Stale {
		t.Errorf("after heartbeat+tracks: got %+v, want EverSeen, not Stale", s)
	}
	if s.TrackCountRecent != 5 {
		t.Errorf("TrackCountRecent: got %d, want 5", s.TrackCountRecent)
	}
	if got := s.Color(); got != "green" {
		t.Errorf("color: got %q, want %q", got, "green")
	}
}

func TestRegistryHeartbeatNoTracksIsGreen(t *testing.T) {
	// An empty sky (heartbeat healthy, no tracks) is green, not yellow.
	// Yellow is reserved for degraded sensor fusion (CAT063, Firefly issue #32).
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)
	// no RecordTracks call → block=0

	s := r.Snapshot(1, t0.Add(1*time.Second))
	if got := s.Color(); got != "green" {
		t.Errorf("color: got %q, want %q", got, "green")
	}
}

func TestRegistryDegradedSensorsIsYellow(t *testing.T) {
	// Yellow = heartbeat healthy but at least one sensor silent.
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)

	s := r.Snapshot(1, t0.Add(1*time.Second))
	s.SensorsTotal = 3
	s.SensorsActive = 2 // one silent
	if got := s.Color(); got != "yellow" {
		t.Errorf("color (2/3 sensors): got %q, want %q", got, "yellow")
	}
}

func TestRegistryAllSensorsActiveIsGreen(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)

	s := r.Snapshot(1, t0.Add(1*time.Second))
	s.SensorsTotal = 3
	s.SensorsActive = 3
	if got := s.Color(); got != "green" {
		t.Errorf("color (3/3 sensors): got %q, want %q", got, "green")
	}
}

func TestRegistryUnknownSensorCountIsGreen(t *testing.T) {
	// SensorsTotal=0 means unknown (no CAT063 yet) — must not trigger yellow.
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)

	s := r.Snapshot(1, t0.Add(1*time.Second))
	// SensorsTotal and SensorsActive default to zero
	if got := s.Color(); got != "green" {
		t.Errorf("color (no CAT063 data): got %q, want %q", got, "green")
	}
}

func TestRegistryStaleIsRed(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)
	r.RecordTracks(1, 2)

	s := r.Snapshot(1, t0.Add(4*time.Second))
	if !s.Stale {
		t.Errorf("after timeout: want Stale")
	}
	if got := s.Color(); got != "red" {
		t.Errorf("color: got %q, want %q", got, "red")
	}
}

func TestRegistryLastHeartbeatAgoS(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)

	s := r.Snapshot(1, t0.Add(2*time.Second))
	if s.LastHeartbeatAgoS < 1.9 || s.LastHeartbeatAgoS > 2.1 {
		t.Errorf("LastHeartbeatAgoS: got %.2f, want ~2.0", s.LastHeartbeatAgoS)
	}
}

func TestRegistryLastHeartbeatAgoNegativeIfNeverSeen(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	// Touch the entry via RecordTracks but never heartbeat.
	r.RecordTracks(1, 0)

	s := r.Snapshot(1, t0)
	if s.LastHeartbeatAgoS >= 0 {
		t.Errorf("LastHeartbeatAgoS: got %.2f, want negative (never seen)", s.LastHeartbeatAgoS)
	}
}

func TestRegistryPerFeedIsolation(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)
	r.RecordTracks(1, 4)
	// Feed 2 never receives a heartbeat.

	s1 := r.Snapshot(1, t0.Add(1*time.Second))
	s2 := r.Snapshot(2, t0.Add(1*time.Second))

	if !s1.EverSeen {
		t.Errorf("feed 1: want EverSeen")
	}
	if s2.EverSeen {
		t.Errorf("feed 2: want not EverSeen (isolated from feed 1)")
	}
}

func TestRegistryAggregateStatusReflectsAnyFeed(t *testing.T) {
	r := NewRegistry(3 * time.Second)
	r.RecordHeartbeat(1, t0)

	st := r.Status(t0.Add(1 * time.Second))
	if !st.EverSeen || st.Stale {
		t.Errorf("aggregate status: got %+v, want EverSeen, not Stale", st)
	}
}

func TestRegistryAggregateObserveReportsTransition(t *testing.T) {
	r := NewRegistry(3 * time.Second)

	// First observation always transitions.
	if _, changed := r.Observe(t0); !changed {
		t.Errorf("first observe: want changed=true")
	}
	// No change.
	if _, changed := r.Observe(t0); changed {
		t.Errorf("second identical observe: want changed=false")
	}
	// Heartbeat → EverSeen transition.
	r.RecordHeartbeat(1, t0.Add(1*time.Second))
	if _, changed := r.Observe(t0.Add(1 * time.Second)); !changed {
		t.Errorf("after heartbeat: want changed=true")
	}
}
