// Package cat063 decodes ASTERIX CAT063 "Sensor Status Messages" — the
// per-sensor liveness signal (Firefly ICD 2.5.0, ADR 0022).
//
// CAT063 travels on the same UDP-multicast group as CAT062 and CAT065; a
// receiver dispatches on the leading CAT octet (0x3F → sensor status). Each
// block carries one record per registered sensor, sent wall-clock-periodically
// (default 5 s). Together with CAT065 (global SDPS heartbeat), CAT063 lets
// Wayfinder distinguish a failed sensor from an empty sky — the basis for the
// sensor-degradation banner (yellow).
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

// FRN indices for the CAT063 UAP items we read (ICD §9, Firefly ADR 0022).
const (
	frnDataSource = 1 // I063/010, 2 octets: SAC + SIC
	frnTimeOfDay  = 2 // I063/030, 3 octets: Time of Day
	frnNOGO       = 3 // I063/060, 1 octet: Sensor Configuration & Status
)

// nogoBits is the mask for the NOGO sub-field of I063/060 (bits 8/7 = 0xC0).
// 0x00 = operational, 0x40 = degraded, 0x80 = not connected, 0xC0 = not
// initialized. Firefly emits only 0x00 (active) and 0x40 (degraded).
const nogoBits = 0xC0

// DecodeError is a CAT063 parsing error.
type DecodeError struct{ msg string }

func (e DecodeError) Error() string { return "CAT063 decode error: " + e.msg }

func newErr(format string, args ...any) DecodeError {
	return DecodeError{msg: fmt.Sprintf(format, args...)}
}

// SensorStatus is one decoded CAT063 sensor status record.
type SensorStatus struct {
	// I063/010 — data source identifier of the sensor.
	SAC uint8
	SIC uint8
	// I063/030 — time of day, seconds since UTC midnight (wraps every 24 h).
	TimeOfDay float64
	// I063/060 NOGO — true when the sensor reports itself operational (NOGO = 00).
	Operational bool
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
	}
	fspec := data[fspecStart:offset]

	// take advances offset by n octets within the record bounds.
	take := func(n int) ([]byte, error) {
		if end-offset < n {
			return nil, newErr("truncated item (need %d octets)", n)
		}
		b := data[offset : offset+n]
		offset += n
		return b, nil
	}

	var haveSource, haveStatus bool
	for frn := uint8(1); int((frn-1)/7) < len(fspec); frn++ {
		if !fspecHas(fspec, frn) {
			continue
		}
		switch frn {
		case frnDataSource:
			b, err := take(2)
			if err != nil {
				return s, offset, err
			}
			s.SAC, s.SIC, haveSource = b[0], b[1], true
		case frnTimeOfDay:
			b, err := take(3)
			if err != nil {
				return s, offset, err
			}
			ticks := (uint32(b[0]) << 16) | (uint32(b[1]) << 8) | uint32(b[2])
			s.TimeOfDay = float64(ticks) * timeLSBSeconds
		case frnNOGO:
			b, err := take(1)
			if err != nil {
				return s, offset, err
			}
			s.Operational, haveStatus = b[0]&nogoBits == 0, true
		default:
			return s, offset, newErr("unknown FRN %d present", frn)
		}
	}

	if !haveSource || !haveStatus {
		return s, offset, newErr("sensor status record missing a required item")
	}
	return s, offset, nil
}

// fspecHas reports whether the FSPEC marks the given FRN present.
func fspecHas(fspec []byte, frn uint8) bool {
	octet := int((frn - 1) / 7)
	if octet >= len(fspec) {
		return false
	}
	bit := 7 - ((frn - 1) % 7)
	return fspec[octet]&(1<<bit) != 0
}
