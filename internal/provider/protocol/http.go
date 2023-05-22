package protocol

import (
	"context"
	"net/http"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromHTTP(ctx context.Context, ppfmt pp.PP, url string) (netip.Addr, bool) {
	c := httpCore{
		url:               url,
		method:            http.MethodGet,
		additionalHeaders: nil,
		requestBody:       nil,
		extract: func(_ pp.PP, body []byte) (netip.Addr, bool) {
			ipString := string(body)
			ip, err := netip.ParseAddr(ipString)
			if err != nil {
				ppfmt.Errorf(pp.EmojiImpossible, `Failed to parse the IP address in the response of %q: %s`, url, ipString)
				return netip.Addr{}, false
			}
			return ip, true
		},
	}

	return c.getIP(ctx, ppfmt)
}

// HTTP represents a generic detection protocol to use an HTTP response directly.
type HTTP struct {
	ProviderName string                // name of the protocol
	URL          map[ipnet.Type]Switch // URL of the detection page
}

// Name of the detection protocol.
func (p *HTTP) Name() string {
	return p.ProviderName
}

// GetIP detects the IP address by using the HTTP response directly.
func (p *HTTP) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, use1001 bool) (netip.Addr, bool) {
	url, found := p.URL[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return netip.Addr{}, false
	}

	ip, ok := getIPFromHTTP(ctx, ppfmt, url.Switch(use1001))
	if !ok {
		return netip.Addr{}, false
	}

	return ipNet.NormalizeDetectedIP(ppfmt, ip)
}
