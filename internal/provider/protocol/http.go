package protocol

import (
	"context"
	"net/http"
	"net/netip"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getIPFromHTTP(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, url string) (netip.Addr, bool) {
	c := httpCore{
		ipFamily:          ipFamily,
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
	ProviderName            string                  // name of the protocol
	URL                     map[ipnet.Family]string // URL of the page for detection
	ForcedTransportIPFamily *ipnet.Family
	// ForcedTransportIPFamily optionally overrides the network family used for
	// the HTTP connection. When absent, GetIPs uses the requested family itself.
}

// Name of the detection protocol.
func (p HTTP) Name() string {
	return p.ProviderName
}

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (HTTP) IsExplicitEmpty() bool {
	return false
}

// GetRawData detects the IP address by using the HTTP response directly.
func (p HTTP) GetRawData(
	ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	url, found := p.URL[ipFamily]
	if !found {
		ppfmt.Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", ipFamily.Describe())
		return NewUnavailableDetectionResult()
	}

	transportIP := ipFamily
	if p.ForcedTransportIPFamily != nil {
		transportIP = *p.ForcedTransportIPFamily
	}

	ip, ok := getIPFromHTTP(ctx, ppfmt, transportIP, url)
	if !ok {
		return NewUnavailableDetectionResult()
	}

	rawEntries, ok := NormalizeDetectedRawIPs(ppfmt, ipFamily, defaultPrefixLen, []netip.Addr{ip})
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(rawEntries)
}
