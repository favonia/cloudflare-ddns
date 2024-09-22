package protocol

import (
	"context"
	"net"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// LocalAuto detects the IP address by pretending to send out an UDP packet
// and using the source IP address assigned by the system. In most cases
// it will detect the IP address of the network interface toward the internet.
// (No actual UDP packets will be sent out.)
type LocalAuto struct {
	// Name of the detection protocol.
	ProviderName string

	// The target of the hypothetical UDP packet to be sent.
	RemoteUDPAddr string
}

// Name of the detection protocol.
func (p LocalAuto) Name() string {
	return p.ProviderName
}

// ExtractUDPAddr converts an address from [net.Interface.Addrs] to [netip.Addr].
// The address will be unmapped.
func ExtractUDPAddr(ppfmt pp.PP, addr net.Addr) (netip.Addr, bool) {
	switch v := addr.(type) {
	case *net.UDPAddr:
		return v.AddrPort().Addr().Unmap(), true
	default:
		ppfmt.Noticef(pp.EmojiImpossible, "Unexpected address data of type %T when detecting a local address", addr)
		return netip.Addr{}, false
	}
}

// GetIP detects the IP address by pretending to send an UDP packet.
// (No actual UDP packets will be sent out.)
func (p LocalAuto) GetIP(_ context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, Method, bool) {
	conn, err := net.Dial(ipNet.UDPNetwork(), p.RemoteUDPAddr)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to detect a local %s address: %v", ipNet.Describe(), err)
		return netip.Addr{}, MethodUnspecified, false
	}
	defer conn.Close()

	ip, ok := ExtractUDPAddr(ppfmt, conn.LocalAddr())
	if !ok {
		return netip.Addr{}, MethodUnspecified, false
	}

	normalizedIP, ok := ipNet.NormalizeDetectedIP(ppfmt, ip)
	return normalizedIP, MethodPrimary, ok
}
