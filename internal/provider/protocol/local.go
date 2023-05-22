package protocol

import (
	"context"
	"net"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Local detects the IP address by pretending to send out an UDP packet
// and using the source IP address assigned by the system. In most cases
// it will detect the IP address of the network interface toward the internet.
// (No actual UDP packets will be sent out.)
type Local struct {
	// Name of the detection protocol.
	ProviderName string

	// Whether 1.1.1.1 is used for IPv4
	Is1111UsedForIP4 bool

	// The target IP address of the UDP packet to be sent.
	RemoteUDPAddr map[ipnet.Type]Switch
}

// Name of the detection protocol.
func (p *Local) Name() string {
	return p.ProviderName
}

// GetIP detects the IP address by pretending to send an UDP packet.
// (No actual UDP packets will be sent out.)
func (p *Local) GetIP(_ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, use1001 bool) (netip.Addr, bool) {
	var invalidIP netip.Addr

	remoteUDPAddr, found := p.RemoteUDPAddr[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return invalidIP, false
	}

	conn, err := net.Dial(ipNet.UDPNetwork(), remoteUDPAddr.Switch(use1001))
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to detect a local %s address: %v", ipNet.Describe(), err)
		return invalidIP, false
	}
	defer conn.Close()

	ip := conn.LocalAddr().(*net.UDPAddr).AddrPort().Addr() //nolint:forcetypeassert

	return ipNet.NormalizeDetectedIP(ppfmt, ip)
}

// ShouldWeCheck1111 returns whether we should check 1.1.1.1.
func (p *Local) ShouldWeCheck1111() bool { return p.Is1111UsedForIP4 }
