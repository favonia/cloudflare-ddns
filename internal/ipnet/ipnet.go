// Package ipnet contains utility functions for IP network versions
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

// Describe returns a description of the IP network.
func (t Type) Describe() string {
	switch t {
	case IP4, IP6:
		return fmt.Sprintf("IPv%d", t)
	default:
		return "<unrecognized IP network>"
	}
}

// RecordType prints out the type of DNS records for the IP network.
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

// NormalizeIP normalizes an IP into an IPv4 or IPv6 address.
func (t Type) NormalizeIP(ppfmt pp.PP, ip netip.Addr) (netip.Addr, bool) {
	inputIP := ip
	switch t {
	case IP4:
		// Turns an IPv4-mapped IPv6 address back to an IPv4 address
		ip = ip.Unmap()
		if !ip.Is4() {
			ppfmt.Warningf(pp.EmojiError, "%q is not a valid %s address", inputIP, t.Describe())
			return netip.Addr{}, false
		}
		return ip, true
	case IP6:
		ip = netip.AddrFrom16(ip.As16())
		if !ip.Is6() {
			ppfmt.Warningf(pp.EmojiError, "%q is not a valid %s address", inputIP, t.Describe())
			return netip.Addr{}, false
		}
		return ip, true
	default:
		ppfmt.Warningf(pp.EmojiError, "%q is not a valid %s address", inputIP, t.Describe())
		return netip.Addr{}, false
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
