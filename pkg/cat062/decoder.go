package cat062

import (
	"fmt"
	"strings"
)

// DecodeDataBlock parses a CAT062 data block.
// Format: [CAT=0x3E] [LEN: u16 BE] [Record]...
func DecodeDataBlock(data []byte) ([]DecodedTrack, error) {
	if len(data) < 3 {
		return nil, NewDecodeError("data block too short")
	}

	// Check CAT
	if data[0] != 0x3E {
		return nil, NewDecodeError(fmt.Sprintf("invalid CAT: 0x%02x (expected 0x3E)", data[0]))
	}

	// Parse LEN (big-endian u16, includes the 3-byte header)
	lenVal := (int(data[1]) << 8) | int(data[2])
	if lenVal < 3 {
		return nil, NewDecodeError("invalid LEN (too small)")
	}
	if lenVal > len(data) {
		return nil, NewDecodeError("LEN exceeds data length")
	}

	// Parse records from byte 3 onwards until LEN is exhausted.
	var tracks []DecodedTrack
	offset := 3

	for offset < lenVal {
		track, newOffset, err := DecodeRecord(data, offset)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
		offset = newOffset
	}

	return tracks, nil
}

// DecodeRecord parses one ASTERIX CAT062 record starting at offset.
// Returns the decoded track and the offset after this record.
func DecodeRecord(data []byte, offset int) (DecodedTrack, int, error) {
	track := DecodedTrack{}

	// Parse FSPEC
	fspec, offset, err := NewFSPEC(data, offset)
	if err != nil {
		return track, offset, err
	}

	// Decode items in standard EUROCONTROL UAP order. FRNs follow the real
	// CAT062 UAP (ICD v2.0.0): I062/136 (Measured Flight Level) at FRN 17,
	// I062/500 (Estimated Accuracies) at FRN 27 (not the old non-standard 16).
	// I062/245 (Target Identification / Callsign, ICD v2.1.0) sits at FRN 10.
	uapOrder := []uint8{1, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 17, 18, 19, 20, 27}

	for _, frn := range uapOrder {
		if !fspec.HasItem(frn) {
			continue
		}

		switch frn {
		case 1: // I062/010: Data Source ID
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/010")
			}
			track.Source = DataSourceID{
				SAC: data[offset],
				SIC: data[offset+1],
			}
			offset += 2

		case 4: // I062/070: Time of Day (3 bytes, 1/128 s)
			if offset+3 > len(data) {
				return track, offset, NewDecodeError("truncated I062/070")
			}
			// u24 big-endian
			ticks := (uint32(data[offset]) << 16) | (uint32(data[offset+1]) << 8) | uint32(data[offset+2])
			track.TimeOfDay.Seconds = float64(ticks) / 128.0
			offset += 3

		case 5: // I062/105: WGS84 Position (8 bytes: lat i32, lon i32)
			if offset+8 > len(data) {
				return track, offset, NewDecodeError("truncated I062/105")
			}
			latTicks := int32((uint32(data[offset]) << 24) | (uint32(data[offset+1]) << 16) |
				(uint32(data[offset+2]) << 8) | uint32(data[offset+3]))
			lonTicks := int32((uint32(data[offset+4]) << 24) | (uint32(data[offset+5]) << 16) |
				(uint32(data[offset+6]) << 8) | uint32(data[offset+7]))

			const posLSB = 180.0 / (1 << 25)
			track.WGS84.Latitude = float64(latTicks) * posLSB
			track.WGS84.Longitude = float64(lonTicks) * posLSB
			offset += 8

		case 6: // I062/100: System Cartesian Position (6 bytes: X i24, Y i24)
			if offset+6 > len(data) {
				return track, offset, NewDecodeError("truncated I062/100")
			}

			// i24 big-endian (3 bytes each, signed)
			// Extract as 32-bit value, then shift left to align sign bit at 31, then arithmetic shift right
			xVal := signExtendI24((int32(data[offset]) << 16) | (int32(data[offset+1]) << 8) | int32(data[offset+2]))
			yVal := signExtendI24((int32(data[offset+3]) << 16) | (int32(data[offset+4]) << 8) | int32(data[offset+5]))

			const cartLSB = 0.5
			track.Cartesian.X = float64(xVal) * cartLSB
			track.Cartesian.Y = float64(yVal) * cartLSB
			offset += 6

		case 7: // I062/185: Velocity (4 bytes: Vx i16, Vy i16)
			if offset+4 > len(data) {
				return track, offset, NewDecodeError("truncated I062/185")
			}
			vxTicks := int16((uint16(data[offset]) << 8) | uint16(data[offset+1]))
			vyTicks := int16((uint16(data[offset+2]) << 8) | uint16(data[offset+3]))

			const velLSB = 0.25
			track.Velocity.Vx = float64(vxTicks) * velLSB
			track.Velocity.Vy = float64(vyTicks) * velLSB
			offset += 4

		case 8: // I062/210: Calculated Acceleration (2 bytes, Ax then Ay, each i8, LSB 0.25 m/s²)
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/210")
			}
			const accLSB = 0.25 // m/s² per tick; Ax = East, Ay = North (ICD §4.9)
			ax := float64(int8(data[offset])) * accLSB
			ay := float64(int8(data[offset+1])) * accLSB
			track.AccelAxMS2 = &ax
			track.AccelAyMS2 = &ay
			offset += 2

		case 9: // I062/060: Mode 3/A Code (2 bytes, 12-bit code in low bits)
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/060")
			}
			code := uint16((uint16(data[offset]) << 8) | uint16(data[offset+1]))
			code &= 0x0FFF // low 12 bits
			track.Mode3A = &code
			offset += 2

		case 10: // I062/245: Target Identification (Callsign, 7 bytes)
			if offset+7 > len(data) {
				return track, offset, NewDecodeError("truncated I062/245")
			}
			callsign := decodeTargetIdentification(data[offset : offset+7])
			track.Callsign = &callsign
			offset += 7

		case 11: // I062/380: Aircraft Derived Data (compound, subfield-driven, ICD 3.4.0)
			newOffset, err := decodeAircraftDerivedData(data, offset, &track)
			if err != nil {
				return track, offset, err
			}
			offset = newOffset

		case 12: // I062/040: Track Number (2 bytes, u16 BE)
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/040")
			}
			track.TrackNum = (uint16(data[offset]) << 8) | uint16(data[offset+1])
			offset += 2

		case 13: // I062/080: Track Status (variable, with FX chaining)
			status, newOffset, err := decodeTrackStatus(data, offset)
			if err != nil {
				return track, offset, err
			}
			track.Status = status
			offset = newOffset

		case 14: // I062/290: Update Ages (compound: per-technology ages, ICD 2.6.0)
			if offset+1 > len(data) {
				return track, offset, NewDecodeError("truncated I062/290 primary")
			}
			primary := data[offset]
			offset++

			// Each set bit in the primary subfield (MSB→LSB) is followed by one
			// 1-byte age value (LSB = 1/4 s). We walk the bits in order and pick
			// out the five Firefly emits — PSR (0x40, always), SSR (0x20), MDS
			// (0x10), ES (0x08, Extended Squitter / ADS-B, ICD 2.4.0) and FLARM
			// (0x04, Firefly vendor subfield; ICD 2.6.0, ADR 0027) — while
			// consuming and skipping any others. Doing it positionally (rather
			// than reading ages at fixed offsets) keeps the decoder correct if
			// Firefly ever inserts a subfield between them — the tolerant
			// decoder the charter requires (Abschnitt 2/7). Bit 0 (0x01) is FX;
			// Firefly never sets it, so a second primary octet is not expected
			// here.
			for bit := 7; bit >= 1; bit-- {
				mask := byte(1) << uint(bit)
				if (primary & mask) == 0 {
					continue
				}
				if offset+1 > len(data) {
					return track, offset, NewDecodeError("truncated I062/290 subfield")
				}
				ageSeconds := float64(data[offset]) * 0.25 // LSB = 1/4 s
				offset++
				switch mask {
				case 0x40: // PSR age
					track.UpdateAge.PSRAge = ageSeconds
				case 0x20: // SSR (Mode A/C) age, ICD 2.6.0
					v := ageSeconds
					track.UpdateAge.SSRAge = &v
				case 0x10: // MDS (Mode S) age, ICD 2.6.0
					v := ageSeconds
					track.UpdateAge.MDSAge = &v
				case 0x08: // ES age (Extended Squitter / ADS-B), ICD 2.4.0
					es := ageSeconds
					track.UpdateAge.ESAge = &es
				case 0x04: // FLARM age (Firefly vendor subfield), ICD 2.6.0
					v := ageSeconds
					track.UpdateAge.FLARMAge = &v
				}
			}

		case 15: // I062/200: Mode of Movement (1 byte, three 2-bit axes + ADF/spare)
			if offset+1 > len(data) {
				return track, offset, NewDecodeError("truncated I062/200")
			}
			b := data[offset]
			// Bits 8-7 = TRANS (course), 6-5 = LONG (speed), 4-3 = VERT; each
			// 2-bit field's value 3 = "undetermined", left as a nil pointer.
			if c := courseTrend((b >> 6) & 0x03); c != nil {
				track.MotionCourse = c
			}
			if s := speedTrend((b >> 4) & 0x03); s != nil {
				track.MotionSpeed = s
			}
			if v := verticalTrend((b >> 2) & 0x03); v != nil {
				track.MotionVertical = v
			}
			offset++

		case 17: // I062/136: Measured Flight Level (2 bytes, signed i16, LSB 1/4 FL = 25 ft)
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/136")
			}
			flTicks := int16((uint16(data[offset]) << 8) | uint16(data[offset+1]))
			fl := float64(flTicks) * 25.0 // LSB = 1/4 FL = 25 ft
			track.FlightLevelFt = &fl
			offset += 2

		case 18: // I062/130: Calculated Track Geometric Altitude (2 bytes, i16, LSB 6.25 ft)
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/130")
			}
			ticks := int16((uint16(data[offset]) << 8) | uint16(data[offset+1]))
			alt := float64(ticks) * 6.25 // LSB = 6.25 ft, WGS-84 geometric
			track.GeometricAltitudeFt = &alt
			offset += 2

		case 19: // I062/135: Calculated Track Barometric Altitude (2 bytes)
			// Bit 16 (0x8000) is the QNH-correction flag; bits 15..1 are a 15-bit
			// two's-complement altitude in 1/4-FL (25 ft) steps. This is NOT a plain
			// i16: the sign bit is bit 15 (0x4000 within the masked value), so the
			// value is sign-extended from 15 bits, not 16.
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/135")
			}
			raw := (uint16(data[offset]) << 8) | uint16(data[offset+1])
			track.BaroQNHCorrected = (raw & 0x8000) != 0
			v := int(raw & 0x7FFF)
			if v&0x4000 != 0 { // sign bit of the 15-bit field
				v -= 0x8000
			}
			baro := float64(v) * 25.0 // LSB = 1/4 FL = 25 ft
			track.BarometricAltitudeFt = &baro
			offset += 2

		case 20: // I062/220: Calculated Rate of Climb/Descent (2 bytes, i16, LSB 6.25 ft/min)
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/220")
			}
			ticks := int16((uint16(data[offset]) << 8) | uint16(data[offset+1]))
			rocd := float64(ticks) * 6.25 // LSB = 6.25 ft/min, positive = climb
			track.RateOfClimbDescentFtMin = &rocd
			offset += 2

		case 27: // I062/500: Estimated Accuracies (compound, currently just APC)
			if offset+1 > len(data) {
				return track, offset, NewDecodeError("truncated I062/500 primary")
			}
			primary := data[offset]
			offset++

			// Check APC bit (bit 7, 0x80)
			if (primary & 0x80) != 0 {
				if offset+4 > len(data) {
					return track, offset, NewDecodeError("truncated I062/500 APC")
				}
				// APC: X and Y components (u16 BE each), LSB = 0.5 m
				xTicks := (uint16(data[offset]) << 8) | uint16(data[offset+1])
				yTicks := (uint16(data[offset+2]) << 8) | uint16(data[offset+3])
				track.Accuracy.APC = float64((xTicks+yTicks)/2) * 0.5 // Average for simplicity
				offset += 4
			}
		}
	}

	return track, offset, nil
}

