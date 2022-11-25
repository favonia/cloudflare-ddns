package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// NewIpify creates a specialized HTTP provider that uses the ipify service.
func NewIpify() Provider {
	return &HTTP{
		ProviderName: "ipify",
		URL: map[ipnet.Type]string{
			ipnet.IP4: "https://api4.ipify.org",
			ipnet.IP6: "https://api6.ipify.org",
		},
	}
}
