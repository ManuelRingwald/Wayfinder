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

// TestReferenceVector tests against a known Firefly encoder output.
// (Placeholder for M1.1.d when we integrate actual Firefly reference vectors.)
func TestReferenceVector(t *testing.T) {
	// TODO: Embed a byte-exact reference dump from Firefly's encoder.
	// Example: single_track_matches_reference_dump from firefly-asterix tests.
	t.Skip("Firefly reference vector integration pending (M1.1.d)")
}
