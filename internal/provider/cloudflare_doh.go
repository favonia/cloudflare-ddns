package provider

import (
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareDOH creates a new provider that queries whoami.cloudflare.
// via Cloudflare DNS over HTTPS at https://cloudflare-dns.com/dns-query.
func NewCloudflareDOH() Provider {
	return protocol.DNSOverHTTPS{
		ProviderName: "cloudflare.doh",
		Param: map[ipnet.Type]protocol.DNSOverHTTPSParam{
			ipnet.IP4: {
				"https://cloudflare-dns.com/dns-query",
				"whoami.cloudflare.", dnsmessage.ClassCHAOS,
			},
			ipnet.IP6: {
				"https://cloudflare-dns.com/dns-query",
				"whoami.cloudflare.", dnsmessage.ClassCHAOS,
			},
		},
	}
}
