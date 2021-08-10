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

func (p *Local) IsManaged() bool {
	return true
}

func (p *Local) String() string {
	return p.PolicyName
}

func (p *Local) GetIP(ctx context.Context, indent pp.Indent, ipNet ipnet.Type) net.IP {
	remoteUDPAddr, found := p.RemoteUDPAddr[ipNet]
	if !found {
		return nil
	}

	conn, err := net.Dial(ipNet.UDPNetwork(), remoteUDPAddr)
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to detect a local %s address: %v", ipNet.Describe(), err)
		return nil
	}
	defer conn.Close()

	return ipNet.NormalizeIP(conn.LocalAddr().(*net.UDPAddr).IP)
}
