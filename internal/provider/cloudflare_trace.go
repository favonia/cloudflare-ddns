package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareTrace creates a specialized CloudflareTrace provider.
func NewCloudflareTrace() Provider {
	return protocol.CloudflareTrace{
		ProviderName: "cloudflare.trace",
		URLs: map[ipnet.Family][]string{
			ipnet.IP4: {
				"https://api.cloudflare.com/cdn-cgi/trace",
				"https://www.cloudflare.com/cdn-cgi/trace",
				"https://connectivity.cloudflareclient.com/cdn-cgi/trace",
			},
			ipnet.IP6: {
				"https://api.cloudflare.com/cdn-cgi/trace",
				"https://www.cloudflare.com/cdn-cgi/trace",
				"https://connectivity.cloudflareclient.com/cdn-cgi/trace",
			},
		},
	}
}
