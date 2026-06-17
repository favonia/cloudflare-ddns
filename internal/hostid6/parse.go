package hostid6

import (
	"errors"
	"strconv"
)

var errInvalidMAC = errors.New("invalid 48-bit MAC address")

// ParseMAC parses an exactly 48-bit MAC address in hyphen or colon notation.
//
// We deliberately avoid net.ParseMAC: it also accepts EUI-64, 20-byte
// InfiniBand, and Cisco dotted notation (xxxx.xxxx.xxxx), so delegating to it
// would hand the accepted-input spec to the standard library and silently admit
// the dotted form. No host OS emits dotted notation, and network engineers read
// colon/hyphen just as fluently, so this strict 48-bit subset loses nothing
// while keeping the accepted language trivially specifiable.
func ParseMAC(text string) ([6]byte, error) {
	var mac [6]byte

	// A 48-bit MAC address has 6 octets (2 hex digits each) and 5 separators: 6*2 + 5 = 17.
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
