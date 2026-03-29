package provider

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareTrace creates a specialized CloudflareTrace provider.
// It parses https://api.cloudflare.com/cdn-cgi/trace.
func NewCloudflareTrace() Provider {
	return protocol.CloudflareTrace{
		ProviderName: "cloudflare.trace",
		URL: map[ipnet.Family]string{
			ipnet.IP4: "https://api.cloudflare.com/cdn-cgi/trace",
			ipnet.IP6: "https://api.cloudflare.com/cdn-cgi/trace",
		},
	}
}

// NewCloudflareTraceCustom creates a specialized CloudflareTrace provider
// with a specific URL.
func NewCloudflareTraceCustom(ppfmt pp.PP, envKey string, url string) (Provider, bool) {
	if strings.TrimSpace(url) == "" {
		ppfmt.Noticef(
			pp.EmojiUserError,
			`%s=cloudflare.trace: must be followed by a URL`,
			envKey,
		)
		return nil, false
	}

	return protocol.CloudflareTrace{
		ProviderName: "cloudflare.trace",
		URL: map[ipnet.Family]string{
			ipnet.IP4: url,
			ipnet.IP6: url,
		},
	}, true
}

// MustNewCloudflareTraceCustom creates a CloudflareTrace provider and panics if it fails.
func MustNewCloudflareTraceCustom(url string) Provider {
	var buf strings.Builder
	p, ok := NewCloudflareTraceCustom(pp.NewDefault(&buf), "IP_PROVIDER", url)
	if !ok {
		panic(buf.String())
	}
	return p
}
