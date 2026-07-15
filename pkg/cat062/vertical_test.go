package cat062

import "testing"

// Vertical chain (I062/130 geometric altitude, I062/135 filtered barometric
// altitude with QNH bit, I062/220 rate of climb/descent), ICD 3.5.0 / #241.
// The byte vectors below are the byte-exact reference dump from Firefly's ICD
// §4.8 (encoder tests vertical_items_encode_byte_exactly_and_absence_is_unchanged
// / decode_recovers_vertical_items) — the ground truth for this decoder.

// verticalRecord builds a minimal CAT062 datablock carrying I062/010 (SAC/SIC)
// and exactly one vertical item at the given FRN (18/19/20, all in the third
// FSPEC octet), so a single reference vector can be decoded in isolation.
func verticalRecord(t *testing.T, frn uint8, item []byte) []byte {
	t.Helper()
	var bit byte
	switch frn {
	case 18: // I062/130
		bit = 0x10
	case 19: // I062/135
		bit = 0x08
	case 20: // I062/220
		bit = 0x04
	default:
		t.Fatalf("verticalRecord: unsupported frn %d", frn)
	}
	// FSPEC: octet1 = FRN1 (0x80) + FX (0x01); octet2 = FX only (0x01) to reach
	// the third octet; octet3 = the single vertical FRN bit (no FX).
	body := []byte{0x81, 0x01, bit, 0x19, 0x02} // FSPEC + I062/010 SAC/SIC
	body = append(body, item...)
	total := 3 + len(body) // CAT + LEN(2) header + body
	data := []byte{0x3E, byte(total >> 8), byte(total & 0xFF)}
	return append(data, body...)
}

func TestDecodeGeometricAltitudeReferenceVectors(t *testing.T) {
	cases := []struct {
		name  string
		bytes []byte
		want  float64
	}{
		{"10000ft", []byte{0x06, 0x40}, 10000.0}, // 1600 ticks × 6.25 ft
		{"-625ft", []byte{0xFF, 0x9C}, -625.0},   // -100 ticks × 6.25 ft
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tracks, err := DecodeDataBlock(verticalRecord(t, 18, tc.bytes))
			if err != nil {
				t.Fatalf("DecodeDataBlock failed: %v", err)
			}
			if len(tracks) != 1 {
				t.Fatalf("expected 1 track, got %d", len(tracks))
			}
			got := tracks[0].GeometricAltitudeFt
			if got == nil {
				t.Fatalf("GeometricAltitudeFt is nil, expected %v", tc.want)
			}
			if *got != tc.want {
				t.Errorf("GeometricAltitudeFt = %v, want %v", *got, tc.want)
			}
			// Barometric/ROCD must stay absent when only I062/130 is present.
			if tracks[0].BarometricAltitudeFt != nil || tracks[0].RateOfClimbDescentFtMin != nil {
				t.Errorf("unexpected other vertical items set: baro=%v rocd=%v",
					tracks[0].BarometricAltitudeFt, tracks[0].RateOfClimbDescentFtMin)
			}
		})
	}
}

func TestDecodeBarometricAltitudeReferenceVectors(t *testing.T) {
	cases := []struct {
		name        string
		bytes       []byte
		wantFt      float64
		wantQNHCorr bool
	}{
		// FL350 pressure altitude, uncorrected: 1400 ticks × 25 ft, QNH bit clear.
		{"FL350_uncorrected", []byte{0x05, 0x78}, 35000.0, false},
		// 3000 ft QNH-corrected: 120 ticks × 25 ft, QNH bit set.
		{"3000ft_qnh", []byte{0x80, 0x78}, 3000.0, true},
		// -400 ft QNH-corrected: -16 ticks (15-bit two's complement) × 25 ft.
		{"-400ft_qnh", []byte{0xFF, 0xF0}, -400.0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tracks, err := DecodeDataBlock(verticalRecord(t, 19, tc.bytes))
			if err != nil {
				t.Fatalf("DecodeDataBlock failed: %v", err)
			}
			if len(tracks) != 1 {
				t.Fatalf("expected 1 track, got %d", len(tracks))
			}
			got := tracks[0].BarometricAltitudeFt
			if got == nil {
				t.Fatalf("BarometricAltitudeFt is nil, expected %v", tc.wantFt)
			}
			if *got != tc.wantFt {
				t.Errorf("BarometricAltitudeFt = %v, want %v", *got, tc.wantFt)
			}
			if tracks[0].BaroQNHCorrected != tc.wantQNHCorr {
				t.Errorf("BaroQNHCorrected = %v, want %v", tracks[0].BaroQNHCorrected, tc.wantQNHCorr)
			}
		})
	}
}

func TestDecodeRateOfClimbDescentReferenceVectors(t *testing.T) {
	cases := []struct {
		name  string
		bytes []byte
		want  float64
	}{
		{"+3000ftmin", []byte{0x01, 0xE0}, 3000.0},  // 480 ticks × 6.25 ft/min
		{"-1200ftmin", []byte{0xFF, 0x40}, -1200.0}, // -192 ticks × 6.25 ft/min
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tracks, err := DecodeDataBlock(verticalRecord(t, 20, tc.bytes))
			if err != nil {
				t.Fatalf("DecodeDataBlock failed: %v", err)
			}
			if len(tracks) != 1 {
				t.Fatalf("expected 1 track, got %d", len(tracks))
			}
			got := tracks[0].RateOfClimbDescentFtMin
			if got == nil {
				t.Fatalf("RateOfClimbDescentFtMin is nil, expected %v", tc.want)
			}
			if *got != tc.want {
				t.Errorf("RateOfClimbDescentFtMin = %v, want %v", *got, tc.want)
			}
		})
	}
}

