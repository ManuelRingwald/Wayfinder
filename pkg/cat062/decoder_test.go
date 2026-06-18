package cat062

import (
	"testing"
)

// TestSignExtendI24 tests 24-bit sign extension to 32-bit.
func TestSignExtendI24(t *testing.T) {
	tests := []struct {
		name     string
		input    int32
		expected int32
	}{
		{"positive max 24-bit", 0x7FFFFF, 0x7FFFFF}, // 2^23 - 1
		{"negative -1", 0xFFFFFF, -1},               // all 24 bits set
		{"negative -8388608", 0x800000, -8388608},   // -(2^23)
		{"zero", 0x000000, 0},
		{"small positive", 0x000001, 1},
		{"small negative", 0xFFFFFF, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := signExtendI24(tt.input)
			if result != tt.expected {
				t.Errorf("input 0x%X: expected %d, got %d", tt.input, tt.expected, result)
			}
		})
	}
}

// TestFSPECParser tests FSPEC parsing with FX chaining.
func TestFSPECParser(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		items  map[uint8]bool // expected HasItem results
		offset int            // expected next offset
	}{
		{
			name: "single octet, FRN 1-7 all present",
			data: []byte{0xFE}, // 0b11111110: bits 7-1 set = FRN 1-7, bit 0 (FX) = 0
			items: map[uint8]bool{
				1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true,
				8: false, 9: false,
			},
			offset: 1,
		},
		{
			name: "two octets with FX chaining",
			data: []byte{0x81, 0x80}, // first: FRN1 + FX; second: FRN8, no FX
			items: map[uint8]bool{
				1: true, 2: false, 3: false, 4: false, 5: false, 6: false, 7: false,
				8: true, 9: false,
			},
			offset: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fspec, offset, err := NewFSPEC(tt.data, 0)
			if err != nil {
				t.Fatalf("NewFSPEC failed: %v", err)
			}
			if offset != tt.offset {
				t.Errorf("expected offset %d, got %d", tt.offset, offset)
			}
			for frn, expected := range tt.items {
				if fspec.HasItem(frn) != expected {
					t.Errorf("FRN %d: expected %v, got %v", frn, expected, fspec.HasItem(frn))
				}
			}
		})
	}
}

// TestDecodeDataSourceID tests I062/010 parsing.
func TestDecodeDataSourceID(t *testing.T) {
	// Minimal record: CAT + LEN + FSPEC(FRN1) + I062/010(SAC+SIC)
	// Total: 1 (CAT) + 2 (LEN) + 1 (FSPEC) + 2 (I062/010) = 6 bytes
	data := []byte{
		0x3E,       // CAT
		0x00, 0x06, // LEN (6 bytes including CAT+LEN)
		0x80,       // FSPEC: FRN1 present, no FX
		0x19, 0x02, // I062/010: SAC=0x19 (25), SIC=0x02 (2)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].Source.SAC != 0x19 || tracks[0].Source.SIC != 0x02 {
		t.Errorf("Source mismatch: got %v", tracks[0].Source)
	}
}

