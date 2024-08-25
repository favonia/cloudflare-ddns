package provider

import (
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

var fieldIP = regexp.MustCompile(`(?m:^ip=(.*)$)`)

// NewCloudflareTrace creates a specialized CloudflareTrace provider that parses https://1.1.1.1/cdn-cgi/trace.
// If use1001 is true, 1.0.0.1 is used instead of 1.1.1.1.
func NewCloudflareTrace() Provider {
	return NewHappyEyeballs(protocol.Regexp{
		ProviderName: "cloudflare.trace",
		Param: map[ipnet.Type]protocol.RegexpParam{
			ipnet.IP4: {
				protocol.Switchable{
					Primary:     "https://1.1.1.1/cdn-cgi/trace",
					Alternative: "https://1.0.0.1/cdn-cgi/trace",
				},
				fieldIP,
			},
			ipnet.IP6: {
				protocol.Constant("https://[2606:4700:4700::1111]/cdn-cgi/trace"),
				fieldIP,
			},
		},
	})
}
