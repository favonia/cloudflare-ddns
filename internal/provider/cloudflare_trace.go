package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareTrace creates a specialized CloudflareTrace provider that parses https://1.1.1.1/cdn-cgi/trace.
func NewCloudflareTrace() Provider {
	ip4URL := "https://1.1.1.1/cdn-cgi/trace"
	if UseAlternativeCloudflareIPs {
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
