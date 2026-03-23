package protocol

import (
	"context"
	"net"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type udpDialContext func(context.Context, string, string) (net.Conn, error)

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

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (LocalAuto) IsExplicitEmpty() bool {
	return false
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

// GetRawData detects the IP address by pretending to send an UDP packet.
// (No actual UDP packets will be sent out.)
func (p LocalAuto) GetRawData(
	ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	return p.getRawDataWithDialContext(
		ctx,
		ppfmt,
		ipFamily,
		defaultPrefixLen,
		func(ctx context.Context, network, remoteUDPAddr string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, remoteUDPAddr)
		},
	)
}

func (p LocalAuto) getRawDataWithDialContext(
	ctx context.Context,
	ppfmt pp.PP,
	ipFamily ipnet.Family,
	defaultPrefixLen int,
	dialContext udpDialContext,
) DetectionResult {
	conn, err := dialContext(ctx, ipFamily.UDPNetwork(), p.RemoteUDPAddr)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to detect a local %s address: %v", ipFamily.Describe(), err)
		return NewUnavailableDetectionResult()
	}
	defer conn.Close()

	ip, ok := ExtractUDPAddr(ppfmt, conn.LocalAddr())
	if !ok {
		return NewUnavailableDetectionResult()
	}

	rawEntries, ok := NormalizeDetectedRawIPs(ppfmt, ipFamily, defaultPrefixLen, []netip.Addr{ip})
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(rawEntries)
}
