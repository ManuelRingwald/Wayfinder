package cat062

import "testing"

// Kinematics chain (I062/210 Calculated Acceleration, I062/200 Mode of Movement),
// ICD 3.6.0 / #242. The byte vectors below are the byte-exact reference dump from
// Firefly's ICD §4.9 (encoder test kinematics_items_encode_byte_exactly_and_round_trip)
// — the ground truth for this decoder.

// accelRecord builds a minimal CAT062 datablock carrying I062/010 (SAC/SIC) and
// I062/210 (FRN 8, second FSPEC octet, bit 0x80).
func accelRecord(item []byte) []byte {
	// FSPEC: octet1 = FRN1 (0x80) + FX (0x01); octet2 = FRN8 (0x80), no FX.
	body := []byte{0x81, 0x80, 0x19, 0x02} // FSPEC + I062/010 SAC/SIC
	body = append(body, item...)
	total := 3 + len(body)
	data := []byte{0x3E, byte(total >> 8), byte(total & 0xFF)}
	return append(data, body...)
}

// modeRecord builds a minimal CAT062 datablock carrying I062/010 and I062/200
// (FRN 15, third FSPEC octet, bit 0x80).
func modeRecord(b byte) []byte {
	// FSPEC: octet1 = FRN1 (0x80) + FX; octet2 = FX only; octet3 = FRN15 (0x80).
	body := []byte{0x81, 0x01, 0x80, 0x19, 0x02, b}
	total := 3 + len(body)
	data := []byte{0x3E, byte(total >> 8), byte(total & 0xFF)}
	return append(data, body...)
}

func TestDecodeAccelerationReferenceVectors(t *testing.T) {
	cases := []struct {
		name   string
		bytes  []byte
		wantAx float64
		wantAy float64
	}{
		{"ax+1_ay-0.5", []byte{0x04, 0xFE}, 1.0, -0.5}, // +4 ticks, -2 ticks × 0.25
		{"clamped", []byte{0x7F, 0x80}, 31.75, -32.0},  // i8 field range +127 / -128 ticks
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tracks, err := DecodeDataBlock(accelRecord(tc.bytes))
			if err != nil {
				t.Fatalf("DecodeDataBlock failed: %v", err)
			}
			if len(tracks) != 1 {
				t.Fatalf("expected 1 track, got %d", len(tracks))
			}
			ax, ay := tracks[0].AccelAxMS2, tracks[0].AccelAyMS2
			if ax == nil || ay == nil {
				t.Fatalf("acceleration nil: ax=%v ay=%v", ax, ay)
			}
			if *ax != tc.wantAx {
				t.Errorf("AccelAxMS2 = %v, want %v", *ax, tc.wantAx)
			}
			if *ay != tc.wantAy {
				t.Errorf("AccelAyMS2 = %v, want %v", *ay, tc.wantAy)
			}
			// Mode of Movement must stay absent when only I062/210 is present.
			if tracks[0].MotionCourse != nil || tracks[0].MotionSpeed != nil || tracks[0].MotionVertical != nil {
				t.Errorf("unexpected mode of movement set")
			}
		})
	}
}

func TestDecodeModeOfMovementReferenceVectors(t *testing.T) {
	rightIncrClimb := modeRecord(0x54) // right turn + GS increasing + climb
	tracks, err := DecodeDataBlock(rightIncrClimb)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	tr := tracks[0]
	if tr.MotionCourse == nil || *tr.MotionCourse != CourseRight {
		t.Errorf("MotionCourse = %v, want right", tr.MotionCourse)
	}
	if tr.MotionSpeed == nil || *tr.MotionSpeed != SpeedIncreasing {
		t.Errorf("MotionSpeed = %v, want increasing", tr.MotionSpeed)
	}
	if tr.MotionVertical == nil || *tr.MotionVertical != VerticalClimb {
		t.Errorf("MotionVertical = %v, want climb", tr.MotionVertical)
	}
	// Acceleration must stay absent when only I062/200 is present.
	if tr.AccelAxMS2 != nil || tr.AccelAyMS2 != nil {
		t.Errorf("unexpected acceleration set")
	}

	// 0xB0: left turn + LONG undetermined + level. The undetermined axis (LONG=3)
	// must be a nil pointer, not a spurious member.
	leftUndetLevel := modeRecord(0xB0)
	tracks, err = DecodeDataBlock(leftUndetLevel)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	tr = tracks[0]
	if tr.MotionCourse == nil || *tr.MotionCourse != CourseLeft {
		t.Errorf("MotionCourse = %v, want left", tr.MotionCourse)
	}
	if tr.MotionSpeed != nil {
		t.Errorf("MotionSpeed = %v, want nil (undetermined)", *tr.MotionSpeed)
	}
	if tr.MotionVertical == nil || *tr.MotionVertical != VerticalLevel {
		t.Errorf("MotionVertical = %v, want level", tr.MotionVertical)
	}
}

