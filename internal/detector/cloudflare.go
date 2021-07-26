package detector

import (
	"context"
	"net"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

type Cloudflare struct{}

func (p *Cloudflare) IsManaged() bool {
	return true
}

func (p *Cloudflare) String() string {
	return "cloudflare"
}

func (p *Cloudflare) getIP4(ctx context.Context, indent pp.Indent) (net.IP, bool) {
	ip, ok := getIPFromDNS(ctx, indent, "https://1.1.1.1/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS)
	if !ok {
		return nil, false
	}

	return ip.To4(), true
}

func (p *Cloudflare) getIP6(ctx context.Context, indent pp.Indent) (net.IP, bool) {
	ip, ok := getIPFromDNS(ctx, indent,
		"https://[2606:4700:4700::1111]/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS)
	if !ok {
		return nil, false
	}

	return ip.To16(), true
}

func (p *Cloudflare) GetIP(ctx context.Context, indent pp.Indent, ipNet ipnet.Type) (net.IP, bool) {
	switch ipNet {
	case ipnet.IP4:
		return p.getIP4(ctx, indent)
	case ipnet.IP6:
		return p.getIP6(ctx, indent)
	default:
		return nil, false
	}
}
