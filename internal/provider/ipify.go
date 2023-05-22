package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewIpify creates a specialized HTTP provider that uses the ipify service.
func NewIpify() Provider {
	return &protocol.HTTP{
		ProviderName: "ipify",
		URL: map[ipnet.Type]protocol.Switch{
			ipnet.IP4: protocol.Constant("https://api4.ipify.org"),
			ipnet.IP6: protocol.Constant("https://api6.ipify.org"),
		},
	}
}
