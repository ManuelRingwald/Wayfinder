package cat063

import (
	"math"
	"testing"
)

// referenceSensorWithBias returns the byte-exact CAT063 block Firefly emits for
// one operational sensor (SIC=1) carrying an applied registration correction of
// +150 m range / +0.30° azimuth — ICD 3.3.0 §9, Firefly encoder test
// sensor_with_bias_matches_reference_dump. FSPEC 0xBB 0x80 = FRN {1,3,4,5,7,8};
// I063/080 SRG=0 / SRB=10 (0x000A), I063/081 SAB=55 (0x0037).
func referenceSensorWithBias() []byte {
	return []byte{
		0x3F, 0x00, 0x13,
		0xBB, 0x80, // FSPEC: FRN 1,3,4,5 + FX → 7,8
		0x19, 0x02, // I063/010 SDPS 25/2
		0x00, 0x00, 0x00, // I063/030 time=0
		0x00, 0x01, // I063/050 sensor 0/1
		0x00,                   // I063/060 CON operational
		0x00, 0x00, 0x00, 0x0A, // I063/080 SRG=0, SRB=10
		0x00, 0x37, // I063/081 SAB=55
	}
}

// TestDecodeRegistrationBias verifies the I063/080 SRB (range) and I063/081 SAB
// (azimuth) decode against Firefly's 3.3.0 reference dump: 10 counts → ~144.7 m,
// 55 counts → ~0.302°.
func TestDecodeRegistrationBias(t *testing.T) {
	statuses, err := DecodeSensorBlock(referenceSensorWithBias())
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.SAC != 0 || s.SIC != 1 {
		t.Errorf("sensor: got %02x/%02x, want 00/01", s.SAC, s.SIC)
	}
	if s.RangeBiasM == nil {
		t.Fatalf("RangeBiasM: expected ~144.7 m, got nil")
	}
	if math.Abs(*s.RangeBiasM-144.6875) > 0.01 {
		t.Errorf("RangeBiasM: got %v, want ~144.69 m (10 × 1852/128)", *s.RangeBiasM)
	}
	if s.AzimuthBiasDeg == nil {
		t.Fatalf("AzimuthBiasDeg: expected ~0.302°, got nil")
	}
	if math.Abs(*s.AzimuthBiasDeg-0.302124) > 0.0001 {
		t.Errorf("AzimuthBiasDeg: got %v, want ~0.302° (55 × 360/65536)", *s.AzimuthBiasDeg)
	}
}

// TestDecodeNegativeBias confirms both bias items are signed (i16): a sensor
// that measures too near / counter-clockwise reports negative corrections.
func TestDecodeNegativeBias(t *testing.T) {
	block := referenceSensorWithBias()
	// SRB = -10 (0xFFF6) at offset 15-16; SAB = -55 (0xFFC9) at offset 17-18.
	block[15], block[16] = 0xFF, 0xF6
	block[17], block[18] = 0xFF, 0xC9
	statuses, err := DecodeSensorBlock(block)
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	s := statuses[0]
	if s.RangeBiasM == nil || *s.RangeBiasM > 0 {
		t.Errorf("RangeBiasM: expected negative, got %v", s.RangeBiasM)
	}
	if math.Abs(*s.RangeBiasM+144.6875) > 0.01 {
		t.Errorf("RangeBiasM: got %v, want ~-144.69 m", *s.RangeBiasM)
	}
	if s.AzimuthBiasDeg == nil || *s.AzimuthBiasDeg > 0 {
		t.Errorf("AzimuthBiasDeg: expected negative, got %v", s.AzimuthBiasDeg)
	}
}

// TestDecodeNoBias confirms a plain sensor block (no FRN 7/8) leaves both bias
// fields nil — absence means "no correction", never a bias of 0.
func TestDecodeNoBias(t *testing.T) {
	statuses, err := DecodeSensorBlock(referenceSingleSensor())
	if err != nil {
		t.Fatalf("DecodeSensorBlock: %v", err)
	}
	s := statuses[0]
	if s.RangeBiasM != nil || s.AzimuthBiasDeg != nil {
		t.Errorf("expected no bias for a plain sensor, got range=%v az=%v",
			s.RangeBiasM, s.AzimuthBiasDeg)
	}
}