// TestDecodeTimeOfDay tests I062/070 parsing.
func TestDecodeTimeOfDay(t *testing.T) {
	// Record with I062/010 (FRN1) and I062/070 (FRN4)
	// FSPEC bits for FRN 1,4 present: bits 7,4 set -> 0b10010000 = 0x90
	// At 6:00 UTC: 6*3600 = 21600 seconds = 21600*128 ticks = 2764800 = 0x2A3000
	// Total: 1 (CAT) + 2 (LEN) + 1 (FSPEC) + 2 (I062/010) + 3 (I062/070) = 9 bytes
	data := []byte{
		0x3E,       // CAT
		0x00, 0x09, // LEN (9 bytes)
		0x90,       // FSPEC: FRN1,4 present
		0x19, 0x02, // I062/010 (SAC=0x19, SIC=0x02)
		0x2A, 0x30, 0x00, // I062/070 (21600 s = 0x2A3000)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	// Expect TimeOfDay ≈ 21600 seconds
	if tracks[0].TimeOfDay.Seconds < 21599.99 || tracks[0].TimeOfDay.Seconds > 21600.01 {
		t.Errorf("TimeOfDay mismatch: expected ≈21600, got %v", tracks[0].TimeOfDay.Seconds)
	}
}

// TestDecodeWGS84Position tests I062/105 parsing.
// Reference: latitude 45.0°, longitude 11.25° (Frankfurt region).
// LSB = 180 / 2^25 ≈ 5.36e-6 degrees per tick.
// 45.0° -> ticks ≈ 45 / (180/2^25) = 45 * 2^25 / 180 = 2^24 * 2 = 33554432
// 11.25° -> ticks ≈ 11.25 * 2^25 / 180 = 2^24 / 2 = 8388608
func TestDecodeWGS84Position(t *testing.T) {
	// FSPEC: FRN1 (bit 7), FRN5 (bit 3) -> 0b10001000 = 0x88
	// SAC/SIC: 0x19, 0x02
	// Lat=45.0°: ticks=0x00800000 (i32 big-endian)
	// Lon=11.25°: ticks=0x00200000 (i32 big-endian)
	// Total: 1 (CAT) + 2 (LEN) + 1 (FSPEC) + 2 (I062/010) + 8 (I062/105) = 14 bytes
	data := []byte{
		0x3E,       // CAT
		0x00, 0x0E, // LEN (14 bytes)
		0x88,       // FSPEC: FRN1,5 (bits 7,3)
		0x19, 0x02, // I062/010
		0x00, 0x80, 0x00, 0x00, // I062/105 Latitude (45°, i32 BE)
		0x00, 0x20, 0x00, 0x00, // I062/105 Longitude (11.25°, i32 BE)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	// Check latitude (tolerance: 1e-5 degrees)
	if tracks[0].WGS84.Latitude < 44.999 || tracks[0].WGS84.Latitude > 45.001 {
		t.Errorf("Latitude mismatch: expected ≈45, got %v", tracks[0].WGS84.Latitude)
	}
	// Check longitude (tolerance: 1e-5 degrees)
	if tracks[0].WGS84.Longitude < 11.249 || tracks[0].WGS84.Longitude > 11.251 {
		t.Errorf("Longitude mismatch: expected ≈11.25, got %v", tracks[0].WGS84.Longitude)
	}
}

// TestDecodeVelocity tests I062/185 parsing.
// Example: Vx=100 m/s, Vy=50 m/s.
// LSB=0.25 m/s -> ticks: 100/0.25=400, 50/0.25=200 (i16 big-endian).
func TestDecodeVelocity(t *testing.T) {
	// FSPEC: FRN1,7 -> bits 7,1 set -> 0b10000010 = 0x82
	// Total: 1 (CAT) + 2 (LEN) + 1 (FSPEC) + 2 (I062/010) + 4 (I062/185) = 10 bytes
	data := []byte{
		0x3E,       // CAT
		0x00, 0x0A, // LEN (10 bytes)
		0x82,       // FSPEC: FRN1,7
		0x19, 0x02, // I062/010
		0x01, 0x90, // I062/185 Vx=400 ticks=100m/s (i16 BE)
		0x00, 0xC8, // I062/185 Vy=200 ticks=50m/s (i16 BE)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	if tracks[0].Velocity.Vx < 99.99 || tracks[0].Velocity.Vx > 100.01 {
		t.Errorf("Vx mismatch: expected ≈100, got %v", tracks[0].Velocity.Vx)
	}
	if tracks[0].Velocity.Vy < 49.99 || tracks[0].Velocity.Vy > 50.01 {
		t.Errorf("Vy mismatch: expected ≈50, got %v", tracks[0].Velocity.Vy)
	}
}

// TestDecodeCartesianPosition tests I062/100 parsing with sign-extended i24 values.
func TestDecodeCartesianPosition(t *testing.T) {
	// FSPEC: FRN1 (bit 7), FRN6 (bit 2) -> 0b10000100 = 0x84
	// SAC/SIC: 0x19, 0x02
	// X = 1000.0 meters -> ticks = 1000 / 0.5 = 2000 = 0x0007D0 (i24 BE)
	// Y = -500.0 meters -> ticks = -500 / 0.5 = -1000 = 0xFFFC18 (i24 BE, two's complement)
	// Total: 1 (CAT) + 2 (LEN) + 1 (FSPEC) + 2 (I062/010) + 6 (I062/100) = 12 bytes
	data := []byte{
		0x3E,       // CAT
		0x00, 0x0C, // LEN (12 bytes)
		0x84,       // FSPEC: FRN1,6 (bits 7,2 set)
		0x19, 0x02, // I062/010
		0x00, 0x07, 0xD0, // I062/100 X = 2000 ticks = 1000.0 m (i24 BE)
		0xFF, 0xFC, 0x18, // I062/100 Y = -1000 ticks = -500.0 m (i24 BE)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	// Check X (tolerance: 0.01 m)
	if tracks[0].Cartesian.X < 999.99 || tracks[0].Cartesian.X > 1000.01 {
		t.Errorf("X mismatch: expected ≈1000.0, got %v", tracks[0].Cartesian.X)
	}
	// Check Y (tolerance: 0.01 m)
	if tracks[0].Cartesian.Y < -500.01 || tracks[0].Cartesian.Y > -499.99 {
		t.Errorf("Y mismatch: expected ≈-500.0, got %v", tracks[0].Cartesian.Y)
	}
}

// TestDecodeMultipleTracks tests multiple records in one block.
func TestDecodeMultipleTracks(t *testing.T) {
	// Two minimal records, each with I062/010 only.
	record1 := []byte{0x80, 0x19, 0x02} // FSPEC(FRN1), SAC=0x19, SIC=0x02
	record2 := []byte{0x80, 0x1A, 0x03} // FSPEC(FRN1), SAC=0x1A, SIC=0x03

	payload := append(record1, record2...)
	lenVal := 3 + len(payload) // header (3) + payload

	data := []byte{0x3E}
	data = append(data, byte(lenVal>>8), byte(lenVal&0xFF))
	data = append(data, payload...)

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 2 {
		t.Errorf("expected 2 tracks, got %d", len(tracks))
	}
	if tracks[0].Source.SAC != 0x19 || tracks[1].Source.SAC != 0x1A {
		t.Errorf("Source mismatch: track0=%v, track1=%v", tracks[0].Source, tracks[1].Source)
	}
}

// TestDecodeAdsbAge decodes a record whose I062/290 carries the ES (Extended
// Squitter / ADS-B) age subfield (primary bit 0x08, ICD 2.4.0) — the signal
// that a track has an ADS-B component (Firefly ADR 0019). The reference
// vector's PSR-only I062/290 [0x40, 0x08] is extended to [0x48, 0x08, 0x0C]:
// the primary subfield gains the ES bit (0x40|0x08 = 0x48), the PSR age stays
// 2 s (0x08), and the ES age 3 s (0x0C = 12 * 1/4 s) is appended. This is the
// byte-exact dump of Firefly's encoder test
// `single_track_with_adsb_hit_matches_reference_dump`; the FSPEC is unchanged
// (ES rides inside the already-present I062/290, FRN 14 — additive, ICD 2.4.0).
func TestDecodeAdsbAge(t *testing.T) {
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x29, // LEN = 41 (one byte more than the reference: I062/290 grew to 3 bytes)
		0x9F, 0x0F, 0x01, 0x04, // FSPEC {1, 4, 5, 6, 7, 12, 13, 14, 27} — unchanged
		0x19, 0x02, // I062/010 SAC/SIC
		0x00, 0x06, 0x00, // I062/070 time
		0x00, 0x80, 0x00, 0x00, // I062/105 latitude 45°
		0x00, 0x20, 0x00, 0x00, // I062/105 longitude 11.25°
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100 X=0, Y=0
		0x01, 0x90, // I062/185 Vx = 100 m/s
		0xFF, 0x38, // I062/185 Vy = -50 m/s
		0x00, 0x01, // I062/040 track number 1
		0x00,             // I062/080 confirmed, fresh
		0x48, 0x08, 0x0C, // I062/290 PSR age = 2 s + ES age = 3 s (ICD 2.4.0)
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500 APC = 100 m (FRN 27)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	track := tracks[0]
	// PSR age still decodes correctly alongside the new ES age.
	if track.UpdateAge.PSRAge < 1.99 || track.UpdateAge.PSRAge > 2.01 {
		t.Errorf("PSRAge mismatch: expected ≈2.0, got %v", track.UpdateAge.PSRAge)
	}
	if track.UpdateAge.ESAge == nil {
		t.Fatalf("ESAge mismatch: expected ≈3.0, got nil")
	}
	if *track.UpdateAge.ESAge < 2.99 || *track.UpdateAge.ESAge > 3.01 {
		t.Errorf("ESAge mismatch: expected ≈3.0, got %v", *track.UpdateAge.ESAge)
	}
	// The items after I062/290 must still decode correctly around the longer item.
	if track.TrackNum != 1 {
		t.Errorf("TrackNum mismatch: expected 1, got %d", track.TrackNum)
	}
	if track.Accuracy.APC < 99.99 || track.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC mismatch: expected ≈100.0, got %v", track.Accuracy.APC)
	}
}

// TestDecodeNoAdsbAge confirms a radar-only track (the PSR-only reference
// vector) decodes ESAge as nil — there is no spurious ADS-B badge.
func TestDecodeNoAdsbAge(t *testing.T) {
	data := []byte{
		0x3E,
		0x00, 0x28, // LEN = 40 (reference vector, PSR-only I062/290)
		0x9F, 0x0F, 0x01, 0x04,
		0x19, 0x02,
		0x00, 0x06, 0x00,
		0x00, 0x80, 0x00, 0x00,
		0x00, 0x20, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x90,
		0xFF, 0x38,
		0x00, 0x01,
		0x00,
		0x40, 0x08, // I062/290 PSR age = 2 s, no ES subfield
		0x80, 0x00, 0xC8, 0x00, 0xC8,
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if tracks[0].UpdateAge.ESAge != nil {
		t.Errorf("ESAge mismatch: expected nil for radar-only track, got %v", *tracks[0].UpdateAge.ESAge)
	}
}

// BenchmarkDecode benchmarks the decoder on a single record.
func BenchmarkDecodeRecord(b *testing.B) {
	data := []byte{
		0x3E,
		0x00, 0x13,
		0xA0,
		0x19, 0x02,
		0x02, 0x00, 0x00, 0x00,
		0x00, 0x80, 0x00, 0x00,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = DecodeDataBlock(data)
	}
}

// TestReferenceVector decodes the byte-exact CAT062 dump from Firefly's
// firefly-asterix encoder test `single_track_matches_reference_dump`
// (crates/firefly-asterix/src/cat062.rs). This is the ground truth for the
// wire contract between Firefly (encoder) and Wayfinder (decoder).
//
// Reference track: SAC/SIC=0x19/0x02, time=12.0s, lat=45°, lon=11.25°,
// system-cartesian at the reference point (0,0), Vx=100 m/s, Vy=-50 m/s,
// track #1, confirmed/fresh, PSR age=2s, position accuracy (APC)=100m.
func TestReferenceVector(t *testing.T) {
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x28, // LEN = 40
		0x9F, 0x0F, 0x01, 0x04, // FSPEC {1, 4, 5, 6, 7, 12, 13, 14, 27}
		0x19, 0x02, // I062/010 SAC/SIC
		0x00, 0x06, 0x00, // I062/070 time = 1536 ticks (12.0 s)
		0x00, 0x80, 0x00, 0x00, // I062/105 latitude 45°
		0x00, 0x20, 0x00, 0x00, // I062/105 longitude 11.25°
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100 X=0, Y=0 (reference point)
		0x01, 0x90, // I062/185 Vx = 100 m/s
		0xFF, 0x38, // I062/185 Vy = -50 m/s
		0x00, 0x01, // I062/040 track number 1
		0x00,       // I062/080 confirmed, fresh
		0x40, 0x08, // I062/290 PSR age = 2 s
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500 APC = 100 m (FRN 27)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	track := tracks[0]

	if track.Source.SAC != 0x19 || track.Source.SIC != 0x02 {
		t.Errorf("Source mismatch: got %v", track.Source)
	}
	if track.TimeOfDay.Seconds < 11.99 || track.TimeOfDay.Seconds > 12.01 {
		t.Errorf("TimeOfDay mismatch: expected ≈12.0, got %v", track.TimeOfDay.Seconds)
	}
	if track.WGS84.Latitude < 44.999 || track.WGS84.Latitude > 45.001 {
		t.Errorf("Latitude mismatch: expected ≈45.0, got %v", track.WGS84.Latitude)
	}
	if track.WGS84.Longitude < 11.249 || track.WGS84.Longitude > 11.251 {
		t.Errorf("Longitude mismatch: expected ≈11.25, got %v", track.WGS84.Longitude)
	}
	if track.Cartesian.X != 0.0 || track.Cartesian.Y != 0.0 {
		t.Errorf("Cartesian mismatch: expected (0,0), got (%v,%v)", track.Cartesian.X, track.Cartesian.Y)
	}
	if track.Velocity.Vx < 99.99 || track.Velocity.Vx > 100.01 {
		t.Errorf("Vx mismatch: expected ≈100, got %v", track.Velocity.Vx)
	}
	if track.Velocity.Vy < -50.01 || track.Velocity.Vy > -49.99 {
		t.Errorf("Vy mismatch: expected ≈-50, got %v", track.Velocity.Vy)
	}
	if track.TrackNum != 1 {
		t.Errorf("TrackNum mismatch: expected 1, got %d", track.TrackNum)
	}
	if !track.Status.Confirmed {
		t.Errorf("Status.Confirmed mismatch: expected true, got %v", track.Status.Confirmed)
	}
	if track.Status.Coasting {
		t.Errorf("Status.Coasting mismatch: expected false, got %v", track.Status.Coasting)
	}
	if track.UpdateAge.PSRAge < 1.99 || track.UpdateAge.PSRAge > 2.01 {
		t.Errorf("PSRAge mismatch: expected ≈2.0, got %v", track.UpdateAge.PSRAge)
	}
	if track.Accuracy.APC < 99.99 || track.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC mismatch: expected ≈100.0, got %v", track.Accuracy.APC)
	}
	// The reference track carries no Mode C reply, so I062/136 is absent.
	if track.FlightLevelFt != nil {
		t.Errorf("FlightLevelFt mismatch: expected nil, got %v", *track.FlightLevelFt)
	}
}

// TestDecodeFlightLevel decodes a record that includes I062/136 (Measured
// Flight Level, FRN 17), per ICD v2.0.0. The reference track plus a flight
// level of FL350 (35000 ft = 1400 * 25-ft steps = 0x0578).
func TestDecodeFlightLevel(t *testing.T) {
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x2A, // LEN = 42
		0x9F, 0x0F, 0x21, 0x04, // FSPEC {1, 4, 5, 6, 7, 12, 13, 14, 17, 27}
		0x19, 0x02, // I062/010 SAC/SIC
		0x00, 0x06, 0x00, // I062/070 time
		0x00, 0x80, 0x00, 0x00, // I062/105 latitude 45°
		0x00, 0x20, 0x00, 0x00, // I062/105 longitude 11.25°
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100 X=0, Y=0
		0x01, 0x90, // I062/185 Vx = 100 m/s
		0xFF, 0x38, // I062/185 Vy = -50 m/s
		0x00, 0x01, // I062/040 track number 1
		0x00,       // I062/080 confirmed, fresh
		0x40, 0x08, // I062/290 PSR age = 2 s
		0x05, 0x78, // I062/136 flight level = 1400 * 25 ft = 35000 ft
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500 APC = 100 m (FRN 27)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	track := tracks[0]
	if track.FlightLevelFt == nil {
		t.Fatalf("FlightLevelFt mismatch: expected ≈35000, got nil")
	}
	if *track.FlightLevelFt < 34999.0 || *track.FlightLevelFt > 35001.0 {
		t.Errorf("FlightLevelFt mismatch: expected ≈35000, got %v", *track.FlightLevelFt)
	}
	// The other items must still decode correctly around the new one.
	if track.TrackNum != 1 {
		t.Errorf("TrackNum mismatch: expected 1, got %d", track.TrackNum)
	}
	if track.Accuracy.APC < 99.99 || track.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC mismatch: expected ≈100.0, got %v", track.Accuracy.APC)
	}
}

// TestDecodeCallsign decodes a record that includes I062/245 (Target
// Identification / Callsign, FRN 10), per ICD v2.1.0. FRN 10 sits in the
// second FSPEC octet, which is already present in every record (additive,
// non-breaking). The callsign "DLH123" is packed as 8x6-bit IA-5 codes
// (D=4, L=12, H=8, '1'=49, '2'=50, '3'=51, space=32, space=32), matching
// Firefly's encoder test `target_identification_packs_eight_six_bit_ia5_codes`.
func TestDecodeCallsign(t *testing.T) {
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x2F, // LEN = 47
		0x9F, 0x2F, 0x01, 0x04, // FSPEC {1, 4, 5, 6, 7, 10, 12, 13, 14, 27}
		0x19, 0x02, // I062/010 SAC/SIC
		0x00, 0x06, 0x00, // I062/070 time
		0x00, 0x80, 0x00, 0x00, // I062/105 latitude 45°
		0x00, 0x20, 0x00, 0x00, // I062/105 longitude 11.25°
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100 X=0, Y=0
		0x01, 0x90, // I062/185 Vx = 100 m/s
		0xFF, 0x38, // I062/185 Vy = -50 m/s
		0x00, 0x10, 0xC2, 0x31, 0xCB, 0x38, 0x20, // I062/245 "DLH123  "
		0x00, 0x01, // I062/040 track number 1
		0x00,       // I062/080 confirmed, fresh
		0x40, 0x08, // I062/290 PSR age = 2 s
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500 APC = 100 m (FRN 27)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	track := tracks[0]
	if track.Callsign == nil {
		t.Fatalf("Callsign mismatch: expected \"DLH123\", got nil")
	}
	if *track.Callsign != "DLH123" {
		t.Errorf("Callsign mismatch: expected \"DLH123\", got %q", *track.Callsign)
	}
	// The other items must still decode correctly around the new one.
	if track.TrackNum != 1 {
		t.Errorf("TrackNum mismatch: expected 1, got %d", track.TrackNum)
	}
	if track.Accuracy.APC < 99.99 || track.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC mismatch: expected ≈100.0, got %v", track.Accuracy.APC)
	}
}

