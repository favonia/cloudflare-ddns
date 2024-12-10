package provider

import (
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareDOH creates a new provider that queries whoami.cloudflare. via Cloudflare DNS over HTTPS.
// If use1001 is true, 1.0.0.1 is used instead of 1.1.1.1.
func NewCloudflareDOH() Provider {
	return NewHappyEyeballs(protocol.DNSOverHTTPS{
		ProviderName: "cloudflare.doh",
		Param: map[ipnet.Type]protocol.DNSOverHTTPSParam{
			ipnet.IP4: {
				protocol.Constant("https://cloudflare-dns.com/dns-query"),
				"whoami.cloudflare.", dnsmessage.ClassCHAOS,
			},
			ipnet.IP6: {
				protocol.Constant("https://cloudflare-dns.com/dns-query"),
				"whoami.cloudflare.", dnsmessage.ClassCHAOS,
			},
		},
	})
}
