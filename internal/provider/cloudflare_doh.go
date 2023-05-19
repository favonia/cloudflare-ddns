package provider

import (
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCloudflareDOH creates a new provider that queries whoami.cloudflare. via Cloudflare DNS over HTTPS.
// If use1001 is true, 1.0.0.1 is used instead of 1.1.1.1.
func NewCloudflareDOH(use1001 bool) Provider {
	ip4URL := "https://1.1.1.1/dns-query"
	if use1001 {
		ip4URL = "https://1.0.0.1/dns-query"
	}

	return &protocol.DNSOverHTTPS{
		ProviderName: "cloudflare.doh",
		Param: map[ipnet.Type]struct {
			URL   string
			Name  string
			Class dnsmessage.Class
		}{
			ipnet.IP4: {ip4URL, "whoami.cloudflare.", dnsmessage.ClassCHAOS},
			ipnet.IP6: {"https://[2606:4700:4700::1111]/dns-query", "whoami.cloudflare.", dnsmessage.ClassCHAOS},
		},
	}
}
