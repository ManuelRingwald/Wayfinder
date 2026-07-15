// Package cat063 decodes ASTERIX CAT063 "Sensor Status Messages" — the
// per-sensor liveness signal (Firefly ICD, ADR 0022; UAP standardised in
// ADR 0032).
//
// CAT063 travels on the same UDP-multicast group as CAT062 and CAT065; a
// receiver dispatches on the leading CAT octet (0x3F → sensor status). Each
// block carries one record per registered sensor, sent wall-clock-periodically
// (default 5 s). Together with CAT065 (global SDPS heartbeat), CAT063 lets
// Wayfinder distinguish a failed sensor from an empty sky — the basis for the
// sensor-degradation banner (yellow).
//
// Since ICD 3.0.0 (Firefly ADR 0032) the record follows the **standard
// EUROCONTROL CAT063 UAP** (SUR.ET1.ST05.2000-STD-04-01, verified against the
// CroatiaControl reference definition ed. 1.3):
//
//   - FRN 1  I063/010 — Data Source Identifier of the **SDPS** (who reports)
//   - FRN 3  I063/030 — Time of Message (1/128 s since UTC midnight)
//   - FRN 4  I063/050 — Sensor Identifier of the **sensor** (what the record is about)
//   - FRN 5  I063/060 — Sensor Configuration and Status (CON, variable via FX)
//
// Firefly emits exactly FRN {1, 3, 4, 5} → FSPEC 0xB8. The decoder additionally
// knows the length rules of the remaining standard items (I063/015 and the
// bias items I063/070–092) and of the Reserved Expansion (RE) / Special Purpose
// (SP) fields, so it can length-skip them for forward compatibility (CLAUDE.md
// §2, tolerant decoder) — in particular the RE field a later ICD adds for a
// per-source failure reason (ADR 0033).
//
// Like the CAT062/CAT065 decoders this one never trusts a datagram: every read
// is bounds-checked and a malformed block is rejected with an error rather than
// panicking (CLAUDE.md §7, robust decoder).
package cat063

import "fmt"

// Category is the ASTERIX category octet for sensor status messages.
const Category = 0x3F // 63

// timeLSBSeconds: I063/030 counts 1/128-second ticks (same as I062/070 and
// I065/030).
const timeLSBSeconds = 1.0 / 128.0

// maxFSPECOctets caps the FX-chained FSPEC length (the CAT063 UAP ends at
// FRN 14 → 2 octets). Beyond this a datagram is hostile or garbled and is
// rejected. The cap also bounds the FRN iteration in decodeRecord to a safe
// range, so an overlong chain can never overflow the loop counter. Wayfinder
// #235 (mirror of Firefly's QW.2 FSPEC-hardening fix).
const maxFSPECOctets = 36

// FRN indices for the standard EUROCONTROL CAT063 UAP (ADR 0032). Firefly emits
// only 1/3/4/5; the others are known so the decoder can length-skip them.
const (
	frnDataSource   = 1  // I063/010, 2 octets: SDPS SAC/SIC
	frnServiceID    = 2  // I063/015, 1 octet
	frnTimeOfDay    = 3  // I063/030, 3 octets: Time of Message
	frnSensorID     = 4  // I063/050, 2 octets: sensor SAC/SIC
	frnSensorStatus = 5  // I063/060, variable (FX): Sensor Configuration & Status
	frnTimeBias     = 6  // I063/070, 2 octets
	frnSSRModeSBias = 7  // I063/080, 4 octets
	frnSSRAzBias    = 8  // I063/081, 2 octets
	frnPSRRangeBias = 9  // I063/090, 4 octets
	frnPSRAzBias    = 10 // I063/091, 2 octets
	frnPSRElevBias  = 11 // I063/092, 2 octets
	// FRN 12 is spare.
	frnReservedExp    = 13 // Reserved Expansion Field (RE), explicit length
	frnSpecialPurpose = 14 // Special Purpose Field (SP), explicit length
)

