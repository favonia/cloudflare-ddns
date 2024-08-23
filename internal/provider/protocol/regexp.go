package protocol

import (
	"context"
	"net/http"
	"net/netip"
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromRegexp(ctx context.Context, ppfmt pp.PP, url string, re *regexp.Regexp) (netip.Addr, bool) {
	c := httpCore{
		url:               url,
		method:            http.MethodGet,
		additionalHeaders: nil,
		requestBody:       nil,
		extract: func(ppfmt pp.PP, body []byte) (netip.Addr, bool) {
			var invalidIP netip.Addr

			matched := re.FindSubmatch(body)
			if len(matched) < 2 { //nolint:mnd
				ppfmt.Noticef(pp.EmojiError, `Failed to find the IP address in the response of %q: %s`, url, body)
				return invalidIP, false
			}
			ipString := string(matched[1])
			ip, err := netip.ParseAddr(ipString)
			if err != nil {
				ppfmt.Noticef(pp.EmojiError, `Failed to parse the IP address in the response of %q: %s`, url, ipString)
				return invalidIP, false
			}
			return ip, true
		},
	}

	return c.getIP(ctx, ppfmt)
}

// RegexpParam is the type of parameters for the Regexp provider for a specific IP network.
type RegexpParam = struct {
	URL    Switch         // URL of the detection page
	Regexp *regexp.Regexp // regular expression to match the IP address
}

// Regexp represents a generic detection protocol to parse an HTTP response.
type Regexp struct {
	ProviderName string // name of the detection protocol
	Param        map[ipnet.Type]RegexpParam
}

// Name of the detection protocol.
func (p Regexp) Name() string { return p.ProviderName }

// GetIP detects the IP address by parsing the HTTP response.
func (p Regexp) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, method Method) (netip.Addr, bool) {
	param, found := p.Param[ipNet]
	if !found {
		ppfmt.Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return netip.Addr{}, false
	}

	ip, ok := getIPFromRegexp(ctx, ppfmt, param.URL.Switch(method), param.Regexp)
	if !ok {
		return netip.Addr{}, false
	}

	return ipNet.NormalizeDetectedIP(ppfmt, ip)
}

// HasAlternative calls [Switch.HasAlternative].
func (p Regexp) HasAlternative(ipNet ipnet.Type) bool { return p.Param[ipNet].URL.HasAlternative() }
