package provider

import (
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// NewCloudflareDOH creates a new provider that queries whoami.cloudflare. via Cloudflare DNS over HTTPS.
func NewCloudflareDOH() Provider {
	return &DNSOverHTTPS{
		ProviderName: "cloudflare.doh",
		Param: map[ipnet.Type]struct {
			URL   string
			Name  string
			Class dnsmessage.Class
		}{
			ipnet.IP4: {"https://1.1.1.1/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS},
			ipnet.IP6: {"https://[2606:4700:4700::1111]/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS},
		},
	}
}
