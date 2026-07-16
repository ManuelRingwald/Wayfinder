package cat062

import "testing"

// Flight Plan Related Data (I062/390), ICD 3.7.0 / #245. The byte vectors below
// are the byte-exact reference dump from Firefly's ICD §4.10 (encoder test
// flight_plan_item_encodes_byte_exactly_and_round_trips) — the ground truth for
// this decoder.

// flightPlanRecord builds a minimal CAT062 datablock carrying I062/010 (SAC/SIC)
// and I062/390 (FRN 21, third FSPEC octet, bit 0x02).
func flightPlanRecord(item []byte) []byte {
	// FSPEC: octet1 = FRN1 (0x80) + FX; octet2 = FX only; octet3 = FRN21 (0x02).
	body := []byte{0x81, 0x01, 0x02, 0x19, 0x02} // FSPEC + I062/010 SAC/SIC
	body = append(body, item...)
	total := 3 + len(body)
	data := []byte{0x3E, byte(total >> 8), byte(total & 0xFF)}
	return append(data, body...)
}

func str(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestDecodeFlightPlanFull(t *testing.T) {
	// Plan DLH123, EDDF → EDDM: spec 43 80, then CSN/DEP/DST.
	item := []byte{
		0x43, 0x80, // spec: CSN(#2) + DEP(#7) + FX; DST(#8)
		0x44, 0x4C, 0x48, 0x31, 0x32, 0x33, 0x20, // CSN "DLH123 "
		0x45, 0x44, 0x44, 0x46, // DEP "EDDF"
		0x45, 0x44, 0x44, 0x4D, // DST "EDDM"
	}
	tracks, err := DecodeDataBlock(flightPlanRecord(item))
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	tr := tracks[0]
	if tr.PlanCallsign == nil || *tr.PlanCallsign != "DLH123" {
		t.Errorf("PlanCallsign = %q, want DLH123", str(tr.PlanCallsign))
	}
	if tr.PlanDeparture == nil || *tr.PlanDeparture != "EDDF" {
		t.Errorf("PlanDeparture = %q, want EDDF", str(tr.PlanDeparture))
	}
	if tr.PlanDestination == nil || *tr.PlanDestination != "EDDM" {
		t.Errorf("PlanDestination = %q, want EDDM", str(tr.PlanDestination))
	}
}

func TestDecodeFlightPlanCallsignOnly(t *testing.T) {
	// Plan BAW22, callsign only: spec 40, then CSN. DEP/DST absent.
	item := []byte{
		0x40,                                     // spec: CSN(#2) only, no FX
		0x42, 0x41, 0x57, 0x32, 0x32, 0x20, 0x20, // CSN "BAW22  "
	}
	tracks, err := DecodeDataBlock(flightPlanRecord(item))
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	tr := tracks[0]
	if tr.PlanCallsign == nil || *tr.PlanCallsign != "BAW22" {
		t.Errorf("PlanCallsign = %q, want BAW22", str(tr.PlanCallsign))
	}
	if tr.PlanDeparture != nil {
		t.Errorf("PlanDeparture = %q, want nil", *tr.PlanDeparture)
	}
	if tr.PlanDestination != nil {
		t.Errorf("PlanDestination = %q, want nil", *tr.PlanDestination)
	}
}

// TestDecodeFlightPlanSkipsUnknownFixedSubfield verifies the decoder length-skips
// a known fixed-length subfield it does not consume (here #4 FCT, 1 octet) and
// still reads the callsign — forward tolerance for Firefly's additive plan growth.
func TestDecodeFlightPlanSkipsUnknownFixedSubfield(t *testing.T) {
	// spec 50 = CSN(#2, 0x40) + FCT(#4, 0x10), no FX; then CSN(7) + FCT(1).
	item := []byte{
		0x50,
		0x44, 0x4C, 0x48, 0x31, 0x32, 0x33, 0x20, // CSN "DLH123 "
		0x07, // FCT (1 octet, skipped)
	}
	tracks, err := DecodeDataBlock(flightPlanRecord(item))
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if tracks[0].PlanCallsign == nil || *tracks[0].PlanCallsign != "DLH123" {
		t.Errorf("PlanCallsign = %q, want DLH123", str(tracks[0].PlanCallsign))
	}
}

// TestDecodeFlightPlanRejectsVariableSubfield verifies a record carrying the
// variable/repetitive subfield #12 (TOD) is rejected, not mis-parsed — the decoder
// cannot length-skip it, so it must fail rather than desynchronise (robust decoder).
func TestDecodeFlightPlanRejectsVariableSubfield(t *testing.T) {
	// spec 41 08 = CSN(#2) + FX; TOD(#12, 0x08). Then CSN(7), then TOD (unhandled).
	item := []byte{
		0x41, 0x08,
		0x44, 0x4C, 0x48, 0x31, 0x32, 0x33, 0x20, // CSN "DLH123 "
		0x01, 0x00, 0x00, 0x00, 0x00, // some TOD bytes (must not be consumed)
	}
	if _, err := DecodeDataBlock(flightPlanRecord(item)); err == nil {
		t.Fatalf("expected rejection of variable subfield #12, got nil")
	}
}

// TestDecodeFlightPlanFullRecord decodes a full track record carrying I062/390 at
// FRN 21 alongside the ordinary items, and checks the surrounding items still
// decode correctly (no FSPEC/offset drift).
func TestDecodeFlightPlanFullRecord(t *testing.T) {
	data := []byte{
		0x3E,       // CAT 62
		0x00, 0x39, // LEN = 57
		0x9F, 0x0F, 0x03, 0x04, // FSPEC {1,4,5,6,7,12,13,14,21,27}
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
		// I062/390 DLH123 EDDF → EDDM
		0x43, 0x80,
		0x44, 0x4C, 0x48, 0x31, 0x32, 0x33, 0x20,
		0x45, 0x44, 0x44, 0x46,
		0x45, 0x44, 0x44, 0x4D,
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
	if tr.PlanCallsign == nil || *tr.PlanCallsign != "DLH123" {
		t.Errorf("PlanCallsign = %q, want DLH123", str(tr.PlanCallsign))
	}
	if tr.PlanDeparture == nil || *tr.PlanDeparture != "EDDF" {
		t.Errorf("PlanDeparture = %q, want EDDF", str(tr.PlanDeparture))
	}
	if tr.PlanDestination == nil || *tr.PlanDestination != "EDDM" {
		t.Errorf("PlanDestination = %q, want EDDM", str(tr.PlanDestination))
	}
	// Surrounding items intact.
	if tr.TrackNum != 1 {
		t.Errorf("TrackNum = %d, want 1", tr.TrackNum)
	}
	if tr.Accuracy.APC < 99.99 || tr.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC = %v, want ≈100", tr.Accuracy.APC)
	}
}

// TestDecodeFlightPlanAbsenceUnchanged verifies a record without FRN 21 leaves the
// plan fields nil — the additive contract must not synthesise a correlation.
func TestDecodeFlightPlanAbsenceUnchanged(t *testing.T) {
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
	if tr.PlanCallsign != nil || tr.PlanDeparture != nil || tr.PlanDestination != nil {
		t.Errorf("flight-plan fields set on a record without I062/390")
	}
}

// TestDecodeFlightPlanTruncated ensures a record claiming I062/390 CSN but cut
// short is rejected, not read out of bounds (robust decoder, CLAUDE.md §7).
func TestDecodeFlightPlanTruncated(t *testing.T) {
	item := []byte{
		0x40,             // spec: CSN(#2) only
		0x44, 0x4C, 0x48, // CSN truncated (needs 7 octets)
	}
	if _, err := DecodeDataBlock(flightPlanRecord(item)); err == nil {
		t.Fatalf("expected truncation error, got nil")
	}
}
