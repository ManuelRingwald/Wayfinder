package cat063

import "testing"

// referenceSingleSensor returns the byte-exact CAT063 block Firefly emits for
// one operational sensor (sensor SIC=1, SAC=0; SDPS 25/2; midnight). This is the
// cross-project ground truth from Firefly ICD §9 (3.0.0 / ADR 0032).
//
// 0x3F 0x00 0x0C 0xB8 0x19 0x02 0x00 0x00 0x00 0x00 0x01 0x00
// LEN=12; FSPEC=0xB8 (FRN 1+3+4+5); I063/010=19 02 (SDPS 25/2);
// I063/030=00 00 00; I063/050=00 01 (sensor 0/1); I063/060=0x00 (CON operational).
func referenceSingleSensor() []byte {
	return []byte{0x3F, 0x00, 0x0C, 0xB8, 0x19, 0x02, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00}
}

// referenceTwoSensors returns the byte-exact CAT063 block with two sensors
// (SIC=1 operational, SIC=2 degraded). From Firefly ICD §9 (3.0.0).
//
// 0x3F 0x00 0x15
// 0xB8 0x19 0x02 0x00 0x00 0x00 0x00 0x01 0x00   Sensor 1 operational
// 0xB8 0x19 0x02 0x00 0x00 0x00 0x00 0x02 0x40   Sensor 2 degraded (CON 0x40)
func referenceTwoSensors() []byte {
	return []byte{
		0x3F, 0x00, 0x15,
		0xB8, 0x19, 0x02, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00,
		0xB8, 0x19, 0x02, 0x00, 0x00, 0x00, 0x00, 0x02, 0x40,
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
	// Sensor identity now comes from I063/050 (ADR 0032), not I063/010.
	if s.SAC != 0 || s.SIC != 1 {
		t.Errorf("sensor (I063/050): got %02x/%02x, want 00/01", s.SAC, s.SIC)
	}
	// I063/010 now carries the SDPS identity (25/2 = 0x19/0x02).
	if s.SDPSSAC != 25 || s.SDPSSIC != 2 {
		t.Errorf("SDPS (I063/010): got %d/%d, want 25/2", s.SDPSSAC, s.SDPSSIC)
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
		t.Errorf("sensor 1 (I063/050): got %02x/%02x, want 00/01", statuses[0].SAC, statuses[0].SIC)
	}
	if !statuses[0].Operational {
		t.Errorf("sensor 1: expected operational, got degraded")
	}

	if statuses[1].SAC != 0 || statuses[1].SIC != 2 {
		t.Errorf("sensor 2 (I063/050): got %02x/%02x, want 00/02", statuses[1].SAC, statuses[1].SIC)
	}
	if statuses[1].Operational {
		t.Errorf("sensor 2: expected degraded, got operational")
	}
	// Every record carries the same SDPS identity (I063/010 = 25/2).
	for i, s := range statuses {
		if s.SDPSSAC != 25 || s.SDPSSIC != 2 {
			t.Errorf("record %d SDPS: got %d/%d, want 25/2", i, s.SDPSSAC, s.SDPSSIC)
		}
	}
}

// TestDecodeTimeOfDay checks the 1/128-s scaling of I063/030.
// 01:00:00 = 3600 s → 3600 × 128 = 460800 = 0x070800. In the 3.0.0 layout the
// ToD octets sit at offsets 6–8 (after CAT/LEN/FSPEC/I063/010).
func TestDecodeTimeOfDay(t *testing.T) {
	block := referenceSingleSensor()
	block[6], block[7], block[8] = 0x07, 0x08, 0x00
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if got := statuses[0].TimeOfDay; got < 3599.99 || got > 3600.01 {
		t.Errorf("time of day: got %f, want ~3600", got)
	}
}

// TestDecodeDegradedSensor checks that CON=0x40 (I063/060 bits 8/7 = 01) is
// decoded as Operational=false.
func TestDecodeDegradedSensor(t *testing.T) {
	block := referenceSingleSensor()
	block[len(block)-1] = 0x40 // CON = 0x40 → degraded
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if statuses[0].Operational {
		t.Errorf("operational: got true, want false for CON=0x40")
	}
}

// TestDecodeNotConnected checks that CON=0xC0 (bits 8/7 = 11, "not connected")
// is also not operational (extends beyond Firefly's current encoding,
// forward-compat).
func TestDecodeNotConnected(t *testing.T) {
	block := referenceSingleSensor()
	block[len(block)-1] = 0xC0 // CON = 0xC0 → not connected
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if statuses[0].Operational {
		t.Errorf("operational: got true, want false for CON=0xC0")
	}
}

// TestDecodeStandardFSPEC guards against a regression to the old non-standard
// compacted UAP (0xE0): the standard UAP subset is FRN 1+3+4+5 → FSPEC 0xB8.
func TestDecodeStandardFSPEC(t *testing.T) {
	block := referenceSingleSensor()
	if block[3] != 0xB8 {
		t.Fatalf("reference FSPEC: got 0x%02x, want 0xB8", block[3])
	}
	if _, err := DecodeSensorBlock(block); err != nil {
		t.Errorf("standard-UAP block should decode: %v", err)
	}
}

// TestDecodeSkipsReservedExpansion verifies the decoder length-skips a Reserved
// Expansion Field (RE, FRN 13) it does not consume — the forward-compatibility
// path for the per-source failure reason a later ICD adds (Firefly ADR 0033).
// The record still decodes to the sensor status that precedes the RE field.
func TestDecodeSkipsReservedExpansion(t *testing.T) {
	// FSPEC [0xB9, 0x04]: FRN 1+3+4+5 (0xB8) + FX (0x01) → second octet, FRN 13
	// (0x04). RE field: [0x03, 0xAA, 0xBB] (length octet 3 counts itself + 2).
	record := []byte{
		0xB9, 0x04, // FSPEC: FRN 1+3+4+5 + FRN 13 (RE)
		0x19, 0x02, // I063/010 SDPS 25/2
		0x00, 0x00, 0x00, // I063/030 time=0
		0x00, 0x01, // I063/050 sensor 0/1
		0x00,             // I063/060 CON operational
		0x03, 0xAA, 0xBB, // RE: explicit length 3 (skip 2 payload octets)
	}
	block := append([]byte{0x3F, 0x00, byte(3 + len(record))}, record...)
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock with RE field: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].SAC != 0 || statuses[0].SIC != 1 {
		t.Errorf("sensor: got %02x/%02x, want 00/01", statuses[0].SAC, statuses[0].SIC)
	}
	if !statuses[0].Operational {
		t.Errorf("operational: got false, want true")
	}
}

