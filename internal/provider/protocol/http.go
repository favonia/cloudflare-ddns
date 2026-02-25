package protocol

import (
	"context"
	"net/http"
	"net/netip"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromHTTP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, url string) (netip.Addr, bool) {
	c := httpCore{
		ipNet:             ipNet,
		url:               url,
		method:            http.MethodGet,
		additionalHeaders: nil,
		requestBody:       nil,
		extract: func(_ pp.PP, body []byte) (netip.Addr, bool) {
			ipString := strings.TrimSpace(string(body))
			ip, err := netip.ParseAddr(ipString)
			if err != nil {
				ppfmt.Noticef(pp.EmojiError, `Failed to parse the IP address in the response of %q (%q)`, url, ipString)
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
	URL          map[ipnet.Type]string // URL of the page for detection
}

// Name of the detection protocol.
func (p HTTP) Name() string {
	return p.ProviderName
}

// GetIPs detects the IP address by using the HTTP response directly.
func (p HTTP) GetIPs(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) ([]netip.Addr, bool) {
	url, found := p.URL[ipNet]
	if !found {
		ppfmt.Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return nil, false
	}

	ip, ok := getIPFromHTTP(ctx, ppfmt, ipNet, url)
	if !ok {
		return nil, false
	}

	return ipNet.NormalizeDetectedIPs(ppfmt, []netip.Addr{ip})
}
