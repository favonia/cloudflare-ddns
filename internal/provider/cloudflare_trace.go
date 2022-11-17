package provider

import (
	"context"
	"net/http"
	"net/netip"
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromCloudflareTrace(ctx context.Context, ppfmt pp.PP, url string, field string) (netip.Addr, bool) {
	c := httpConn{
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

type CloudflareTrace struct {
	ProviderName string
	Param        map[ipnet.Type]struct {
		URL   string
		Field string
	}
}

func NewCloudflareTrace() Provider {
	return &CloudflareTrace{
		ProviderName: "cloudflare.trace",
		Param: map[ipnet.Type]struct {
			URL   string
			Field string
		}{
			ipnet.IP4: {"https://1.1.1.1/cdn-cgi/trace", "ip"},
			ipnet.IP6: {"https://[2606:4700:4700::1111]/cdn-cgi/trace", "ip"},
		},
	}
}

func (p *CloudflareTrace) Name() string {
	return p.ProviderName
}

func (p *CloudflareTrace) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, bool) {
	param, found := p.Param[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return netip.Addr{}, false
	}

	ip, ok := getIPFromCloudflareTrace(ctx, ppfmt, param.URL, param.Field)
	if !ok {
		return netip.Addr{}, false
	}

	return ipNet.NormalizeIP(ppfmt, ip)
}