// TestDecodeTrackEnd decodes a record whose I062/080 carries the TSE bit
// (Track Service End, octet 2 bit 7 = 0x40), per ICD v2.2.0 — the final report
// for a track being deleted. The reference vector's one-octet status (0x00) is
// extended to two octets [0x01, 0x40]: octet 1 FX-set, octet 2 TSE. The decoder
// must recover Status.Ended while still reading the rest of the record. This
// mirrors Firefly's encoder test `track_status_carries_tse_when_ended`.
func TestDecodeTrackEnd(t *testing.T) {
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x29, // LEN = 41 (one byte more than the reference: I062/080 grew to 2 octets)
		0x9F, 0x0F, 0x01, 0x04, // FSPEC {1, 4, 5, 6, 7, 12, 13, 14, 27}
		0x19, 0x02, // I062/010 SAC/SIC
		0x00, 0x06, 0x00, // I062/070 time
		0x00, 0x80, 0x00, 0x00, // I062/105 latitude 45°
		0x00, 0x20, 0x00, 0x00, // I062/105 longitude 11.25°
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100 X=0, Y=0
		0x01, 0x90, // I062/185 Vx = 100 m/s
		0xFF, 0x38, // I062/185 Vy = -50 m/s
		0x00, 0x01, // I062/040 track number 1
		0x01, 0x40, // I062/080 octet 1 (FX) + octet 2 (TSE) — ended, confirmed, fresh
		0x40, 0x08, // I062/290 PSR age = 2 s
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500 APC = 100 m (FRN 27)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	track := tracks[0]
	if !track.Status.Ended {
		t.Errorf("Status.Ended mismatch: expected true (TSE set), got false")
	}
	if !track.Status.Confirmed {
		t.Errorf("Status.Confirmed mismatch: expected true, got false")
	}
	if track.Status.Coasting {
		t.Errorf("Status.Coasting mismatch: expected false, got true")
	}
	// The items after I062/080 must still decode correctly around the longer item.
	if track.TrackNum != 1 {
		t.Errorf("TrackNum mismatch: expected 1, got %d", track.TrackNum)
	}
	if track.Accuracy.APC < 99.99 || track.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC mismatch: expected ≈100.0, got %v", track.Accuracy.APC)
	}
}