// courseTrend / speedTrend / verticalTrend map a 2-bit I062/200 axis (Mode of
// Movement, ICD 3.6.0) to its typed value, returning nil for the "undetermined"
// wire value (3) so an undetermined axis stays absent rather than asserting a
// state. Values 0..2 are the determined members; anything else (only 3 is
// reachable from a 2-bit field) is treated as undetermined.
func courseTrend(v uint8) *CourseTrend {
	var c CourseTrend
	switch v {
	case 0:
		c = CourseConstant
	case 1:
		c = CourseRight
	case 2:
		c = CourseLeft
	default:
		return nil
	}
	return &c
}

func speedTrend(v uint8) *SpeedTrend {
	var s SpeedTrend
	switch v {
	case 0:
		s = SpeedConstant
	case 1:
		s = SpeedIncreasing
	case 2:
		s = SpeedDecreasing
	default:
		return nil
	}
	return &s
}

func verticalTrend(v uint8) *VerticalTrend {
	var vt VerticalTrend
	switch v {
	case 0:
		vt = VerticalLevel
	case 1:
		vt = VerticalClimb
	case 2:
		vt = VerticalDescent
	default:
		return nil
	}
	return &vt
}

// decodeTrackStatus parses I062/080 (variable length, FX-chained). It reads the
// whole FX chain, then decodes the flags by octet position — mirroring Firefly's
// encoder: CNF in octet 1 (bit 2), TSE in octet 2 (bit 7), CST in octet 4
// (bit 8). Octets the record did not include are simply absent (the item
// extends only as far as the highest set flag), so each read is length-guarded.
func decodeTrackStatus(data []byte, offset int) (TrackStatus, int, error) {
	status := TrackStatus{}

	start := offset
	for {
		if offset >= len(data) {
			return status, offset, NewDecodeError("truncated I062/080")
		}
		oct := data[offset]
		offset++
		if (oct & 0x01) == 0 { // FX clear: end of chain
			break
		}
	}
	octets := data[start:offset]

	// Octet 1 is always present (the FX loop reads at least one octet). MON and
	// SPI (ICD 3.2.0) share it with CNF; TSE (octet 2) and CST (octet 4) extend
	// only as far as the highest set flag, so each higher-octet read is guarded.
	status.Confirmed = (octets[0] & 0x02) == 0                  // octet 1, CNF (1=tentative)
	status.Monosensor = (octets[0] & 0x80) != 0                 // octet 1, MON (single-sensor track)
	status.SPI = (octets[0] & 0x40) != 0                        // octet 1, SPI (ident pulse on last report)
	status.Ended = len(octets) >= 2 && (octets[1]&0x40) != 0    // octet 2, TSE
	status.Coasting = len(octets) >= 4 && (octets[3]&0x80) != 0 // octet 4, CST

	return status, offset, nil
}

