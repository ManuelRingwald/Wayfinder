package health

import (
	"testing"
	"time"
)

func TestFeedStartsUnknownNotStale(t *testing.T) {
	h := New(3 * time.Second)
	s := h.Status(time.Unix(100, 0))
	if s.EverSeen {
		t.Errorf("EverSeen: got true before any heartbeat")
	}
	if s.Stale {
		t.Errorf("Stale: got true before any heartbeat (cannot be stale yet)")
	}
}

func TestFeedFreshAfterHeartbeat(t *testing.T) {
	h := New(3 * time.Second)
	t0 := time.Unix(100, 0)
	h.RecordHeartbeat(t0)

	// Within the timeout: fresh.
	s := h.Status(t0.Add(2 * time.Second))
	if !s.EverSeen || s.Stale {
		t.Errorf("within timeout: got %+v, want EverSeen and not Stale", s)
	}
}

func TestFeedGoesStaleAfterTimeout(t *testing.T) {
	h := New(3 * time.Second)
	t0 := time.Unix(100, 0)
	h.RecordHeartbeat(t0)

	s := h.Status(t0.Add(4 * time.Second))
	if !s.Stale {
		t.Errorf("after timeout: got not stale, want stale")
	}

	// A new heartbeat clears staleness.
	h.RecordHeartbeat(t0.Add(5 * time.Second))
	if h.Status(t0.Add(6 * time.Second)).Stale {
		t.Errorf("after fresh heartbeat: got stale, want fresh")
	}
}

func TestObserveReportsOnlyTransitions(t *testing.T) {
	h := New(3 * time.Second)
	t0 := time.Unix(100, 0)

	// First observation always counts as a change.
	if _, changed := h.Observe(t0); !changed {
		t.Errorf("first observe: want changed=true")
	}
	// No change on a second identical observation.
	if _, changed := h.Observe(t0); changed {
		t.Errorf("second identical observe: want changed=false")
	}

	// Heartbeat → becomes EverSeen+fresh: a transition.
	h.RecordHeartbeat(t0.Add(1 * time.Second))
	if s, changed := h.Observe(t0.Add(1 * time.Second)); !changed || s.Stale || !s.EverSeen {
		t.Errorf("after first heartbeat: got %+v changed=%v, want EverSeen, fresh, changed", s, changed)
	}

	// Goes stale: a transition.
	if s, changed := h.Observe(t0.Add(10 * time.Second)); !changed || !s.Stale {
		t.Errorf("after timeout: got %+v changed=%v, want stale, changed", s, changed)
	}
	// Still stale: no further transition.
	if _, changed := h.Observe(t0.Add(11 * time.Second)); changed {
		t.Errorf("still stale: want changed=false")
	}
}