// TestDecodeRejectsSpareFRN ensures a present bit for a spare/unknown FRN whose
// length the decoder cannot know is rejected (not silently mis-parsed). FRN 12
// is spare in the CAT063 UAP.
func TestDecodeRejectsSpareFRN(t *testing.T) {
	// FSPEC [0xB9, 0x08]: FRN 1+3+4+5 + FX, then FRN 12 (0x08) present.
	record := []byte{
		0xB9, 0x08,
		0x19, 0x02,
		0x00, 0x00, 0x00,
		0x00, 0x01,
		0x00,
	}
	block := append([]byte{0x3F, 0x00, byte(3 + len(record))}, record...)
	if _, err := DecodeSensorBlock(block); err == nil {
		t.Errorf("expected error for spare FRN 12, got nil")
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

// referenceDegradedWithReason returns the byte-exact CAT063 block Firefly emits
// for one degraded sensor carrying an I063/RE SRC-REASON (Firefly ICD 3.1.0 §9,
// ADR 0033). reasonCode is the last octet (1=unreachable, 2=auth, 3=rate_limited).
//
// 0x3F 0x00 0x10 0xB9 0x04 0x19 0x02 0x00 0x00 0x00 0x00 0x01 0x40 0x03 0x80 <code>
// LEN=16; FSPEC=0xB9 0x04 (FRN 1+3+4+5 + FRN 13 RE); I063/060=0x40 (degraded);
// I063/RE=[LEN=3][SUBFIELD=0x80][SRC-REASON].
func referenceDegradedWithReason(reasonCode byte) []byte {
	return []byte{
		0x3F, 0x00, 0x10,
		0xB9, 0x04,
		0x19, 0x02,
		0x00, 0x00, 0x00,
		0x00, 0x01,
		0x40,
		0x03, 0x80, reasonCode,
	}
}

// TestDecodeReasonFromReservedExpansion verifies the decoder reads the SRC-REASON
// from the I063/RE field against Firefly's ICD 3.1.0 reference dump.
func TestDecodeReasonFromReservedExpansion(t *testing.T) {
	statuses, err := DecodeSensorBlock(referenceDegradedWithReason(0x01))
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.Operational {
		t.Errorf("operational: got true, want false (degraded)")
	}
	if s.SAC != 0 || s.SIC != 1 {
		t.Errorf("sensor: got %02x/%02x, want 00/01", s.SAC, s.SIC)
	}
	if s.Reason != ReasonUnreachable {
		t.Errorf("reason: got %q, want %q", s.Reason, ReasonUnreachable)
	}
}

// TestDecodeReasonCodes maps every defined SRC-REASON code to its string, and an
// unknown code to "" (degraded, reason unknown — forward tolerance).
func TestDecodeReasonCodes(t *testing.T) {
	cases := map[byte]string{
		0x01: ReasonUnreachable,
		0x02: ReasonAuth,
		0x03: ReasonRateLimited,
		0x09: "", // unknown → reason unknown
	}
	for code, want := range cases {
		statuses, err := DecodeSensorBlock(referenceDegradedWithReason(code))
		if err != nil {
			t.Fatalf("code 0x%02x: %v", code, err)
		}
		if got := statuses[0].Reason; got != want {
			t.Errorf("code 0x%02x: reason got %q, want %q", code, got, want)
		}
	}
}

// TestOperationalSensorHasNoReason confirms the plain (no-RE) reference block
// decodes to an empty reason.
func TestOperationalSensorHasNoReason(t *testing.T) {
	statuses, err := DecodeSensorBlock(referenceSingleSensor())
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if statuses[0].Reason != "" {
		t.Errorf("operational reason: got %q, want empty", statuses[0].Reason)
	}
}

// TestDominantReason picks the most operator-actionable reason among degraded
// sensors and ignores operational ones.
func TestDominantReason(t *testing.T) {
	// auth outranks rate_limited outranks unreachable.
	got := DominantReason([]SensorStatus{
		{Operational: false, Reason: ReasonUnreachable},
		{Operational: false, Reason: ReasonAuth},
		{Operational: false, Reason: ReasonRateLimited},
	})
	if got != ReasonAuth {
		t.Errorf("dominant: got %q, want %q", got, ReasonAuth)
	}
	// An operational sensor's reason is ignored; only the degraded one counts.
	got = DominantReason([]SensorStatus{
		{Operational: true, Reason: ReasonAuth},
		{Operational: false, Reason: ReasonUnreachable},
	})
	if got != ReasonUnreachable {
		t.Errorf("dominant with operational: got %q, want %q", got, ReasonUnreachable)
	}
	// All operational / no reasons → "".
	if got := DominantReason([]SensorStatus{{Operational: true}}); got != "" {
		t.Errorf("dominant all-ok: got %q, want empty", got)
	}
}

// TestDecodeLENExceedsData checks the bounds check when LEN > actual data.
func TestDecodeLENExceedsData(t *testing.T) {
	block := referenceSingleSensor()
	block[1] = 0x00
	block[2] = 0xFF // LEN=255 but data is only 12 bytes
	if _, err := DecodeSensorBlock(block); err == nil {
		t.Errorf("expected error for LEN > len(data), got nil")
	}
}
