package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareTrace creates a specialized CloudflareTrace provider that parses https://1.1.1.1/cdn-cgi/trace.
// If use1001 is true, 1.0.0.1 is used instead of 1.1.1.1.
func NewCloudflareTrace() Provider {
	return &protocol.Field{
		ProviderName:     "cloudflare.trace",
		Is1111UsedforIP4: true,
		Param: map[ipnet.Type]struct {
			URL   protocol.Switch
			Field string
		}{
			ipnet.IP4: {
				protocol.Switchable{
					Use1111: "https://1.1.1.1/cdn-cgi/trace",
					Use1001: "https://1.0.0.1/cdn-cgi/trace",
				},
				"ip",
			},
			ipnet.IP6: {
				protocol.Constant("https://[2606:4700:4700::1111]/cdn-cgi/trace"),
				"ip",
			},
		},
	}
}
