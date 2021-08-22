// Package ipnet contains utility functions for IP network versions
package ipnet

import (
	"fmt"
	"net"
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

func (t Type) NormalizeIP(ip net.IP) net.IP {
	switch t {
	case IP4:
		return ip.To4()
	case IP6:
		return ip.To16()
	default:
		return ip
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
