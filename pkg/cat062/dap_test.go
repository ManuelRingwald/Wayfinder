package cat062

import (
	"math"
	"testing"
)

// TestDecodeModeSDaps decodes a record whose I062/380 carries the Mode-S DAP
// subfields, against Firefly's ICD 3.4.0 reference dump (§4.7): ADR 0x3C65AC,
// MHG 270°, SAL 35000 ft, IAS 250 kt, Mach 0.784. The record wraps the item
// between I062/010 and I062/040 so a desync in the now-compound I062/380 would
// corrupt the trailing track number — the key regression this guards.
//
// FSPEC {1, 11, 12} = 0x81 0x18. I062/380 spec 0xA5 0x01 0x01 0x0C selects
// subfields #1 (ADR), #3 (MHG), #6 (SAL), #26 (IAR), #27 (MAC).
func TestDecodeModeSDaps(t *testing.T) {
	data := []byte{
		0x3E, 0x00, 0x18, // CAT 62, LEN = 24
		0x81, 0x18, // FSPEC {1, 11, 12}
		0x19, 0x02, // I062/010
		0xA5, 0x01, 0x01, 0x0C, // I062/380 subfield spec (#1,#3,#6,#26,#27)
		0x3C, 0x65, 0xAC, // #1 ADR
		0xC0, 0x00, // #3 MHG = 270°
		0xC5, 0x78, // #6 SAL = 35000 ft (source MCP/FCU, SAS set)
		0x00, 0xFA, // #26 IAR = 250 kt
		0x00, 0x62, // #27 MAC = 0.784
		0x00, 0x2A, // I062/040 track 42
	}
	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	trk := tracks[0]

	if trk.ICAOAddr == nil || *trk.ICAOAddr != 0x3C65AC {
		t.Errorf("ICAOAddr: got %v, want 0x3C65AC", trk.ICAOAddr)
	}
	if trk.SelectedAltitudeFt == nil || math.Abs(*trk.SelectedAltitudeFt-35000) > 0.01 {
		t.Errorf("SelectedAltitudeFt: got %v, want 35000", trk.SelectedAltitudeFt)
	}
	if trk.MagneticHeadingDeg == nil || math.Abs(*trk.MagneticHeadingDeg-270) > 0.01 {
		t.Errorf("MagneticHeadingDeg: got %v, want 270", trk.MagneticHeadingDeg)
	}
	if trk.IndicatedAirspeedKt == nil || math.Abs(*trk.IndicatedAirspeedKt-250) > 0.01 {
		t.Errorf("IndicatedAirspeedKt: got %v, want 250", trk.IndicatedAirspeedKt)
	}
	if trk.MachNumber == nil || math.Abs(*trk.MachNumber-0.784) > 0.0001 {
		t.Errorf("MachNumber: got %v, want 0.784", trk.MachNumber)
	}
	// The track number after the compound item must still decode — proof the
	// subfield-driven parse consumed I062/380 exactly, with no desync.
	if trk.TrackNum != 42 {
		t.Errorf("TrackNum: got %d, want 42 (I062/380 desync?)", trk.TrackNum)
	}
}

// TestDecodeDapsBackwardCompatAdrOnly confirms the pre-3.4.0 form — I062/380 =
// 0x80 (ADR only) + 3 address octets — still decodes: ICAO address present, all
// DAP fields nil, trailing item intact.
func TestDecodeDapsBackwardCompatAdrOnly(t *testing.T) {
	data := []byte{
		0x3E, 0x00, 0x0D, // CAT 62, LEN = 13
		0x81, 0x18, // FSPEC {1, 11, 12}
		0x19, 0x02, // I062/010
		0x80, 0x3C, 0x65, 0xAC, // I062/380 spec 0x80 (ADR only) + address
		0x00, 0x2A, // I062/040 track 42
	}
	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock: %v", err)
	}
	trk := tracks[0]
	if trk.ICAOAddr == nil || *trk.ICAOAddr != 0x3C65AC {
		t.Errorf("ICAOAddr: got %v, want 0x3C65AC", trk.ICAOAddr)
	}
	if trk.SelectedAltitudeFt != nil || trk.MagneticHeadingDeg != nil ||
		trk.IndicatedAirspeedKt != nil || trk.MachNumber != nil {
		t.Errorf("expected no DAPs for an ADR-only I062/380, got SAL=%v MHG=%v IAS=%v MACH=%v",
			trk.SelectedAltitudeFt, trk.MagneticHeadingDeg, trk.IndicatedAirspeedKt, trk.MachNumber)
	}
	if trk.TrackNum != 42 {
		t.Errorf("TrackNum: got %d, want 42", trk.TrackNum)
	}
}

// TestDecodeSelectedAltitudeNegative verifies the 13-bit two's-complement sign
// extension of SAL: a raw field of 0x1FD8 (bits 13-1 = -40) decodes to -1000 ft.
func TestDecodeSelectedAltitudeNegative(t *testing.T) {
	data := []byte{
		0x3E, 0x00, 0x0F, // CAT 62, LEN = 15
		0x81, 0x18, // FSPEC {1, 11, 12}
		0x19, 0x02, // I062/010
		0x84, 0x3C, 0x65, 0xAC, 0x1F, 0xD8, // I062/380: #1 ADR + #6 SAL = -1000 ft
		0x00, 0x2A, // I062/040 track 42
	}
	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock: %v", err)
	}
	if got := tracks[0].SelectedAltitudeFt; got == nil || math.Abs(*got+1000) > 0.01 {
		t.Errorf("SelectedAltitudeFt: got %v, want -1000", got)
	}
}

// TestDecodeDapsRejectsVariableSubfield confirms the robust-decoder contract: a
// present bit for a variable-length subfield the decoder cannot length-skip
// (#9 TID, a repetition field) is rejected, not mis-parsed. Spec 0x81 0x40 =
// #1 (ADR) + FX + #9 (TID).
func TestDecodeDapsRejectsVariableSubfield(t *testing.T) {
	data := []byte{
		0x3E, 0x00, 0x0E, // CAT 62, LEN = 14
		0x81, 0x18, // FSPEC {1, 11, 12}
		0x19, 0x02, // I062/010
		0x81, 0x40, 0x3C, 0x65, 0xAC, // I062/380: spec #1+FX / #9, then ADR
		0x00, 0x2A, // I062/040
	}
	if _, err := DecodeDataBlock(data); err == nil {
		t.Errorf("expected an error for the unsupported variable subfield #9, got nil")
	}
}
