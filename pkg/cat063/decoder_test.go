package cat063

import "testing"

// referenceSingleSensor returns the byte-exact CAT063 block Firefly emits for
// one operational sensor (SIC=1, SAC=0, midnight). This is the cross-project
// ground truth from Firefly ICD §9 / ADR 0022.
//
// 0x3F 0x00 0x0A 0xE0 0x00 0x01 0x00 0x00 0x00 0x00
// LEN=10; FSPEC=0xE0; I063/010=00 01; I063/030=00 00 00; I063/060=0x00.
func referenceSingleSensor() []byte {
	return []byte{0x3F, 0x00, 0x0A, 0xE0, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}
}

// referenceTwoSensors returns the byte-exact CAT063 block with two sensors
// (SIC=1 operational, SIC=2 degraded). From Firefly ICD §9.
//
// 0x3F 0x00 0x11
// 0xE0 0x00 0x01 0x00 0x00 0x00 0x00   Sensor 1 operational
// 0xE0 0x00 0x02 0x00 0x00 0x00 0x40   Sensor 2 degraded (NOGO 0x40)
func referenceTwoSensors() []byte {
	return []byte{
		0x3F, 0x00, 0x11,
		0xE0, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00,
		0xE0, 0x00, 0x02, 0x00, 0x00, 0x00, 0x40,
	}
}

// TestDecodeSingleOperationalSensor verifies the decoder against the ICD §9
// single-sensor reference dump.
func TestDecodeSingleOperationalSensor(t *testing.T) {
	statuses, err := DecodeSensorBlock(referenceSingleSensor())
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.SAC != 0 || s.SIC != 1 {
		t.Errorf("source: got %02x/%02x, want 00/01", s.SAC, s.SIC)
	}
	if s.TimeOfDay != 0 {
		t.Errorf("time of day: got %f, want 0", s.TimeOfDay)
	}
	if !s.Operational {
		t.Errorf("operational: got false, want true")
	}
}

// TestDecodeTwoSensors verifies the decoder against the ICD §9 two-sensor
// reference dump: first sensor operational, second degraded.
func TestDecodeTwoSensors(t *testing.T) {
	statuses, err := DecodeSensorBlock(referenceTwoSensors())
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	if statuses[0].SAC != 0 || statuses[0].SIC != 1 {
		t.Errorf("sensor 1 source: got %02x/%02x, want 00/01", statuses[0].SAC, statuses[0].SIC)
	}
	if !statuses[0].Operational {
		t.Errorf("sensor 1: expected operational, got degraded")
	}

	if statuses[1].SAC != 0 || statuses[1].SIC != 2 {
		t.Errorf("sensor 2 source: got %02x/%02x, want 00/02", statuses[1].SAC, statuses[1].SIC)
	}
	if statuses[1].Operational {
		t.Errorf("sensor 2: expected degraded, got operational")
	}
}

// TestDecodeTimeOfDay checks the 1/128-s scaling of I063/030.
// 01:00:00 = 3600 s → 3600 × 128 = 460800 = 0x070800.
func TestDecodeTimeOfDay(t *testing.T) {
	block := referenceSingleSensor()
	// Overwrite ToD bytes (offsets 6–8) with 0x07 0x08 0x00.
	block[6], block[7], block[8] = 0x07, 0x08, 0x00
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if got := statuses[0].TimeOfDay; got < 3599.99 || got > 3600.01 {
		t.Errorf("time of day: got %f, want ~3600", got)
	}
}

// TestDecodeDegradedSensor checks that NOGO=0x40 (I063/060 bits 8/7 = 01) is
// decoded as Operational=false.
func TestDecodeDegradedSensor(t *testing.T) {
	block := referenceSingleSensor()
	block[len(block)-1] = 0x40 // NOGO = 0x40 → degraded
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if statuses[0].Operational {
		t.Errorf("operational: got true, want false for NOGO=0x40")
	}
}

// TestDecodeNotConnected checks that NOGO=0x80 (bits 8/7 = 10) is also not
// operational (extends beyond Firefly's current encoding, forward-compat).
func TestDecodeNotConnected(t *testing.T) {
	block := referenceSingleSensor()
	block[len(block)-1] = 0x80 // NOGO = 0x80 → not connected
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if statuses[0].Operational {
		t.Errorf("operational: got true, want false for NOGO=0x80")
	}
}

// TestDecodeEmptyBlock checks that a block with no records (header-only) is
// valid and returns an empty slice.
func TestDecodeEmptyBlock(t *testing.T) {
	block := []byte{0x3F, 0x00, 0x03} // LEN=3 = header only, no records
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}

// TestDecodeRejectsWrongCategory ensures a non-CAT063 block is rejected.
func TestDecodeRejectsWrongCategory(t *testing.T) {
	block := referenceSingleSensor()
	block[0] = 0x3E // CAT062 instead of CAT063
	if _, err := DecodeSensorBlock(block); err == nil {
		t.Errorf("expected error for wrong category, got nil")
	}
}

// TestDecodeRejectsTruncatedInput ensures no panic and an error on short input.
func TestDecodeRejectsTruncatedInput(t *testing.T) {
	full := referenceSingleSensor()
	for cut := 0; cut < len(full); cut++ {
		if _, err := DecodeSensorBlock(full[:cut]); err == nil && cut < len(full) {
			t.Errorf("cut=%d: expected error for truncated input, got nil", cut)
		}
	}
}

// TestDecodeLENExceedsData checks the bounds check when LEN > actual data.
func TestDecodeLENExceedsData(t *testing.T) {
	block := referenceSingleSensor()
	block[1] = 0x00
	block[2] = 0xFF // LEN=255 but data is only 10 bytes
	if _, err := DecodeSensorBlock(block); err == nil {
		t.Errorf("expected error for LEN > len(data), got nil")
	}
}
