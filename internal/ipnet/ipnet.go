// Package ipnet contains utility functions for IP network versions
package ipnet

import (
	"fmt"
	"net/netip"
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

func (t Type) NormalizeIP(ip netip.Addr) (netip.Addr, bool) {
	if !ip.IsValid() {
		return ip, false
	}

	switch t {
	case IP4:
		// Turns an IPv4-mapped IPv6 address back to an IPv4 address
		ip = ip.Unmap()
		return ip, ip.Is4()
	case IP6:
		// FIXME: wait until netip gets updated
		ip = netip.AddrFrom16(ip.As16())
		return ip, ip.Is6()
	default:
		return ip, true
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
