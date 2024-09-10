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

// Describe returns a human-readable description of the IP network.
func (t Type) Describe() string {
	switch t {
	case IP4, IP6:
		return fmt.Sprintf("IPv%d", t)
	default:
		return fmt.Sprintf("<unrecognized IP version %d>", t)
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
		ppfmt.Noticef(
			pp.EmojiImpossible,
			`Detected IP address is not valid`,
		)
		return netip.Addr{}, false
	}

	if ip.IsUnspecified() {
		ppfmt.Noticef(
			pp.EmojiImpossible,
			`Detected IP address %s is an unspecified %s address`,
			ip.String(),
			t.Describe(),
		)
		return netip.Addr{}, false
	}

	switch t {
	case IP4:
		// Turns an IPv4-mapped IPv6 address back to an IPv4 address
		ip = ip.Unmap()

		if !ip.Is4() {
			ppfmt.Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", ip.String())
			return netip.Addr{}, false
		}
	case IP6:
		// If the address is an IPv4 address, map it back to an IPv6 address.
		ip = netip.AddrFrom16(ip.As16())
	default:
		ppfmt.Noticef(pp.EmojiImpossible,
			"Unrecognized IP version %d was used; please report this at %s", int(t), pp.IssueReportingURL)
		return netip.Addr{}, false
	}

	if !ip.IsGlobalUnicast() {
		ppfmt.Noticef(
			pp.EmojiUserWarning,
			`Detected IP address %s does not look like a global unicast IP address.`,
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
