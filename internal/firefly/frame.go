// Package firefly contains Wayfinder's representation of the data Firefly
// sends over its WebSocket feed (the "Frame" wire format).
//
// These types mirror crates/firefly-io/src/frame.rs in the Firefly project:
// the tracker's internal radians/velocity-components are already converted
// to map-friendly degrees and derived kinematics on the Firefly side, so
// Wayfinder only needs to decode and render them.
package firefly

// FramePlot is one raw radar detection, included alongside tracks so the ASD
// can show the tracker's input (raw plots) next to its output (tracks).
type FramePlot struct {
	// LatDeg is the latitude in decimal degrees (positive north).
	LatDeg float64 `json:"lat_deg"`
	// LonDeg is the longitude in decimal degrees (positive east).
	LonDeg float64 `json:"lon_deg"`
	// HasSSR reports whether an SSR reply (Mode 3/A, Mode S) was present.
	HasSSR bool `json:"has_ssr"`
}

// FrameTrack is one track in Firefly's web-friendly wire form.
//
// Confirmed, Coasting, UpdateAgeS and PositionUncertaintyM are
// safety-relevant status fields carried through verbatim from the tracker —
// Wayfinder only renders them, it does not reinterpret them (mirrors
// Firefly's ADR 0008).
type FrameTrack struct {
	// ID is the track's stable identity.
	ID uint32 `json:"id"`
	// LatDeg is the latitude in decimal degrees (positive north).
	LatDeg float64 `json:"lat_deg"`
	// LonDeg is the longitude in decimal degrees (positive east).
	LonDeg float64 `json:"lon_deg"`
	// HeightM is the height above the WGS84 ellipsoid, in metres.
	HeightM float64 `json:"height_m"`
	// GroundSpeedMPS is the horizontal ground speed, in metres per second.
	GroundSpeedMPS float64 `json:"ground_speed_mps"`
	// TrackAngleDeg is the course over ground, degrees clockwise from true
	// north in [0, 360).
	TrackAngleDeg float64 `json:"track_angle_deg"`
	// Confirmed reports whether the track is confirmed (vs. still tentative).
	Confirmed bool `json:"confirmed"`
	// Coasting reports whether the track is currently extrapolated because no
	// fresh measurement arrived this scan.
	Coasting bool `json:"coasting"`
	// UpdateAgeS is the data-time since the last real measurement, in seconds.
	UpdateAgeS float64 `json:"update_age_s"`
	// PositionUncertaintyM is the 1-sigma semi-major axis of the position
	// error ellipse, in metres.
	PositionUncertaintyM float64 `json:"position_uncertainty_m"`
}

// Frame is a complete picture of the air situation at one data time, as sent
// by Firefly over its WebSocket feed.
type Frame struct {
	// Time is the data time of this picture, in seconds (Firefly's
	// [Timestamp]; see docs/cross-project/todo-for-firefly.md for the
	// missing UTC reference).
	Time float64 `json:"time"`
	// Sensor identifies the reporting sensor (radar site/channel).
	Sensor uint16 `json:"sensor"`
	// Plots are the raw radar detections (input to the tracker) at this data
	// time.
	Plots []FramePlot `json:"plots"`
	// Tracks are the tracks at this data time, in wire form.
	Tracks []FrameTrack `json:"tracks"`
}
