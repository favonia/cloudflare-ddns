// Package ipnet contains utility functions for IPv4 and IPv6 networks.
package ipnet

import (
	"fmt"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Type is the type of IP networks.
type Type int

const (
	// IP4 is IP version 4.
	IP4 Type = 4

	// IP6 is IP version 6.
	IP6 Type = 6
)

// Describe returns a human-readable description of the IP network.
func (t Type) Describe() string {
	switch t {
	case IP4, IP6:
		return fmt.Sprintf("IPv%d", t)
	default:
		return "<unrecognized IP network>"
	}
}

// RecordType prints out the type of DNS records for the IP network. For IPv4, it is A; for IPv6, it is AAAA.
func (t Type) RecordType() string {
	switch t {
	case IP4:
		return "A"
	case IP6:
		return "AAAA"
	default:
		return ""
	}
}

// Int returns the version of the IP networks. It is either 4 or 6.
func (t Type) Int() int {
	switch t {
	case IP4, IP6:
		return int(t)
	default:
		return 0
	}
}

// NormalizeDetectedIP normalizes an IP into an IPv4 or IPv6 address.
func (t Type) NormalizeDetectedIP(ppfmt pp.PP, ip netip.Addr) (netip.Addr, bool) {
	if !ip.IsValid() {
		ppfmt.Warningf(
			pp.EmojiImpossible,
			`Detected IP address is not valid`,
		)
		return netip.Addr{}, false
	}

	if ip.IsUnspecified() {
		ppfmt.Warningf(
			pp.EmojiImpossible,
			`Detected IP address %s is an unspecified %s address`,
			ip.String(),
			t.Describe(),
		)
		return netip.Addr{}, false
	}

	switch t {
	case IP4:
		if !ip.Is4() && !ip.Is4In6() {
			ppfmt.Warningf(pp.EmojiError, "Detected IP address %s is not a valid %s address", ip.String(), t.Describe())
			return netip.Addr{}, false
		}
		// Turns an IPv4-mapped IPv6 address back to an IPv4 address
		ip = ip.Unmap()
	case IP6:
		ip = netip.AddrFrom16(ip.As16())
	default:
		ppfmt.Warningf(pp.EmojiImpossible, "Detected IP address %s is not a valid %s address", ip.String(), t.Describe())
		ppfmt.Warningf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new") //nolint:lll
		return netip.Addr{}, false
	}

	if !ip.IsGlobalUnicast() {
		ppfmt.Warningf(
			pp.EmojiUserWarning,
			`Detected IP address %s does not look like a global unicast IP address. Please double-check.`,
			ip.String(),
		)
	}

	return ip, true
}

// UDPNetwork gives the network name for net.Dial.
func (t Type) UDPNetwork() string {
	switch t {
	case IP4:
		return "udp4"
	case IP6:
		return "udp6"
	default:
		return ""
	}
}

// Matches checks whether an IP belongs to it.
func (t Type) Matches(ip netip.Addr) bool {
	switch t {
	case IP4:
		return ip.Is4()
	case IP6:
		return ip.Is6()
	default:
		return false
	}
}
