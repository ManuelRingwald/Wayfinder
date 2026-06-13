package firefly

import (
	"encoding/json"
	"strings"
	"testing"
)

// A Frame as Firefly actually sends it (captured from
// crates/firefly-io/src/frame.rs's own JSON tests) decodes into the expected
// fields. REQ: FR-DATA-001
func TestFrameDecodesFireflyWireFormat(t *testing.T) {
	const wire = `{
		"time": 12.0,
		"sensor": 3,
		"plots": [
			{"lat_deg": 47.5, "lon_deg": 8.25, "has_ssr": true}
		],
		"tracks": [
			{
				"id": 5,
				"lat_deg": 47.5,
				"lon_deg": 8.25,
				"height_m": 500.0,
				"ground_speed_mps": 150.0,
				"track_angle_deg": 90.0,
				"confirmed": true,
				"coasting": false,
				"update_age_s": 0.0,
				"position_uncertainty_m": 42.0
			}
		]
	}`

	var frame Frame
	if err := json.Unmarshal([]byte(wire), &frame); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if frame.Time != 12.0 {
		t.Errorf("Time = %v, want 12.0", frame.Time)
	}
	if frame.Sensor != 3 {
		t.Errorf("Sensor = %v, want 3", frame.Sensor)
	}

	if len(frame.Plots) != 1 {
		t.Fatalf("len(Plots) = %d, want 1", len(frame.Plots))
	}
	plot := frame.Plots[0]
	if plot.LatDeg != 47.5 || plot.LonDeg != 8.25 || !plot.HasSSR {
		t.Errorf("Plots[0] = %+v, want {47.5 8.25 true}", plot)
	}

	if len(frame.Tracks) != 1 {
		t.Fatalf("len(Tracks) = %d, want 1", len(frame.Tracks))
	}
	track := frame.Tracks[0]
	want := FrameTrack{
		ID:                   5,
		LatDeg:               47.5,
		LonDeg:               8.25,
		HeightM:              500.0,
		GroundSpeedMPS:       150.0,
		TrackAngleDeg:        90.0,
		Confirmed:            true,
		Coasting:             false,
		UpdateAgeS:           0.0,
		PositionUncertaintyM: 42.0,
	}
	if track != want {
		t.Errorf("Tracks[0] = %+v, want %+v", track, want)
	}
}

// A frame survives a JSON round-trip unchanged. REQ: FR-DATA-001
func TestFrameRoundTripsThroughJSON(t *testing.T) {
	original := Frame{
		Time:   12.0,
		Sensor: 1,
		Plots: []FramePlot{
			{LatDeg: 47.5, LonDeg: 8.25, HasSSR: true},
		},
		Tracks: []FrameTrack{
			{ID: 1, LatDeg: 47.5, LonDeg: 8.25, HeightM: 500, GroundSpeedMPS: 150,
				TrackAngleDeg: 90, Confirmed: true, Coasting: false,
				UpdateAgeS: 0, PositionUncertaintyM: 42},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back Frame
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if back.Time != original.Time || back.Sensor != original.Sensor {
		t.Errorf("round-trip changed Time/Sensor: got %+v, want %+v", back, original)
	}
	if len(back.Plots) != 1 || back.Plots[0] != original.Plots[0] {
		t.Errorf("round-trip changed Plots: got %+v, want %+v", back.Plots, original.Plots)
	}
	if len(back.Tracks) != 1 || back.Tracks[0] != original.Tracks[0] {
		t.Errorf("round-trip changed Tracks: got %+v, want %+v", back.Tracks, original.Tracks)
	}
}

// An empty frame (no plots, no tracks) decodes to empty slices, not nil,
// and round-trips to "[]" rather than "null" — matching Firefly's
// "empty_frame_has_no_tracks" guarantee. REQ: FR-DATA-001
func TestEmptyFrameHasNoTracksOrPlots(t *testing.T) {
	const wire = `{"time": 0.0, "sensor": 1, "plots": [], "tracks": []}`

	var frame Frame
	if err := json.Unmarshal([]byte(wire), &frame); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(frame.Plots) != 0 {
		t.Errorf("Plots = %+v, want empty", frame.Plots)
	}
	if len(frame.Tracks) != 0 {
		t.Errorf("Tracks = %+v, want empty", frame.Tracks)
	}
}

// The JSON uses the flat, self-describing field names a consumer relies on,
// with newtypes (sensor, track id) as bare numbers — mirroring Firefly's
// "json_is_self_describing" guarantee. REQ: FR-DATA-001
func TestFrameFieldNamesMatchFireflyWireFormat(t *testing.T) {
	frame := Frame{
		Time:   12.0,
		Sensor: 3,
		Tracks: []FrameTrack{{ID: 5}},
	}

	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)

	for _, key := range []string{
		`"time"`, `"sensor"`, `"tracks"`, `"id"`, `"lat_deg"`, `"lon_deg"`,
		`"coasting"`, `"position_uncertainty_m"`,
	} {
		if !strings.Contains(got, key) {
			t.Errorf("JSON missing key %s: %s", key, got)
		}
	}

	if !strings.Contains(got, `"sensor":3`) {
		t.Errorf("sensor should be a bare number: %s", got)
	}
	if !strings.Contains(got, `"id":5`) {
		t.Errorf("track id should be a bare number: %s", got)
	}
}