// TestDecodeModeOfMovementAllUndetermined verifies a byte whose three axes are all
// undetermined (0xFC = TRANS/LONG/VERT all 3) yields no member — a robust decoder
// must not synthesise a state (Firefly omits the item entirely in this case).
func TestDecodeModeOfMovementAllUndetermined(t *testing.T) {
	tracks, err := DecodeDataBlock(modeRecord(0xFC))
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	tr := tracks[0]
	if tr.MotionCourse != nil || tr.MotionSpeed != nil || tr.MotionVertical != nil {
		t.Errorf("all-undetermined byte set a member: course=%v speed=%v vert=%v",
			tr.MotionCourse, tr.MotionSpeed, tr.MotionVertical)
	}
}

// TestDecodeKinematicsFullRecord decodes a full record carrying both kinematics
// items (I062/210 at FRN 8, I062/200 at FRN 15) alongside the ordinary items, and
// checks the surrounding items still decode correctly (no FSPEC/offset drift).
func TestDecodeKinematicsFullRecord(t *testing.T) {
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x2B, // LEN = 43
		0x9F, 0x8F, 0x81, 0x04, // FSPEC {1,4,5,6,7,8,12,13,14,15,27}
		0x19, 0x02, // I062/010 SAC/SIC
		0x00, 0x06, 0x00, // I062/070 time
		0x00, 0x80, 0x00, 0x00, // I062/105 latitude 45°
		0x00, 0x20, 0x00, 0x00, // I062/105 longitude 11.25°
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100 X=0, Y=0
		0x01, 0x90, // I062/185 Vx = 100 m/s
		0xFF, 0x38, // I062/185 Vy = -50 m/s
		0x04, 0xFE, // I062/210 Ax = +1.0, Ay = -0.5 m/s²
		0x00, 0x01, // I062/040 track number 1
		0x00,       // I062/080 confirmed, fresh
		0x40, 0x08, // I062/290 PSR age = 2 s
		0x54,                         // I062/200 right turn + GS increasing + climb
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500 APC = 100 m (FRN 27)
	}

	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	tr := tracks[0]

	if tr.AccelAxMS2 == nil || *tr.AccelAxMS2 != 1.0 {
		t.Errorf("AccelAxMS2 = %v, want 1.0", tr.AccelAxMS2)
	}
	if tr.AccelAyMS2 == nil || *tr.AccelAyMS2 != -0.5 {
		t.Errorf("AccelAyMS2 = %v, want -0.5", tr.AccelAyMS2)
	}
	if tr.MotionCourse == nil || *tr.MotionCourse != CourseRight {
		t.Errorf("MotionCourse = %v, want right", tr.MotionCourse)
	}
	if tr.MotionSpeed == nil || *tr.MotionSpeed != SpeedIncreasing {
		t.Errorf("MotionSpeed = %v, want increasing", tr.MotionSpeed)
	}
	if tr.MotionVertical == nil || *tr.MotionVertical != VerticalClimb {
		t.Errorf("MotionVertical = %v, want climb", tr.MotionVertical)
	}
	// Surrounding items intact.
	if tr.TrackNum != 1 {
		t.Errorf("TrackNum = %d, want 1", tr.TrackNum)
	}
	if tr.Velocity.Vx < 99.9 || tr.Velocity.Vx > 100.1 {
		t.Errorf("Vx = %v, want ~100", tr.Velocity.Vx)
	}
	if tr.Accuracy.APC < 99.99 || tr.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC = %v, want ≈100", tr.Accuracy.APC)
	}
}

// TestDecodeKinematicsAbsenceUnchanged verifies a record without FRN 8/15 leaves
// the kinematics fields nil — the additive contract must not synthesise motion.
func TestDecodeKinematicsAbsenceUnchanged(t *testing.T) {
	// FSPEC {1, 12} only (I062/010 + I062/040), no kinematics.
	data := []byte{
		0x3E, 0x00, 0x09, // CAT, LEN = 9
		0x81, 0x08, // FSPEC {1, 12}
		0x19, 0x02, // I062/010 SAC/SIC
		0x00, 0x01, // I062/040 track number 1
	}
	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	tr := tracks[0]
	if tr.AccelAxMS2 != nil || tr.AccelAyMS2 != nil {
		t.Errorf("acceleration set on a record without I062/210")
	}
	if tr.MotionCourse != nil || tr.MotionSpeed != nil || tr.MotionVertical != nil {
		t.Errorf("mode of movement set on a record without I062/200")
	}
}

// TestDecodeAccelerationTruncated ensures a record claiming I062/210 but cut short
// is rejected, not read out of bounds (robust decoder, CLAUDE.md §7).
func TestDecodeAccelerationTruncated(t *testing.T) {
	data := []byte{
		0x3E, 0x00, 0x08, // CAT, LEN = 8
		0x81, 0x80, // FSPEC {1, 8}
		0x19, 0x02, // I062/010 SAC/SIC
		0x04, // I062/210 truncated (needs 2 octets)
	}
	if _, err := DecodeDataBlock(data); err == nil {
		t.Fatalf("expected truncation error, got nil")
	}
}