// fixedItemLen maps the fixed-length standard CAT063 items to their octet
// count, so an unconsumed item can be skipped without desynchronising the
// record parse. I063/060 (FX-variable) and the RE/SP fields (explicit-length)
// are handled separately.
var fixedItemLen = map[uint8]int{
	frnDataSource: 2,
	frnServiceID:  1,
	frnTimeOfDay:  3,
	frnSensorID:   2,
	frnTimeBias:   2,
	// frnSSRModeSBias (I063/080) and frnSSRAzBias (I063/081) are decoded
	// explicitly below (ICD 3.3.0), not length-skipped, so they are absent here.
	frnPSRRangeBias: 4,
	frnPSRAzBias:    2,
	frnPSRElevBias:  2,
}

// conBits is the mask for the CON field of I063/060 (bits 8/7 = 0xC0).
// 0x00 = operational, 0x40 = degraded, 0x80 = initialisation, 0xC0 = not
// connected. Firefly emits only 0x00 (operational) and 0x40 (degraded); a
// consumer treats any non-zero CON as "not operational".
const conBits = 0xC0

// i06360FX is the FX bit (bit 1) of the variable-length I063/060 item.
const i06360FX = 0x01

// Registration-bias LSBs (Firefly REG.3 / ADR 0034, ICD 3.3.0):
//   - I063/080 SRB (slant range bias): LSB = 1/128 NM, expressed here in metres
//     (1 NM = 1852 m) → ≈ 14.469 m per count.
//   - I063/081 SAB (azimuth bias): LSB = 360/2^16 degrees → ≈ 0.00549° per count.
const (
	rangeBiasLSBMetres = 1852.0 / 128.0
	azimuthBiasLSBDeg  = 360.0 / 65536.0
)

// reSrcReasonBit marks the SRC-REASON sub-field present in the I063/RE
// sub-field-spec octet (Firefly ADR 0033).
const reSrcReasonBit = 0x80

// Per-source failure reason codes carried in the I063/RE SRC-REASON sub-field
// (Firefly ADR 0033, ICD 3.1.0). Exposed as strings so they travel unchanged to
// the browser (feed_status.degraded_reason) and drive the feed-health chip.
const (
	// ReasonUnreachable — the source could not be reached (network/firewall/DNS/
	// timeout). Credentials, if any, are fine.
	ReasonUnreachable = "unreachable"
	// ReasonAuth — authentication/authorisation failed (HTTP 401/403).
	ReasonAuth = "auth"
	// ReasonRateLimited — the source rate-limited the request (HTTP 429).
	ReasonRateLimited = "rate_limited"
)

// reasonFromCode maps an I063/RE SRC-REASON octet to a reason string. Code 0
// ("ok") is never sent on the wire; an unknown non-zero code is treated as
// "degraded, reason unknown" ("") rather than guessed — forward tolerance
// (Firefly ICD §9).
func reasonFromCode(code byte) string {
	switch code {
	case 1:
		return ReasonUnreachable
	case 2:
		return ReasonAuth
	case 3:
		return ReasonRateLimited
	default:
		return ""
	}
}

// reasonPriority ranks reasons for the feed-level summary: the most
// operator-actionable reason wins when a feed's degraded sensors disagree. A
// wrong credential (auth) is the most directly fixable, then a rate limit, then
// an unreachable network. Unknown/absent reasons rank lowest.
func reasonPriority(reason string) int {
	switch reason {
	case ReasonAuth:
		return 3
	case ReasonRateLimited:
		return 2
	case ReasonUnreachable:
		return 1
	default:
		return 0
	}
}

// DominantReason returns the single most operator-actionable failure reason
// among the degraded sensors in a block, or "" if none carry one. It collapses
// a multi-sensor CAT063 block into the one reason the feed-health chip shows
// (Firefly ADR 0033 / Wayfinder ADR 0020). Only degraded (non-operational)
// sensors contribute; an operational sensor never carries a reason.
func DominantReason(statuses []SensorStatus) string {
	best := ""
	for _, s := range statuses {
		if s.Operational || s.Reason == "" {
			continue
		}
		if reasonPriority(s.Reason) > reasonPriority(best) {
			best = s.Reason
		}
	}
	return best
}

// DecodeError is a CAT063 parsing error.
type DecodeError struct{ msg string }

func (e DecodeError) Error() string { return "CAT063 decode error: " + e.msg }

func newErr(format string, args ...any) DecodeError {
	return DecodeError{msg: fmt.Sprintf(format, args...)}
}