// i380SubfieldLen maps the fixed-length I062/380 subfields to their octet count,
// so the decoder can length-skip a subfield Firefly does not send (forward
// tolerance, charter §2/§7). The subfields the decoder actually consumes (#1 ADR,
// #3 MHG, #6 SAL, #26 IAR, #27 MAC) are handled explicitly in
// decodeAircraftDerivedData. The variable-length subfields — #8 TIS (FX-extended),
// #9 TID and #25 MB (each a repetition factor) — are deliberately ABSENT: they
// cannot be skipped by a fixed length, so a record carrying one is rejected rather
// than mis-parsed (robust decoder, charter §7).
var i380SubfieldLen = map[int]int{
	1: 3, 2: 6, 3: 2, 4: 2, 5: 2, 6: 2, 7: 2,
	10: 2, 11: 2, 12: 7, 13: 2, 14: 2, 15: 2, 16: 2, 17: 2, 18: 2,
	19: 1, 20: 8, 21: 1, 22: 6, 23: 2, 24: 1,
	26: 2, 27: 2, 28: 2,
}

// maxI380SpecOctets caps the FX-chained subfield spec of I062/380. The standard
// UAP has 28 subfields → 4 octets; a longer chain is a hostile/garbled datagram.
const maxI380SpecOctets = 4

