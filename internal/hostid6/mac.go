package hostid6

import (
	"errors"
	"strconv"
)

var errInvalidMAC = errors.New("invalid 48-bit MAC address")

// ParseMAC parses an exactly 48-bit MAC address in hyphen or colon notation.
func ParseMAC(text string) ([6]byte, error) {
	var mac [6]byte

	if len(text) != 17 {
		return mac, errInvalidMAC
	}

	separator := text[2]
	if separator != '-' && separator != ':' {
		return mac, errInvalidMAC
	}

	for i := range mac {
		offset := i * 3
		if i > 0 && text[offset-1] != separator {
			return [6]byte{}, errInvalidMAC
		}

		octet, err := strconv.ParseUint(text[offset:offset+2], 16, 8)
		if err != nil {
			return [6]byte{}, errInvalidMAC
		}
		mac[i] = byte(octet)
	}

	return mac, nil
}
