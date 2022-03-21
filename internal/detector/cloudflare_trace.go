package detector

import (
	"context"
	"net/http"
	"net/netip"
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromCloudflareTrace(ctx context.Context, ppfmt pp.PP, url string, field string) netip.Addr {
	c := httpConn{
		url:         url,
		method:      http.MethodGet,
		contentType: "",
		accept:      "",
		reader:      nil,
		extract: func(ppfmt pp.PP, body []byte) netip.Addr {
			var invalidIP netip.Addr

			re := regexp.MustCompile(`(?m:^` + regexp.QuoteMeta(field) + `=(.*)$)`)
			matched := re.FindSubmatch(body)
			if matched == nil {
				ppfmt.Errorf(pp.EmojiImpossible, `Failed to find the IP address in the response of %q: %s`, url, body)
				return invalidIP
			}
			ipString := string(matched[1])
			ip, err := netip.ParseAddr(ipString)
			if err != nil {
				ppfmt.Errorf(pp.EmojiImpossible, `Failed to parse the IP address in the response of %q: %s`, url, ipString)
				return invalidIP
			}
			return ip
		},
	}

	return c.getIP(ctx, ppfmt)
}

type CloudflareTrace struct {
	PolicyName string
	Param      map[ipnet.Type]struct {
		URL   string
		Field string
	}
}

func NewCloudflareTrace() Policy {
	return &CloudflareTrace{
		PolicyName: "cloudflare.trace",
		Param: map[ipnet.Type]struct {
			URL   string
			Field string
		}{
			ipnet.IP4: {"https://1.1.1.1/cdn-cgi/trace", "ip"},
			ipnet.IP6: {"https://[2606:4700:4700::1111]/cdn-cgi/trace", "ip"},
		},
	}
}

func (p *CloudflareTrace) name() string {
	return p.PolicyName
}

func (p *CloudflareTrace) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) netip.Addr {
	param, found := p.Param[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return netip.Addr{}
	}

	return NormalizeIP(ppfmt, ipNet, getIPFromCloudflareTrace(ctx, ppfmt, param.URL, param.Field))
}
