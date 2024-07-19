package protocol

import (
	"context"
	"net/http"
	"net/netip"
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromField(ctx context.Context, ppfmt pp.PP, url string, field string) (netip.Addr, bool) {
	c := httpCore{
		url:               url,
		method:            http.MethodGet,
		additionalHeaders: nil,
		requestBody:       nil,
		extract: func(ppfmt pp.PP, body []byte) (netip.Addr, bool) {
			var invalidIP netip.Addr

			re := regexp.MustCompile(`(?m:^` + regexp.QuoteMeta(field) + `=(.*)$)`)
			matched := re.FindSubmatch(body)
			if matched == nil {
				ppfmt.Warningf(pp.EmojiError, `Failed to find the IP address in the response of %q: %s`, url, body)
				return invalidIP, false
			}
			ipString := string(matched[1])
			ip, err := netip.ParseAddr(ipString)
			if err != nil {
				ppfmt.Warningf(pp.EmojiError, `Failed to parse the IP address in the response of %q: %s`, url, ipString)
				return invalidIP, false
			}
			return ip, true
		},
	}

	return c.getIP(ctx, ppfmt)
}

// Field represents a generic detection protocol to parse an HTTP response.
type Field struct {
	ProviderName     string // name of the detection protocol
	Is1111UsedforIP4 bool
	Param            map[ipnet.Type]struct {
		URL   Switch // URL of the detection page
		Field string // name of the field holding the IP address
	}
}

// Name of the detection protocol.
func (p Field) Name() string {
	return p.ProviderName
}

// GetIP detects the IP address by parsing the HTTP response.
func (p Field) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, use1001 bool) (netip.Addr, bool) {
	param, found := p.Param[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return netip.Addr{}, false
	}

	ip, ok := getIPFromField(ctx, ppfmt, param.URL.Switch(use1001), param.Field)
	if !ok {
		return netip.Addr{}, false
	}

	return ipNet.NormalizeDetectedIP(ppfmt, ip)
}

// ShouldWeCheck1111 returns whether we should check 1.1.1.1.
func (p Field) ShouldWeCheck1111() bool { return p.Is1111UsedforIP4 }
