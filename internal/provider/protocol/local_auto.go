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
		ip := v.AddrPort().Addr().Unmap()
		if !ip.IsValid() {
			ppfmt.Noticef(pp.EmojiImpossible, "Failed to parse UDP source address %q", v.IP.String())
			return netip.Addr{}, false
		}
		return ip, ip.IsValid()
	default:
		ppfmt.Noticef(pp.EmojiImpossible, "Unexpected UDP source address data %q of type %T", addr.String(), addr)
		return netip.Addr{}, false
	}
}

// GetIPs detects the IP address by pretending to send an UDP packet.
// (No actual UDP packets will be sent out.)
func (p LocalAuto) GetIPs(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) ([]netip.Addr, bool) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, ipNet.UDPNetwork(), p.RemoteUDPAddr)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to detect a local %s address: %v", ipNet.Describe(), err)
		return nil, false
	}
	defer conn.Close()

	ip, ok := ExtractUDPAddr(ppfmt, conn.LocalAddr())
	if !ok {
		return nil, false
	}

	normalizedIP, ok := ipNet.NormalizeDetectedIP(ppfmt, ip)
	if !ok {
		return nil, false
	}

	return []netip.Addr{normalizedIP}, true
}
