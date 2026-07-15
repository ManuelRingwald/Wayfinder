package cat062

import "testing"

// TestDecodeMonAndSpi decodes a record whose I062/080 carries the MON (0x80) and
// SPI (0x40) flags in octet 1, per ICD v3.2.0 (Firefly QW.3). The reference
// vector's status octet (0x00) is set to 0xC0 = MON|SPI (CNF bit clear →
// confirmed, FX clear → single octet). Mirrors Firefly's encoder test
// track_status_carries_mon_and_spi_in_octet_one. Both flags must be recovered
// while the rest of the record still decodes.
func TestDecodeMonAndSpi(t *testing.T) {
	data := []byte{
		0x3E, 0x00, 0x28, // CAT 62, LEN = 40
		0x9F, 0x0F, 0x01, 0x04, // FSPEC {1, 4, 5, 6, 7, 12, 13, 14, 27}
		0x19, 0x02, // I062/010
		0x00, 0x06, 0x00, // I062/070
		0x00, 0x80, 0x00, 0x00, // I062/105 lat
		0x00, 0x20, 0x00, 0x00, // I062/105 lon
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100
		0x01, 0x90, 0xFF, 0x38, // I062/185
		0x00, 0x01, // I062/040
		0xC0,       // I062/080 = MON | SPI, confirmed, single octet
		0x40, 0x08, // I062/290
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500
	}
	tracks, err := DecodeDataBlock(data)
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	track := tracks[0]
	if !track.Status.Monosensor {
		t.Errorf("Monosensor mismatch: expected true (MON set), got false")
	}
	if !track.Status.SPI {
		t.Errorf("SPI mismatch: expected true (SPI set), got false")
	}
	// CNF bit clear → confirmed; MON/SPI must not disturb the other flags.
	if !track.Status.Confirmed {
		t.Errorf("Confirmed mismatch: expected true, got false")
	}
	if track.Status.Coasting || track.Status.Ended {
		t.Errorf("Coasting/Ended mismatch: expected both false, got %v/%v",
			track.Status.Coasting, track.Status.Ended)
	}
	// Items after I062/080 must still decode correctly around the flags.
	if track.TrackNum != 1 {
		t.Errorf("TrackNum mismatch: expected 1, got %d", track.TrackNum)
	}
	if track.Accuracy.APC < 99.99 || track.Accuracy.APC > 100.01 {
		t.Errorf("Accuracy.APC mismatch: expected ≈100.0, got %v", track.Accuracy.APC)
	}
}

// TestDecodeNoMonNoSpi confirms the plain reference vector (status octet 0x00)
// leaves both flags false — no spurious mono/ident marker on an ordinary track.
func TestDecodeNoMonNoSpi(t *testing.T) {
	tracks, err := DecodeDataBlock(referenceTrackBlock())
	if err != nil {
		t.Fatalf("DecodeDataBlock failed: %v", err)
	}
	if tracks[0].Status.Monosensor || tracks[0].Status.SPI {
		t.Errorf("expected MON and SPI both false for the plain reference, got mono=%v spi=%v",
			tracks[0].Status.Monosensor, tracks[0].Status.SPI)
	}
}
