package cat065

import "testing"

// referenceHeartbeat is the byte-exact CAT065 SDPS-status block Firefly emits
// (firefly-asterix cat065::status_matches_reference_dump): service id 1,
// midnight, operational. This is the cross-project ground truth.
func referenceHeartbeat() []byte {
	return []byte{0x41, 0x00, 0x0C, 0xF4, 0x19, 0x02, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00}
}

// TestDecodeReferenceHeartbeat verifies the decoder against Firefly's exact bytes.
func TestDecodeReferenceHeartbeat(t *testing.T) {
	reports, err := DecodeStatusBlock(referenceHeartbeat())
	if err != nil {
		t.Fatalf("DecodeStatusBlock: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	s := reports[0]
	if s.SAC != 0x19 || s.SIC != 0x02 {
		t.Errorf("source: got %02x/%02x, want 19/02", s.SAC, s.SIC)
	}
	if s.MessageType != MessageTypeSDPSStatus {
		t.Errorf("message type: got %d, want %d", s.MessageType, MessageTypeSDPSStatus)
	}
	if s.ServiceID != 1 {
		t.Errorf("service id: got %d, want 1", s.ServiceID)
	}
	if s.TimeOfDay != 0 {
		t.Errorf("time of day: got %f, want 0", s.TimeOfDay)
	}
	if !s.Operational {
		t.Errorf("operational: got false, want true")
	}
}

// TestDecodeTimeOfDay checks the 1/128-s scaling of I065/030.
func TestDecodeTimeOfDay(t *testing.T) {
	// 01:00:00 = 3600 s → 460800 = 0x070800.
	block := []byte{0x41, 0x00, 0x0C, 0xF4, 0x19, 0x02, 0x01, 0x01, 0x07, 0x08, 0x00, 0x00}
	reports, err := DecodeStatusBlock(block)
	if err != nil {
		t.Fatalf("DecodeStatusBlock: %v", err)
	}
	if got := reports[0].TimeOfDay; got < 3599.99 || got > 3600.01 {
		t.Errorf("time of day: got %f, want ~3600", got)
	}
}

// TestDecodeDegradedStatus checks the NOGO field of I065/040.
func TestDecodeDegradedStatus(t *testing.T) {
	block := referenceHeartbeat()
	block[len(block)-1] = 0x40 // NOGO = 01 (degraded)
	reports, err := DecodeStatusBlock(block)
	if err != nil {
		t.Fatalf("DecodeStatusBlock: %v", err)
	}
	if reports[0].Operational {
		t.Errorf("operational: got true, want false for NOGO=01")
	}
}

// TestDecodeRejectsWrongCategory ensures a CAT062 block is not misread.
func TestDecodeRejectsWrongCategory(t *testing.T) {
	block := referenceHeartbeat()
	block[0] = 0x3E
	if _, err := DecodeStatusBlock(block); err == nil {
		t.Errorf("expected error for wrong category, got nil")
	}
}

// TestDecodeRejectsTruncatedInput ensures no panic and an error on short input.
func TestDecodeRejectsTruncatedInput(t *testing.T) {
	full := referenceHeartbeat()
	for cut := 0; cut < len(full); cut++ {
		// Must not panic; a truncated block should error (or, for a prefix that
		// happens to be a self-consistent shorter LEN, decode — but our LEN is
		// fixed at 12, so any cut < 12 errors).
		if _, err := DecodeStatusBlock(full[:cut]); err == nil && cut < len(full) {
			t.Errorf("cut=%d: expected error, got nil", cut)
		}
	}
}
