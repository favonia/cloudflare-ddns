package detector

import (
	"context"
	"net"

	"golang.org/x/net/dns/dnsmessage"
)

type Cloudflare struct{}

func (p *Cloudflare) IsManaged() bool {
	return true
}

func (p *Cloudflare) String() string {
	return "cloudflare"
}

func (p *Cloudflare) GetIP4(ctx context.Context) (net.IP, bool) {
	ip, ok := getIPFromDNS(ctx, "https://1.1.1.1/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS)
	if !ok {
		return nil, false
	}
	return ip.To4(), true
}

func (p *Cloudflare) GetIP6(ctx context.Context) (net.IP, bool) {
	ip, ok := getIPFromDNS(ctx, "https://[2606:4700:4700::1111]/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS)
	if !ok {
		return nil, false
	}
	return ip.To16(), true
}
