package detector

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

type Local struct{}

func (p *Local) IsManaged() bool {
	return true
}

func (p *Local) String() string {
	return "local"
}

func (p *Local) getIP4(_ context.Context, indent pp.Indent) (net.IP, bool) {
	conn, err := net.Dial("udp4", "1.1.1.1:443")
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to detect a local IPv4 address: %v", err)
		return nil, false
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.To4(), true
}

func (p *Local) getIP6(_ context.Context, indent pp.Indent) (net.IP, bool) {
	conn, err := net.Dial("udp6", "[2606:4700:4700::1111]:443")
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to detect a local IPv6 address: %v", err)
		return nil, false
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.To16(), true
}

func (p *Local) GetIP(ctx context.Context, indent pp.Indent, ipNet ipnet.Type) (net.IP, bool) {
	switch ipNet {
	case ipnet.IP4:
		return p.getIP4(ctx, indent)
	case ipnet.IP6:
		return p.getIP6(ctx, indent)
	default:
		return nil, false
	}
}
