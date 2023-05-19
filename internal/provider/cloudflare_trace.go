package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareTrace creates a specialized CloudflareTrace provider that parses https://1.1.1.1/cdn-cgi/trace.
// If use1001 is true, 1.0.0.1 is used instead of 1.1.1.1.
func NewCloudflareTrace(use1001 bool) Provider {
	ip4URL := "https://1.1.1.1/cdn-cgi/trace"
	if use1001 {
		ip4URL = "https://1.0.0.1/cdn-cgi/trace"
	}

	return &protocol.Field{
		ProviderName: "cloudflare.trace",
		Param: map[ipnet.Type]struct {
			URL   string
			Field string
		}{
			ipnet.IP4: {ip4URL, "ip"},
			ipnet.IP6: {"https://[2606:4700:4700::1111]/cdn-cgi/trace", "ip"},
		},
	}
}
