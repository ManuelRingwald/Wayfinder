package cat062

// FSPEC holds the Field Specification (which items are present in a record).
// Each byte's bits 8-2 indicate FRN 1-7; bit 1 (FX) indicates if another byte follows.
// Bit 7 (MSB) is reserved.
type FSPEC struct {
	octets []uint8
}

// NewFSPEC parses FSPEC octets from a data stream.
// It reads octets until one has FX=0 (no more octets).
func NewFSPEC(data []byte, offset int) (*FSPEC, int, error) {
	var octets []uint8
	i := offset

	for {
		if i >= len(data) {
			return nil, i, ErrTruncated
		}
		oct := data[i]
		octets = append(octets, oct)
		i++

		// Check FX bit (bit 0, LSB): 1 = another octet follows, 0 = end of FSPEC
		if (oct & 0x01) == 0 {
			break
		}
	}

	return &FSPEC{octets: octets}, i, nil
}

// HasItem returns true if the given FRN (1-7 per octet) is present.
// FRN numbering: octet 0 has bits for FRN 1-7, octet 1 has bits for FRN 8-14, etc.
func (f *FSPEC) HasItem(frn uint8) bool {
	if frn < 1 {
		return false
	}

	// FRN to (octet index, bit position in that octet)
	octetIdx := (frn - 1) / 7
	bitPos := 7 - ((frn - 1) % 7) // bits 7-1 in descending order (MSB=bit7 for FRN1)

	if octetIdx >= uint8(len(f.octets)) {
		return false
	}

	return (f.octets[octetIdx] & (1 << bitPos)) != 0
}

// Errors
var (
	ErrTruncated = NewDecodeError("truncated data")
)

// DecodeError is a parsing error.
type DecodeError struct {
	msg string
}

func NewDecodeError(msg string) DecodeError {
	return DecodeError{msg: msg}
}

func (e DecodeError) Error() string {
	return "CAT062 decode error: " + e.msg
}
