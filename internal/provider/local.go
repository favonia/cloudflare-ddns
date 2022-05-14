package provider

import (
	"context"
	"net"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Local struct {
	ProviderName  string
	RemoteUDPAddr map[ipnet.Type]string
}

func (p *Local) Name() string {
	return p.ProviderName
}

func (p *Local) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) netip.Addr {
	var invalidIP netip.Addr

	remoteUDPAddr, found := p.RemoteUDPAddr[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return invalidIP
	}

	conn, err := net.Dial(ipNet.UDPNetwork(), remoteUDPAddr)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to detect a local %s address: %v", ipNet.Describe(), err)
		return invalidIP
	}
	defer conn.Close()

	ip := conn.LocalAddr().(*net.UDPAddr).AddrPort().Addr() //nolint:forcetypeassert

	return NormalizeIP(ppfmt, ipNet, ip)
}
