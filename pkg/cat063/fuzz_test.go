package cat063

import (
	"testing"
	"time"
)

// FuzzDecodeSensorBlock exercises the CAT063 sensor-status decoder against
// arbitrary bytes from the unauthenticated multicast trust boundary
// (CLAUDE.md §7). The property is absolute: no input may panic or hang — every
// datagram must yield (statuses, nil) or (nil, error). Seeded with the byte-exact
// reference vectors plus a hostile over-long FSPEC. Satisfies the charter's
// "Fuzzing des Parsers vorsehen" (§7).
func FuzzDecodeSensorBlock(f *testing.F) {
	for _, seed := range [][]byte{
		referenceSingleSensor(),
		referenceTwoSensors(),
		referenceDegradedWithReason(0x02),
		overlongFSPECSensorBlock(),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = DecodeSensorBlock(data)
	})
}

// overlongFSPECSensorBlock builds a CAT063 block whose record carries a would-be
// 37-octet FSPEC (36 FX-set octets + one FX-clear). Before the octet cap this
// drove the uint8 FRN counter past 255 and wrapped it, spinning decodeRecord
// forever (Wayfinder #235). The decoder must now reject it — and, above all,
// return.
func overlongFSPECSensorBlock() []byte {
	rec := make([]byte, 0, 37)
	for i := 0; i < 36; i++ {
		rec = append(rec, 0x01) // FX set, no FRN bits
	}
	rec = append(rec, 0x00) // FX clear: FSPEC would be 37 octets
	block := []byte{0x3F, 0x00, byte(3 + len(rec))}
	return append(block, rec...)
}

// TestOverlongFSPECReturns is the direct regression for the FSPEC FX-chain
// infinite loop: the decode must finish (not loop) and reject the crafted block.
func TestOverlongFSPECReturns(t *testing.T) {
	block := overlongFSPECSensorBlock()
	done := make(chan error, 1)
	go func() { _, err := DecodeSensorBlock(block); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected an error for an over-long FSPEC, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("decode did not return within 2s — FSPEC FX-chain infinite loop (regression of #235)")
	}
}