// decodeAircraftDerivedData parses I062/380 (Aircraft Derived Data / Mode-S DAPs,
// ICD 3.4.0). It is a compound item: an FX-chained subfield spec (up to 4 octets)
// followed by the present subfields in ascending number. The decoder reads the
// operationally useful ones — ADR (#1, ICAO address), MHG (#3, magnetic heading),
// SAL (#6, selected altitude — the level-bust signal), IAR (#26, IAS) and MAC
// (#27, Mach) — length-skips any other known fixed-length subfield, and rejects a
// variable/unknown subfield rather than desynchronising the record. A DAP-less
// track (spec = 0x80 + ADR only) decodes exactly as before this item became
// compound (no wire break for the common case).
func decodeAircraftDerivedData(data []byte, offset int, track *DecodedTrack) (int, error) {
	specStart := offset
	for {
		if offset >= len(data) {
			return offset, NewDecodeError("truncated I062/380 subfield spec")
		}
		oct := data[offset]
		offset++
		if oct&0x01 == 0 { // FX clear: end of spec
			break
		}
		if offset-specStart >= maxI380SpecOctets {
			return offset, NewDecodeError("I062/380 subfield spec too long")
		}
	}
	spec := data[specStart:offset]

	take := func(n int) ([]byte, error) {
		if offset+n > len(data) {
			return nil, NewDecodeError("truncated I062/380 subfield")
		}
		b := data[offset : offset+n]
		offset += n
		return b, nil
	}

	maxSub := len(spec) * 7
	for sf := 1; sf <= maxSub; sf++ {
		if !i380Present(spec, sf) {
			continue
		}
		switch sf {
		case 1: // ADR — 24-bit ICAO target address
			b, err := take(3)
			if err != nil {
				return offset, err
			}
			addr := (uint32(b[0]) << 16) | (uint32(b[1]) << 8) | uint32(b[2])
			track.ICAOAddr = &addr
		case 3: // MHG — Magnetic Heading, u16 × 360/2^16 degrees
			b, err := take(2)
			if err != nil {
				return offset, err
			}
			deg := float64(uint16(b[0])<<8|uint16(b[1])) * (360.0 / 65536.0)
			track.MagneticHeadingDeg = &deg
		case 6: // SAL — Selected Altitude. Bits 13-1 = altitude (13-bit two's
			// complement, LSB 25 ft); bit 16 = SAS, bits 15-14 = source (dropped —
			// the value is what feeds level-bust detection).
			b, err := take(2)
			if err != nil {
				return offset, err
			}
			v := int(uint16(b[0])<<8|uint16(b[1])) & 0x1FFF
			if v&0x1000 != 0 { // sign bit of the 13-bit field → sign-extend
				v -= 0x2000
			}
			ft := float64(v) * 25.0
			track.SelectedAltitudeFt = &ft
		case 26: // IAR — Indicated Airspeed, u16 × 1 kt
			b, err := take(2)
			if err != nil {
				return offset, err
			}
			kt := float64(uint16(b[0])<<8 | uint16(b[1]))
			track.IndicatedAirspeedKt = &kt
		case 27: // MAC — Mach Number, u16 × 0.008
			b, err := take(2)
			if err != nil {
				return offset, err
			}
			mach := float64(uint16(b[0])<<8|uint16(b[1])) * 0.008
			track.MachNumber = &mach
		default:
			n, ok := i380SubfieldLen[sf]
			if !ok {
				return offset, NewDecodeError(fmt.Sprintf("unsupported I062/380 subfield #%d", sf))
			}
			if _, err := take(n); err != nil {
				return offset, err
			}
		}
	}
	return offset, nil
}