// TestDecodeVerticalChainFullRecord decodes a full track record carrying all
// three vertical items together (in UAP order after I062/136), verifying they
// coexist with the measured flight level and that the surrounding items still
// decode correctly around them (no FSPEC/offset drift).
func TestDecodeVerticalChainFullRecord(t *testing.T) {
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x30, // LEN = 48
		0x9F, 0x0F, 0x3D, 0x04, // FSPEC {1,4,5,6,7,12,13,14,17,18,19,20,27}
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
		0x05, 0x78, // I062/136 measured flight level = 35000 ft
		0x06, 0x40, // I062/130 geometric altitude = 10000 ft
		0x80, 0x78, // I062/135 barometric altitude = 3000 ft, QNH-corrected
		0x01, 0xE0, // I062/220 rate of climb = +3000 ft/min
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

	if track.FlightLevelFt == nil || *track.FlightLevelFt != 35000.0 {
		t.Errorf("FlightLevelFt = %v, want 35000", track.FlightLevelFt)
	}
	if track.GeometricAltitudeFt == nil || *track.GeometricAltitudeFt != 10000.0 {
		t.Errorf("GeometricAltitudeFt = %v, want 10000", track.GeometricAltitudeFt)
	}
	if track.BarometricAltitudeFt == nil || *track.BarometricAltitudeFt != 3000.0 {
		t.Errorf("BarometricAltitudeFt = %v, want 3000", track.BarometricAltitudeFt)
	}
	if !track.BaroQNHCorrected {
		t.Errorf("BaroQNHCorrected = false, want true")
	}
	if track.RateOfClimbDescentFtMin == nil || *track.RateOfClimbDescentFtMin != 3000.0 {
		t.Errorf("RateOfClimbDescentFtMin = %v, want 3000", track.RateOfClimbDescentFtMin)
	}
	// Surrounding items intact.
	if track.TrackNum != 1 {
		t.Errorf("TrackNum = %d, want 1", track.TrackNum)
	}
	if track.Accuracy.APC < 99.99 || track.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC = %v, want ≈100", track.Accuracy.APC)
	}
}

// TestDecodeVerticalAbsenceUnchanged verifies a record WITHOUT the vertical
// items (the pre-3.5.0 form) leaves all three fields nil and the QNH flag
// false — the additive contract must not synthesise a vertical solution.
func TestDecodeVerticalAbsenceUnchanged(t *testing.T) {
	// A record with I062/136 present but no FRN 18/19/20 (FSPEC third octet 0x21).
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x2A, // LEN = 42
		0x9F, 0x0F, 0x21, 0x04, // FSPEC {1,4,5,6,7,12,13,14,17,27}
		0x19, 0x02, // I062/010 SAC/SIC
		0x00, 0x06, 0x00, // I062/070 time
		0x00, 0x80, 0x00, 0x00, // I062/105 latitude 45°
		0x00, 0x20, 0x00, 0x00, // I062/105 longitude 11.25°
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100 X=0, Y=0
		0x01, 0x90, // I062/185 Vx
		0xFF, 0x38, // I062/185 Vy
		0x00, 0x01, // I062/040 track number 1
		0x00,       // I062/080
		0x40, 0x08, // I062/290 PSR age
		0x05, 0x78, // I062/136 flight level = 35000 ft
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500 APC
	}
	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	track := tracks[0]
	if track.GeometricAltitudeFt != nil {
		t.Errorf("GeometricAltitudeFt = %v, want nil", *track.GeometricAltitudeFt)
	}
	if track.BarometricAltitudeFt != nil {
		t.Errorf("BarometricAltitudeFt = %v, want nil", *track.BarometricAltitudeFt)
	}
	if track.RateOfClimbDescentFtMin != nil {
		t.Errorf("RateOfClimbDescentFtMin = %v, want nil", *track.RateOfClimbDescentFtMin)
	}
	if track.BaroQNHCorrected {
		t.Errorf("BaroQNHCorrected = true, want false")
	}
}

// TestDecodeBarometricTruncated ensures a record that claims I062/135 but is cut
// short is rejected, not read out of bounds (robust decoder, CLAUDE.md §7).
func TestDecodeBarometricTruncated(t *testing.T) {
	// FSPEC {1, 19}; I062/010 present but I062/135 has only 1 of its 2 octets.
	data := []byte{
		0x3E, 0x00, 0x09, // CAT, LEN = 9
		0x81, 0x01, 0x08, // FSPEC {1, 19}
		0x19, 0x02, // I062/010 SAC/SIC
		0x80, // I062/135 truncated (needs 2 octets)
	}
	if _, err := DecodeDataBlock(data); err == nil {
		t.Fatalf("expected truncation error, got nil")
	}
}
