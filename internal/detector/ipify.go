package detector

import (
	"context"
	"net"
	"net/http"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromHTTP(ctx context.Context, indent pp.Indent, url string) (net.IP, bool) {
	c := httpConn{
		method:  http.MethodGet,
		url:     url,
		reader:  nil,
		prepare: func(_ pp.Indent, _ *http.Request) bool { return true },
		extract: func(_ pp.Indent, body []byte) (string, bool) { return string(body), true },
	}

	return c.getIP(ctx, indent)
}

type Ipify struct{}

func (p *Ipify) IsManaged() bool {
	return true
}

func (p *Ipify) String() string {
	return "ipify"
}

func (p *Ipify) getIP4(ctx context.Context, indent pp.Indent) (net.IP, bool) {
	ip, ok := getIPFromHTTP(ctx, indent, "https://api4.ipify.org")
	if !ok {
		return nil, false
	}

	return ip.To4(), true
}

func (p *Ipify) getIP6(ctx context.Context, indent pp.Indent) (net.IP, bool) {
	ip, ok := getIPFromHTTP(ctx, indent, "https://api6.ipify.org")
	if !ok {
		return nil, false
	}

	return ip.To16(), true
}

func (p *Ipify) GetIP(ctx context.Context, indent pp.Indent, ipNet ipnet.Type) (net.IP, bool) {
	switch ipNet {
	case ipnet.IP4:
		return p.getIP4(ctx, indent)
	case ipnet.IP6:
		return p.getIP6(ctx, indent)
	default:
		return nil, false
	}
}