// i380Present reports whether the I062/380 subfield spec marks subfield sf
// (1-based) present. Each spec octet carries 7 subfield bits (7..1) + an FX bit (0).
func i380Present(spec []byte, sf int) bool {
	oct := (sf - 1) / 7
	if oct >= len(spec) {
		return false
	}
	bit := 7 - ((sf - 1) % 7)
	return spec[oct]&(1<<bit) != 0
}

// decodeTargetIdentification decodes I062/245 (7 bytes): octet 1 is the
// STI/spare primary subfield and is dropped; octets 2-7 pack 8 characters as
// 8x6-bit IA-5 codes (48 bits), MSB-first. Trailing spaces are trimmed.
func decodeTargetIdentification(b []byte) string {
	var bits uint64
	for _, v := range b[1:7] {
		bits = (bits << 8) | uint64(v)
	}

	chars := make([]byte, 8)
	for i := range chars {
		shift := uint(7-i) * 6
		code := byte((bits >> shift) & 0x3F)
		chars[i] = ia5Decode(code)
	}

	return strings.TrimRight(string(chars), " ")
}

// ia5Decode decodes one 6-bit ASTERIX IA-5 code to ASCII (ICAO Annex 10):
// 1-26 -> 'A'-'Z', 48-57 -> '0'-'9', anything else (including 32, space)
// defensively maps to space. A foreign/malformed datagram can send any
// 6-bit value; never let it produce an unexpected byte.
func ia5Decode(code byte) byte {
	switch {
	case code >= 1 && code <= 26:
		return 'A' + (code - 1)
	case code >= 48 && code <= 57:
		return '0' + (code - 48)
	default:
		return ' '
	}
}

// signExtendI24 converts a 24-bit signed value (stored in bits 0-23 of a 32-bit int)
// to a proper 32-bit signed integer with sign extension.
// Bit 23 is the sign bit; if set, all upper bits (24-31) are set to 1.
func signExtendI24(val int32) int32 {
	// If sign bit (bit 23) is set, this is a negative number in 24-bit representation.
	// Subtract 2^24 to convert to proper 32-bit signed two's complement.
	if (val & 0x800000) != 0 {
		val -= (1 << 24)
	}
	return val
}
