package detector

import (
	"context"
	"net"
	"net/http"
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromCloudflareTrace(ctx context.Context, ppfmt pp.PP, url string, field string) net.IP {
	c := httpConn{
		url:         url,
		method:      http.MethodGet,
		contentType: "",
		accept:      "",
		reader:      nil,
		extract: func(ppfmt pp.PP, body []byte) net.IP {
			re := regexp.MustCompile(`(?m:^` + regexp.QuoteMeta(field) + `=(.*)$)`)
			matched := re.FindSubmatch(body)
			if matched == nil {
				ppfmt.Errorf(pp.EmojiImpossible, `Failed to find the IP address in the response of %q: %s`, url, body)
				return nil
			}
			return net.ParseIP(string(matched[1]))
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
		PolicyName: "cloudflare-trace",
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

func (p *CloudflareTrace) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) net.IP {
	param, found := p.Param[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return nil
	}

	return NormalizeIP(ppfmt, ipNet, getIPFromCloudflareTrace(ctx, ppfmt, param.URL, param.Field))
}
