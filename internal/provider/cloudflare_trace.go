package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareTrace creates a specialized CloudflareTrace provider.
// It parses https://api.cloudflare.com/cdn-cgi/trace.
func NewCloudflareTrace() Provider {
	return NewCloudflareTraceCustom("https://api.cloudflare.com/cdn-cgi/trace")
}

// NewCloudflareTraceCustom creates a specialized CloudflareTrace provider
// with a specific URL.
func NewCloudflareTraceCustom(url string) Provider {
	return protocol.CloudflareTrace{
		ProviderName: "cloudflare.trace",
		URL: map[ipnet.Family]string{
			ipnet.IP4: url,
			ipnet.IP6: url,
		},
	}
}
