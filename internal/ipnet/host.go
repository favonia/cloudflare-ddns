package ipnet

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// HostID is the host part of an IPv6 address.
type HostID interface {
	// Describe prints the HostID.
	Describe() string

	Normalize(ppfmt pp.PP, domain domain.Domain, prefixLen int) (HostID, bool)

	// WithPrefix calculates the new address with a prefix.
	WithPrefix(prefix netip.Prefix) netip.Addr
}

// mask gives a bitwise mask:
// - mask(0): 11111111.
// - mask(1): 01111111.
// - mask(2): 00111111.
// - mask(3): 00011111.
// - mask(4): 00001111.
// - mask(5): 00000111.
// - mask(6): 00000011.
// - mask(7): 00000001.
func mask(s int) byte {
	return ^byte(0) >> s
}

// IP6Suffix represents a suffix of an IPv6 address.
type IP6Suffix [16]byte

// Describe prints the suffix as an IPv6 address.
func (r IP6Suffix) Describe() string { return netip.AddrFrom16(r).String() }

func (r IP6Suffix) mask(prefixLen int) IP6Suffix {
	for i := range prefixLen / 8 {
		r[i] = 0
	}
	if prefixLen%8 > 0 {
		r[prefixLen/8] &= mask(prefixLen % 8)
	}
	return r
}

// Normalize masks the suffix with the given prefix length.
func (r IP6Suffix) Normalize(ppfmt pp.PP, domain domain.Domain, prefixLen int) (HostID, bool) {
	if prefixLen < 0 || prefixLen > 128 {
		ppfmt.Noticef(pp.EmojiImpossible, "IP6_PREFIX_LEN (%d) should be in the range 0 to 128", prefixLen)
		return nil, false
	}
	normalized := r.mask(prefixLen)
	if normalized != r {
		ppfmt.Infof(pp.EmojiTruncate, "The host ID %q of %q was truncated to %q (with %d higher bits removed)",
			r.Describe(), domain.Describe(), normalized.Describe(), prefixLen)
	}
	return normalized, true
}

// WithPrefix combines a prefix and a host ID to construct an IPv6 address.
func (r IP6Suffix) WithPrefix(prefix netip.Prefix) netip.Addr {
	ip := r.mask(prefix.Bits())
	prefixAsBytes := prefix.Masked().Addr().As16()
	for i := range 128 / 8 {
		ip[i] |= prefixAsBytes[i]
	}
	return netip.AddrFrom16(ip)
}

// EUI48 represents a MAC (EUI-48) address.
type EUI48 [6]byte

// Describe prints the suffix as a MAC address.
func (e EUI48) Describe() string { return net.HardwareAddr(e[:]).String() }

// Normalize masks the suffix with the given prefix length.
func (e EUI48) Normalize(ppfmt pp.PP, domain domain.Domain, prefixLen int) (HostID, bool) {
	if prefixLen < 0 || prefixLen > 128 {
		ppfmt.Noticef(pp.EmojiImpossible, "IP6_PREFIX_LEN (%d) should be in the range 0 to 128", prefixLen)
		return nil, false
	}
	if prefixLen > 64 {
		ppfmt.Noticef(pp.EmojiUserError, "IP6_PREFIX_LEN (%d) is too large (> 64) to use the MAC (EUI-48) address %q as the IPv6 host ID of %q. Converting a MAC address to a host ID requires IPv6 Stateless Address Auto-configuration (SLAAC), which necessitates an IPv6 range of size at least /64 (represented by a prefix length at most 64).", //nolint:lll
			prefixLen, e.Describe(), domain.Describe())
		return nil, false
	}
	return e, true
}

// WithPrefix combines a prefix and a host ID to construct an IPv6 address.
func (e EUI48) WithPrefix(prefix netip.Prefix) netip.Addr {
	if prefix.Bits() > 64 {
		return netip.Addr{}
	}
	prefixAsBytes := prefix.Masked().Addr().As16()

	bytes := [16]byte{
		prefixAsBytes[0],
		prefixAsBytes[1],
		prefixAsBytes[2],
		prefixAsBytes[3],
		prefixAsBytes[4],
		prefixAsBytes[5],
		prefixAsBytes[6],
		prefixAsBytes[7],
		e[0] ^ 0x02, // flip the global-local bit
		e[1],
		e[2],
		0xff,
		0xfe,
		e[3],
		e[4],
		e[5],
	}
	return netip.AddrFrom16(bytes)
}

// Errors from ParseHost.
var (
	ErrHostIDHasIP6Zone   = errors.New("an IPv6 address as a host ID must not have an IPv6 zone")
	ErrIP4AddressAsHostID = errors.New("cannot use IPv4 address as a host ID")
	ErrEUI64AsHostID      = errors.New("cannot use EUI-64 address as a host ID")
	ErrIPOIBAsHostID      = errors.New("cannot use IP address over InfiniBand as a host ID")
)

// ParseHost parses a host ID for an IPv6 address.
func ParseHost(s string) (HostID, error) {
	if s == "" {
		return nil, nil //nolint:nilnil
	}

	ip, errIP := netip.ParseAddr(s)
	if errIP == nil {
		if !ip.Is6() {
			return nil, ErrIP4AddressAsHostID
		}
		if ip.Zone() != "" {
			return nil, ErrHostIDHasIP6Zone
		}

		return IP6Suffix(ip.As16()), nil
	}

	// Possible formats for MAC (EUI-48)
	// 00:00:5e:00:53:01
	// 00-00-5e-00-53-01
	// 0000.5e00.5301
	mac, errMAC := net.ParseMAC(s)
	if errMAC != nil {
		return nil, fmt.Errorf("not an IPv6 address (%w) and not a MAC (EUI-48) address (%w)", errIP, errMAC)
	}
	switch len(mac) {
	case 6:
		return EUI48(mac), nil
	case 8:
		return nil, ErrEUI64AsHostID
	case 20:
		return nil, ErrIPOIBAsHostID
	default:
		return nil, fmt.Errorf("not an IPv6 address (%w) and not a MAC (EUI-48) address", errIP)
	}
}