// SensorStatus is one decoded CAT063 sensor status record.
type SensorStatus struct {
	// I063/050 — data source identifier of the sensor this record is about.
	// (Before ICD 3.0.0 the sensor identity travelled in I063/010; ADR 0032
	// moved it to I063/050 and gave I063/010 the SDPS identity.)
	SAC uint8
	SIC uint8
	// I063/010 — data source identifier of the reporting SDPS (the same SAC/SIC
	// as CAT062 I062/010 and CAT065 I065/010). Retained for traceability; the
	// feed-health logic keys on the sensor identity above.
	SDPSSAC uint8
	SDPSSIC uint8
	// I063/030 — time of day, seconds since UTC midnight (wraps every 24 h).
	TimeOfDay float64
	// I063/060 CON — true when the sensor reports itself operational (CON = 00).
	Operational bool
	// I063/RE SRC-REASON — the per-source failure reason for a degraded sensor
	// (Firefly ADR 0033): "unreachable", "auth", "rate_limited", or "" when no RE
	// field is present (operational, or reason unknown). One of the Reason*
	// constants.
	Reason string
	// RangeBiasM is the applied slant-range registration correction in metres
	// (I063/080 SRB, Firefly REG.3 / ADR 0034, ICD 3.3.0). AzimuthBiasDeg is the
	// applied azimuth correction in degrees (I063/081 SAB). Both are nil when the
	// sensor carries no active registration correction — absence means "no
	// correction", never a bias of 0. A positive range bias means the sensor
	// measures too far; a positive azimuth bias is clockwise.
	RangeBiasM     *float64
	AzimuthBiasDeg *float64
}

// DecodeSensorBlock parses a CAT063 data block: [CAT=0x3F][LEN: u16 BE][Record...].
func DecodeSensorBlock(data []byte) ([]SensorStatus, error) {
	if len(data) < 3 {
		return nil, newErr("data block too short")
	}
	if data[0] != Category {
		return nil, newErr("invalid CAT: 0x%02x (expected 0x3F)", data[0])
	}
	lenVal := (int(data[1]) << 8) | int(data[2])
	if lenVal < 3 {
		return nil, newErr("invalid LEN (too small)")
	}
	if lenVal > len(data) {
		return nil, newErr("LEN exceeds data length")
	}

	var statuses []SensorStatus
	offset := 3
	for offset < lenVal {
		s, next, err := decodeRecord(data, offset, lenVal)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, s)
		offset = next
	}
	return statuses, nil
}

