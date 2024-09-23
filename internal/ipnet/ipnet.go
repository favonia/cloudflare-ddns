// Package ipnet contains utility functions for IPv4 and IPv6 networks.
package ipnet

import (
	"fmt"
	"iter"
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

// Int returns the version of the IP networks. It is either 4 or 6.
func (t Type) Int() int {
	switch t {
	case IP4, IP6:
		return int(t)
	default:
		return 0
	}
}

// Describe returns a human-readable description of the IP network.
func (t Type) Describe() string {
	switch t {
	case IP4, IP6:
		return fmt.Sprintf("IPv%d", t)
	default:
		return ""
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
	ip = ip.Unmap()
	switch t {
	case IP4:
		return ip.Is4()
	case IP6:
		return ip.Is6()
	default:
		return false
	}
}

// NormalizeDetectedIP normalizes an IP into an IPv4 or IPv6 address.
func (t Type) NormalizeDetectedIP(ppfmt pp.PP, ip netip.Addr) (netip.Addr, bool) {
	if !ip.IsValid() {
		ppfmt.Noticef(pp.EmojiImpossible,
			`Detected IP address is not valid; this should not happen and please report it at %s`,
			pp.IssueReportingURL,
		)
		return netip.Addr{}, false
	}

	switch t {
	case IP4:
		if !ip.Is4() && !ip.Is4In6() {
			ppfmt.Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", ip.String())
			return netip.Addr{}, false
		}
		// Turns an IPv4-mapped IPv6 address back to an IPv4 address
		ip = ip.Unmap()

	case IP6:
		// Turns an IPv4-mapped IPv6 address back to an IPv4 address
		if !ip.Is6() {
			ppfmt.Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv6 address", ip.String())
			return netip.Addr{}, false
		}
		if ip.Is4In6() {
			ppfmt.Noticef(pp.EmojiError, "Detected IP address %s is an IPv4-mapped IPv6 address", ip.String())
			ppfmt.Hintf(pp.HintIP4MappedIP6Address,
				"An IPv4-mapped IPv6 address is an IPv4 address in disguise. "+
					"It cannot be used for routing IPv6 traffic. "+
					"If you need to use it for DNS, please open an issue at %s",
				pp.IssueReportingURL)
			return netip.Addr{}, false
		}

	default:
		return netip.Addr{}, false
	}

	switch {
	case ip.IsUnspecified():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is an unspecified address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	case ip.IsLoopback():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is a loopback address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	case ip.IsInterfaceLocalMulticast():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is an interface-local multicast address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	case ip.IsLinkLocalMulticast(), ip.IsLinkLocalUnicast():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is a link-local address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	}

	if !ip.IsGlobalUnicast() {
		ppfmt.Noticef(
			pp.EmojiWarning,
			`Detected %s address %s does not look like a global unicast address`,
			t.Describe(), ip.String(),
		)
	}

	return ip, true
}

// All enumerates [IP4] and then [IP6].
func All(yield func(Type) bool) {
	_ = yield(IP4) && yield(IP6)
}

// Bindings enumerates the key [IP4] and then [IP6] for a map.
func Bindings[V any](m map[Type]V) iter.Seq2[Type, V] {
	return func(yield func(Type, V) bool) {
		for ipNet := range All {
			v, ok := m[ipNet]
			if ok {
				if !yield(ipNet, v) {
					return
				}
			}
		}
	}
}
