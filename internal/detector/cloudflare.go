package detector

import (
	"context"
	"net"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

type Cloudflare struct {
	Net ipnet.Type
}

func (p *Cloudflare) IsManaged() bool {
	return true
}

func (p *Cloudflare) String() string {
	return "cloudflare"
}

func (p *Cloudflare) getIP4(ctx context.Context) (net.IP, bool) {
	ip, ok := getIPFromDNS(ctx, "https://1.1.1.1/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS)
	if !ok {
		return nil, false
	}

	return ip.To4(), true
}

func (p *Cloudflare) getIP6(ctx context.Context) (net.IP, bool) {
	ip, ok := getIPFromDNS(ctx, "https://[2606:4700:4700::1111]/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS)
	if !ok {
		return nil, false
	}

	return ip.To16(), true
}

func (p *Cloudflare) GetIP(ctx context.Context) (net.IP, bool) {
	switch p.Net {
	case ipnet.IP4:
		return p.getIP4(ctx)
	case ipnet.IP6:
		return p.getIP6(ctx)
	default:
		return nil, false
	}
}