// decodeRecord parses one sensor status record starting at offset, bounded by
// end (the block's declared length). Returns the status and the offset just
// past it.
func decodeRecord(data []byte, offset, end int) (SensorStatus, int, error) {
	var s SensorStatus

	// Parse the FSPEC: octets up to and including the first with FX (bit 0) clear.
	// A crafted datagram could chain FX forever; cap the length so the parse can
	// neither read nor (below) iterate an unbounded FSPEC (Wayfinder #235).
	fspecStart := offset
	for {
		if offset >= end {
			return s, offset, newErr("truncated FSPEC")
		}
		fx := data[offset]&0x01 != 0
		offset++
		if !fx {
			break
		}
		if offset-fspecStart >= maxFSPECOctets {
			return s, offset, newErr("FSPEC exceeds maximum length (%d octets)", maxFSPECOctets)
		}
	}
	fspec := data[fspecStart:offset]

	// take advances offset by n octets within the record bounds.
	take := func(n int) ([]byte, error) {
		if n < 0 || end-offset < n {
			return nil, newErr("truncated item (need %d octets)", n)
		}
		b := data[offset : offset+n]
		offset += n
		return b, nil
	}

	// Iterate FRNs as an int, not a uint8: the highest FRN a capped FSPEC can
	// address is maxFSPECOctets*7 = 252, which fits a uint8 — but only just.
	// Using int removes any chance of the counter wrapping past 255 and looping
	// forever on a crafted (over-long) FSPEC (Wayfinder #235).
	var haveSDPS, haveSensor, haveTime, haveStatus bool
	maxFRN := len(fspec) * 7
	for frn := 1; frn <= maxFRN; frn++ {
		if !fspecHas(fspec, frn) {
			continue
		}
		switch frn {
		case frnDataSource:
			b, err := take(2)
			if err != nil {
				return s, offset, err
			}
			s.SDPSSAC, s.SDPSSIC, haveSDPS = b[0], b[1], true
		case frnTimeOfDay:
			b, err := take(3)
			if err != nil {
				return s, offset, err
			}
			ticks := (uint32(b[0]) << 16) | (uint32(b[1]) << 8) | uint32(b[2])
			s.TimeOfDay = float64(ticks) * timeLSBSeconds
			haveTime = true
		case frnSensorID:
			b, err := take(2)
			if err != nil {
				return s, offset, err
			}
			s.SAC, s.SIC, haveSensor = b[0], b[1], true
		case frnSensorStatus:
			// I063/060 is variable-length (FX): the first octet carries the CON
			// field; further octets follow while the FX bit stays set. Read the
			// first, then skip any extensions so an extended status item never
			// desynchronises the record parse.
			b, err := take(1)
			if err != nil {
				return s, offset, err
			}
			s.Operational, haveStatus = b[0]&conBits == 0, true
			octet := b[0]
			for octet&i06360FX != 0 {
				ext, err := take(1)
				if err != nil {
					return s, offset, err
				}
				octet = ext[0]
			}
		case frnReservedExp:
			// I063/RE Reserved Expansion Field: [LEN][SUBFIELD_SPEC][SRC-REASON…].
			// LEN counts itself. Parse the SRC-REASON sub-field (Firefly ADR 0033)
			// so the feed-health chip can show WHY a source is degraded; anything
			// past it is skipped by length. RE is self-delimiting, so an unknown
			// trailing sub-field never desynchronises the parse.
			lb, err := take(1)
			if err != nil {
				return s, offset, err
			}
			fieldLen := int(lb[0])
			if fieldLen < 1 {
				return s, offset, newErr("RE field length is zero")
			}
			body, err := take(fieldLen - 1)
			if err != nil {
				return s, offset, err
			}
			// body[0] = sub-field spec; body[1] = SRC-REASON when its bit is set.
			if len(body) >= 2 && body[0]&reSrcReasonBit != 0 {
				s.Reason = reasonFromCode(body[1])
			}
		case frnSpecialPurpose:
			// SP (Special Purpose) is an explicit-length field the decoder does not
			// interpret: skip it length-aware (first octet = length incl. itself).
			lb, err := take(1)
			if err != nil {
				return s, offset, err
			}
			fieldLen := int(lb[0])
			if fieldLen < 1 {
				return s, offset, newErr("SP field length is zero")
			}
			if _, err := take(fieldLen - 1); err != nil {
				return s, offset, err
			}
		case frnSSRModeSBias:
			// I063/080 (4 octets): SRG Range Gain (i16, ignored — Firefly always
			// sends 0) + SRB slant Range Bias (i16, LSB 1/128 NM → metres). Present
			// only when a registration correction is applied to this sensor.
			b, err := take(4)
			if err != nil {
				return s, offset, err
			}
			srb := float64(int16(uint16(b[2])<<8|uint16(b[3]))) * rangeBiasLSBMetres
			s.RangeBiasM = &srb
		case frnSSRAzBias:
			// I063/081 (2 octets): SAB Azimuth Bias (i16, LSB 360/2^16 degrees).
			b, err := take(2)
			if err != nil {
				return s, offset, err
			}
			sab := float64(int16(uint16(b[0])<<8|uint16(b[1]))) * azimuthBiasLSBDeg
			s.AzimuthBiasDeg = &sab
		default:
			// A remaining standard item with a known fixed length is skipped;
			// anything else cannot be length-skipped safely, so reject rather
			// than mis-parse (robust decoder). Firefly emits only FRN {1,3,4,5}.
			n, ok := fixedItemLen[uint8(frn)]
			if !ok {
				return s, offset, newErr("unknown FRN %d present", frn)
			}
			if _, err := take(n); err != nil {
				return s, offset, err
			}
		}
	}

	if !haveSDPS || !haveSensor || !haveTime || !haveStatus {
		return s, offset, newErr("sensor status record missing a required item")
	}
	return s, offset, nil
}

// fspecHas reports whether the FSPEC marks the given FRN present.
func fspecHas(fspec []byte, frn int) bool {
	octet := (frn - 1) / 7
	if octet >= len(fspec) {
		return false
	}
	bit := 7 - ((frn - 1) % 7)
	return fspec[octet]&(1<<bit) != 0
}
