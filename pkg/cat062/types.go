package cat062

import "fmt"

// DataSourceID identifies the radar (SAC/SIC).
type DataSourceID struct {
	SAC uint8 // Surveillance Area Code
	SIC uint8 // System Identification Code
}

func (d DataSourceID) String() string {
	return fmt.Sprintf("DataSource(%d.%d)", d.SAC, d.SIC)
}

// TimeOfDay is ASTERIX I062/070: seconds since UTC midnight, in 1/128-second units.
type TimeOfDay struct {
	Seconds float64
}

func (t TimeOfDay) String() string {
	hours := int(t.Seconds) / 3600
	mins := (int(t.Seconds) % 3600) / 60
	secs := int(t.Seconds) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
}

// WGS84Position is latitude/longitude in degrees.
type WGS84Position struct {
	Latitude  float64 // degrees, signed
	Longitude float64 // degrees, signed
}

// CartesianPosition is system-stereographic X/Y in meters.
type CartesianPosition struct {
	X float64 // meters
	Y float64 // meters
}

// Velocity is Vx/Vy in m/s (Cartesian frame).
type Velocity struct {
	Vx float64 // m/s
	Vy float64 // m/s
}

// TrackStatus holds I062/080 flags (variable length with FX).
type TrackStatus struct {
	Confirmed bool // CNF bit (octet 1): set = confirmed, unset = tentative
	Coasting  bool // CST bit (octet 4): set = coasting (no recent update)
	Ended     bool // TSE bit (octet 2): set = last report, track is being deleted
	// Monosensor is the MON bit (octet 1): only one sensor contributed to the
	// track within the freshness window, so no second source cross-checks the
	// estimate (more prone to ghosts/bias). A quality hint, not an operator
	// action. Firefly ICD 3.2.0 (ADR QW.3).
	Monosensor bool
	// SPI is the SPI bit (octet 1): the last associated report carried the
	// Special Position Identification pulse — the pilot pressed "ident" on the
	// controller's request. Transient (describes only the last report), so it
	// naturally follows the ~15–30 s the transponder emits the pulse.
	SPI bool
}

// UpdateAge is I062/290: time since last update, in seconds.
type UpdateAge struct {
	PSRAge float64 // Primary Surveillance Radar age, seconds
	// ESAge is the time since the last Extended Squitter (ADS-B) contribution,
	// in seconds, decoded from the ES subfield of I062/290 (ICD 2.4.0). It is
	// present only for tracks that have been updated by ADS-B; a radar-only
	// track leaves it nil. Its presence is what tells the ASD a track carries
	// an ADS-B component (Firefly ADR 0019).
	ESAge *float64 // optional, I062/290 ES (Extended Squitter / ADS-B) age, seconds

	// SSRAge, MDSAge and FLARMAge are the remaining per-technology update ages
	// from I062/290 (ICD 2.6.0, Firefly ADR 0027): SSR = Mode A/C replies,
	// MDS = Mode S, FLARM = the Firefly vendor subfield (no EUROCONTROL
	// standard bit). Like ESAge they are present only when the track has been
	// updated by that technology; together they give the ASD an authoritative
	// per-track provenance instead of the old frontend heuristic.
	SSRAge   *float64 // optional, I062/290 SSR (Mode A/C) age, seconds
	MDSAge   *float64 // optional, I062/290 MDS (Mode S) age, seconds
	FLARMAge *float64 // optional, I062/290 FLARM (vendor subfield) age, seconds
}

// PositionAccuracy is I062/500: estimated 1-sigma position uncertainty, in meters.
type PositionAccuracy struct {
	APC float64 // Accuracy of Calculated Position (Cartesian), meters
}

// DecodedTrack represents one ASTERIX System Track record.
type DecodedTrack struct {
	Source    DataSourceID
	TimeOfDay TimeOfDay
	WGS84     WGS84Position
	Cartesian CartesianPosition
	Velocity  Velocity
	TrackNum  uint16
	Status    TrackStatus
	UpdateAge UpdateAge
	Accuracy  PositionAccuracy
	Mode3A    *uint16 // optional, I062/060
	ICAOAddr  *uint32 // optional, I062/380 Target Address

	// FlightLevelFt is the measured barometric flight level in feet, decoded
	// from I062/136 when present (the track carries a Mode C reply). Optional.
	FlightLevelFt *float64 // optional, I062/136 Measured Flight Level

	// Callsign is the target identification (flight ID), decoded from I062/245
	// when present (the track carries a Mode S identification reply). Trailing
	// spaces are trimmed. Optional.
	Callsign *string // optional, I062/245 Target Identification
}

func (t DecodedTrack) String() string {
	return fmt.Sprintf(
		"Track{source=%v time=%v track_num=%d lat=%.6f lon=%.6f vx=%.2f vy=%.2f}",
		t.Source, t.TimeOfDay, t.TrackNum, t.WGS84.Latitude, t.WGS84.Longitude,
		t.Velocity.Vx, t.Velocity.Vy,
	)
}
