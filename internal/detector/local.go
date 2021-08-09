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
		pp.Printf(indent, pp.EmojiError, "Failed to detect a local %s address: %v", ipNet.String(), err)
		return nil
	}
	defer conn.Close()

	return ipNet.NormalizeIP(conn.LocalAddr().(*net.UDPAddr).IP)
}

func NewLocal() Policy {
	return &Local{
		PolicyName: "local",
		RemoteUDPAddr: map[ipnet.Type]string{
			ipnet.IP4: "1.1.1.1:443",
			ipnet.IP6: "[2606:4700:4700::1111]:443",
		},
	}
}
