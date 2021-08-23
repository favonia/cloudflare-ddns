package detector

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Local struct {
	PolicyName    string
	RemoteUDPAddr map[ipnet.Type]string
}

func (p *Local) name() string {
	return p.PolicyName
}

func (p *Local) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) net.IP {
	remoteUDPAddr, found := p.RemoteUDPAddr[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return nil
	}

	conn, err := net.Dial(ipNet.UDPNetwork(), remoteUDPAddr)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to detect a local %s address: %v", ipNet.Describe(), err)
		return nil
	}
	defer conn.Close()

	return NormalizeIP(ppfmt, ipNet, conn.LocalAddr().(*net.UDPAddr).IP)
}
