package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareTrace creates a specialized CloudflareTrace provider that parses https://1.1.1.1/cdn-cgi/trace.
func NewCloudflareTrace() Provider {
	return &protocol.Field{
		ProviderName: "cloudflare.trace",
		Param: map[ipnet.Type]struct {
			URL   string
			Field string
		}{
			ipnet.IP4: {"https://cloudflare-dns.com/cdn-cgi/trace", "ip"},
			ipnet.IP6: {"https://cloudflare-dns.com/cdn-cgi/trace", "ip"},
		},
	}
}
