package protocol

import (
	"context"
	"net/http"
	"net/netip"
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromRegexp(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, url string, re *regexp.Regexp,
) (netip.Addr, bool) {
	c := httpCore{
		ipFamily:          ipFamily,
		url:               url,
		method:            http.MethodGet,
		additionalHeaders: nil,
		requestBody:       nil,
		extract: func(ppfmt pp.PP, body []byte) (netip.Addr, bool) {
			var invalidIP netip.Addr

			matched := re.FindSubmatch(body)
			if len(matched) < 2 {
				ppfmt.Noticef(pp.EmojiError, `Failed to find the IP address in the response of %q (%q)`, url, body)
				return invalidIP, false
			}
			ipString := string(matched[1])
			ip, err := netip.ParseAddr(ipString)
			if err != nil {
				ppfmt.Noticef(pp.EmojiError, `Failed to parse the IP address in the response of %q (%q)`, url, ipString)
				return invalidIP, false
			}
			return ip, true
		},
	}

	return c.getIP(ctx, ppfmt)
}

// RegexpParam is the type of parameters for the Regexp provider for a specific IP family.
type RegexpParam = struct {
	URL    string         // URL of the detection page
	Regexp *regexp.Regexp // regular expression to match the IP address
}

// Regexp represents a generic detection protocol to parse an HTTP response.
type Regexp struct {
	ProviderName string // name of the detection protocol
	Param        map[ipnet.Family]RegexpParam
}

// Name of the detection protocol.
func (p Regexp) Name() string { return p.ProviderName }

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (Regexp) IsExplicitEmpty() bool { return false }

// GetRawData detects the IP address by parsing the HTTP response.
func (p Regexp) GetRawData(
	ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	param, found := p.Param[ipFamily]
	if !found {
		ppfmt.Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", ipFamily.Describe())
		return NewUnavailableDetectionResult()
	}

	ip, ok := getIPFromRegexp(ctx, ppfmt, ipFamily, param.URL, param.Regexp)
	if !ok {
		return NewUnavailableDetectionResult()
	}

	cidrs, ok := NormalizeDetectedRawData(ppfmt, ipFamily, defaultPrefixLen, []netip.Addr{ip})
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(cidrs)
}
