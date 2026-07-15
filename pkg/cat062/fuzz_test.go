package cat062

import (
	"testing"
	"time"
)

// FuzzDecodeDataBlock exercises the CAT062 track decoder against arbitrary bytes
// from the unauthenticated multicast trust boundary (CLAUDE.md §7). The property
// is absolute: no input may panic or hang — every datagram must yield
// (tracks, nil) or (nil, error). Seeded with byte-exact valid blocks plus a
// hostile over-long FSPEC so the fuzzer starts from both structurally valid and
// adversarial inputs and mutates outward. Satisfies the charter's "Fuzzing des
// Parsers vorsehen" (§7); mirrors Firefly's QW.2 fuzzing (NFR-SAFE-002 there).
func FuzzDecodeDataBlock(f *testing.F) {
	for _, seed := range [][]byte{
		{0x3E, 0x00, 0x06, 0x80, 0x19, 0x02}, // minimal record: I062/010 only
		referenceTrackBlock(),                // full single-track reference vector
		overlongFSPECDataBlock(),             // hostile: over-long FX chain
	} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		// The bounds-checked decoder must never panic (fuzzing catches that) and
		// must always return — an unbounded FSPEC used to be the one way it could
		// spin. We ignore the result; we only assert it comes back at all.
		_, _ = DecodeDataBlock(data)
	})
}

// referenceTrackBlock is the byte-exact CAT062 single-track dump from Firefly's
// encoder test single_track_matches_reference_dump — the cross-project ground
// truth (see TestReferenceVector). Used as a fuzz seed.
func referenceTrackBlock() []byte {
	return []byte{
		0x3E, 0x00, 0x28, // CAT 62, LEN = 40
		0x9F, 0x0F, 0x01, 0x04, // FSPEC {1, 4, 5, 6, 7, 12, 13, 14, 27}
		0x19, 0x02, // I062/010
		0x00, 0x06, 0x00, // I062/070
		0x00, 0x80, 0x00, 0x00, // I062/105 lat
		0x00, 0x20, 0x00, 0x00, // I062/105 lon
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // I062/100
		0x01, 0x90, 0xFF, 0x38, // I062/185
		0x00, 0x01, // I062/040
		0x00,       // I062/080
		0x40, 0x08, // I062/290
		0x80, 0x00, 0xC8, 0x00, 0xC8, // I062/500
	}
}

// overlongFSPECDataBlock builds a CAT062 block whose record carries a would-be
// 37-octet FSPEC (36 FX-set octets + one FX-clear). The octet cap must reject it
// rather than read an unbounded chain (Wayfinder #235).
func overlongFSPECDataBlock() []byte {
	rec := make([]byte, 0, 37)
	for i := 0; i < 36; i++ {
		rec = append(rec, 0x01) // FX set, no FRN bits
	}
	rec = append(rec, 0x00) // FX clear: FSPEC would be 37 octets
	block := []byte{0x3E, 0x00, byte(3 + len(rec))}
	return append(block, rec...)
}

// TestOverlongFSPECIsRejected is the direct regression for the FSPEC FX-chain:
// the decode must finish (not loop) and reject the crafted block.
func TestOverlongFSPECIsRejected(t *testing.T) {
	block := overlongFSPECDataBlock()
	done := make(chan error, 1)
	go func() { _, err := DecodeDataBlock(block); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected an error for an over-long FSPEC, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("decode did not return within 2s — FSPEC FX-chain runaway (regression of #235)")
	}
}
