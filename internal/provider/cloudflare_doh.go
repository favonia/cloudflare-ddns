package provider

import (
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareDOH creates a new provider that queries whoami.cloudflare. via Cloudflare DNS over HTTPS.
// If use1001 is true, 1.0.0.1 is used instead of 1.1.1.1.
func NewCloudflareDOH() Provider {
	return &protocol.DNSOverHTTPS{
		ProviderName:     "cloudflare.doh",
		Is1111UsedForIP4: true,
		Param: map[ipnet.Type]struct {
			URL   protocol.Switch
			Name  string
			Class dnsmessage.Class
		}{
			ipnet.IP4: {
				protocol.Switchable{
					ValueFor1111: "https://1.1.1.1/dns-query",
					ValueFor1001: "https://1.0.0.1/dns-query",
				},
				"whoami.cloudflare.", dnsmessage.ClassCHAOS,
			},
			ipnet.IP6: {
				protocol.Constant("https://[2606:4700:4700::1111]/dns-query"),
				"whoami.cloudflare.", dnsmessage.ClassCHAOS,
			},
		},
	}
}
