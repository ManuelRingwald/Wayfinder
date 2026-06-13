package cat062

import (
	"fmt"
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

	// Decode items in UAP order (FRN 1, 4, 5, 6, 7, 9, 11, 12, 13, 14, 16)
	uapOrder := []uint8{1, 4, 5, 6, 7, 9, 11, 12, 13, 14, 16}

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

		case 9: // I062/060: Mode 3/A Code (2 bytes, 12-bit code in low bits)
			if offset+2 > len(data) {
				return track, offset, NewDecodeError("truncated I062/060")
			}
			code := uint16((uint16(data[offset]) << 8) | uint16(data[offset+1]))
			code &= 0x0FFF // low 12 bits
			track.Mode3A = &code
			offset += 2

		case 11: // I062/380: Aircraft Derived Data (variable, Target Address in ADR subfield)
			if offset+1 > len(data) {
				return track, offset, NewDecodeError("truncated I062/380 primary subfield")
			}
			primary := data[offset]
			offset++

			// Check ADR bit (bit 7, 0x80): Target Address present
			if (primary & 0x80) != 0 {
				if offset+3 > len(data) {
					return track, offset, NewDecodeError("truncated I062/380 ADR")
				}
				addr := (uint32(data[offset]) << 16) | (uint32(data[offset+1]) << 8) | uint32(data[offset+2])
				track.ICAOAddr = &addr
				offset += 3
			}

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

		case 14: // I062/290: Update Ages (compound, currently just PSR age)
			if offset+1 > len(data) {
				return track, offset, NewDecodeError("truncated I062/290 primary")
			}
			primary := data[offset]
			offset++

			// Check PSR bit (bit 6, 0x40)
			if (primary & 0x40) != 0 {
				if offset+1 > len(data) {
					return track, offset, NewDecodeError("truncated I062/290 PSR")
				}
				psrAgeTicks := data[offset]
				track.UpdateAge.PSRAge = float64(psrAgeTicks) * 0.25 // LSB = 1/4 s
				offset++
			}

		case 16: // I062/500: Estimated Accuracies (compound, currently just APC)
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

// decodeTrackStatus parses I062/080 (variable length with FX chaining).
func decodeTrackStatus(data []byte, offset int) (TrackStatus, int, error) {
	status := TrackStatus{}

	if offset >= len(data) {
		return status, offset, NewDecodeError("truncated I062/080")
	}

	// Octet 1
	oct1 := data[offset]
	status.Confirmed = (oct1 & 0x02) == 0 // CNF bit: 0=confirmed, 1=tentative
	offset++

	// Check FX bit in octet 1
	if (oct1 & 0x01) == 0 {
		// No further octets
		return status, offset, nil
	}

	// Octet 2+
	for {
		if offset >= len(data) {
			return status, offset, NewDecodeError("truncated I062/080 continuation")
		}
		oct := data[offset]
		offset++

		// Example: octet 4, bit 7 (0x80) = CST (coasting)
		// In practice, octet positions vary; simplify for now.
		status.Coasting = (oct & 0x80) != 0

		if (oct & 0x01) == 0 {
			break
		}
	}

	return status, offset, nil
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
