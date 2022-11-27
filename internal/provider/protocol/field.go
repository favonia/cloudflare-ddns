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
		url:         url,
		method:      http.MethodGet,
		contentType: "",
		accept:      "",
		reader:      nil,
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
	ProviderName string // name of the detection protocol
	Param        map[ipnet.Type]struct {
		URL   string // URL of the detection page
		Field string // name of the field holding the IP address
	}
}

// Name of the detection protocol.
func (p *Field) Name() string {
	return p.ProviderName
}

// GetIP detects the IP address by parsing the HTTP response.
func (p *Field) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, bool) {
	param, found := p.Param[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return netip.Addr{}, false
	}

	ip, ok := getIPFromField(ctx, ppfmt, param.URL, param.Field)
	if !ok {
		return netip.Addr{}, false
	}

	return ipNet.NormalizeDetectedIP(ppfmt, ip)
}
