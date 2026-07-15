// Package cat065 decodes ASTERIX CAT065 "SDPS Service Status Messages" — the
// feed heartbeat (Firefly ADR 0018, ICD-CAT062.md §8, v2.3.0).
//
// CAT065 travels on the same UDP-multicast group as CAT062; a receiver
// dispatches on the leading CAT octet (0x3E → tracks, 0x41 → status). The
// heartbeat lets the ASD tell an empty sky (a valid, track-less CAT062 block)
// apart from a dead feed (nothing arriving at all), which is the basis for
// staleness detection and a meaningful readiness signal.
//
// Like the CAT062 decoder, this one never trusts a datagram: every read is
// bounds-checked and a malformed block is rejected with an error rather than
// panicking (CLAUDE.md §7, robust decoder).
package cat065

import "fmt"

// Category is the ASTERIX category octet for SDPS service status messages.
const Category = 0x41 // 65

// MessageTypeSDPSStatus is I065/000 for a periodic SDPS status report — the
// heartbeat we care about.
const MessageTypeSDPSStatus = 1

// ServiceStatus is one decoded CAT065 SDPS status report.
type ServiceStatus struct {
	// I065/010 — the data source that produced this status.
	SAC uint8
	SIC uint8
	// I065/000 — message type (MessageTypeSDPSStatus for a heartbeat).
	MessageType uint8
	// I065/015 — service identification.
	ServiceID uint8
	// I065/030 — time of day, seconds since UTC midnight (wraps every 24 h).
	TimeOfDay float64
	// I065/040 NOGO — true when the SDPS reports itself operational.
	Operational bool
}

// timeLSBSeconds: I065/030 counts 1/128-second ticks (as I062/070).
const timeLSBSeconds = 1.0 / 128.0

// maxFSPECOctets caps the FX-chained FSPEC length (the CAT065 UAP ends at
// FRN 7 → 1 octet). Beyond this a datagram is hostile or garbled and is
// rejected. The cap also bounds the FRN iteration in decodeRecord to a safe
// range, so an overlong chain can never overflow the loop counter. Wayfinder
// #235 (mirror of Firefly's QW.2 FSPEC-hardening fix).
const maxFSPECOctets = 36

// FRN widths (octets) for the CAT065 UAP items we read.
const (
	frnDataSource     = 1 // I065/010, 2 octets
	frnMessageType    = 2 // I065/000, 1 octet
	frnServiceID      = 3 // I065/015, 1 octet
	frnTimeOfDay      = 4 // I065/030, 3 octets
	frnBatchNumber    = 5 // I065/020, 1 octet (other message types)
	frnSDPSConfig     = 6 // I065/040, 1 octet
	frnServiceReport  = 7 // I065/050, 1 octet (other message types)
	sdpsStatusNOGOBit = 0xC0
)

// DecodeError is a CAT065 parsing error.
type DecodeError struct{ msg string }

func (e DecodeError) Error() string { return "CAT065 decode error: " + e.msg }

func newErr(format string, args ...any) DecodeError {
	return DecodeError{msg: fmt.Sprintf(format, args...)}
}

// DecodeStatusBlock parses a CAT065 data block: [CAT=0x41][LEN: u16 BE][Record].
func DecodeStatusBlock(data []byte) ([]ServiceStatus, error) {
	if len(data) < 3 {
		return nil, newErr("data block too short")
	}
	if data[0] != Category {
		return nil, newErr("invalid CAT: 0x%02x (expected 0x41)", data[0])
	}
	lenVal := (int(data[1]) << 8) | int(data[2])
	if lenVal < 3 {
		return nil, newErr("invalid LEN (too small)")
	}
	if lenVal > len(data) {
		return nil, newErr("LEN exceeds data length")
	}

	var reports []ServiceStatus
	offset := 3
	for offset < lenVal {
		status, next, err := decodeRecord(data, offset, lenVal)
		if err != nil {
			return nil, err
		}
		reports = append(reports, status)
		offset = next
	}
	return reports, nil
}

// decodeRecord parses one status record starting at offset, bounded by end (the
// block's declared length). Returns the status and the offset just past it.
func decodeRecord(data []byte, offset, end int) (ServiceStatus, int, error) {
	var status ServiceStatus

	// Parse the FSPEC: octets up to and including the first with FX (bit 0) clear.
	// A crafted datagram could chain FX forever; cap the length so the parse can
	// neither read nor (below) iterate an unbounded FSPEC (Wayfinder #235).
	fspecStart := offset
	for {
		if offset >= end {
			return status, offset, newErr("truncated FSPEC")
		}
		fx := data[offset]&0x01 != 0
		offset++
		if !fx {
			break
		}
		if offset-fspecStart >= maxFSPECOctets {
			return status, offset, newErr("FSPEC exceeds maximum length (%d octets)", maxFSPECOctets)
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

	// Iterate FRNs as an int, not a uint8, so the counter cannot wrap past 255
	// and loop forever on a crafted (over-long) FSPEC (Wayfinder #235). The cap
	// above already bounds maxFRN to maxFSPECOctets*7 = 252.
	var haveSource, haveType, haveService, haveTime, haveStatus bool
	maxFRN := len(fspec) * 7
	for frn := 1; frn <= maxFRN; frn++ {
		if !fspecHas(fspec, frn) {
			continue
		}
		switch frn {
		case frnDataSource:
			b, err := take(2)
			if err != nil {
				return status, offset, err
			}
			status.SAC, status.SIC, haveSource = b[0], b[1], true
		case frnMessageType:
			b, err := take(1)
			if err != nil {
				return status, offset, err
			}
			status.MessageType, haveType = b[0], true
		case frnServiceID:
			b, err := take(1)
			if err != nil {
				return status, offset, err
			}
			status.ServiceID, haveService = b[0], true
		case frnTimeOfDay:
			b, err := take(3)
			if err != nil {
				return status, offset, err
			}
			ticks := (uint32(b[0]) << 16) | (uint32(b[1]) << 8) | uint32(b[2])
			status.TimeOfDay, haveTime = float64(ticks)*timeLSBSeconds, true
		case frnBatchNumber:
			if _, err := take(1); err != nil { // present in other message types
				return status, offset, err
			}
		case frnSDPSConfig:
			b, err := take(1)
			if err != nil {
				return status, offset, err
			}
			status.Operational, haveStatus = b[0]&sdpsStatusNOGOBit == 0, true
		case frnServiceReport:
			if _, err := take(1); err != nil { // present in other message types
				return status, offset, err
			}
		default:
			return status, offset, newErr("unknown FRN %d present", frn)
		}
	}

	if !haveSource || !haveType || !haveService || !haveTime || !haveStatus {
		return status, offset, newErr("status record missing a required item")
	}
	return status, offset, nil
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
