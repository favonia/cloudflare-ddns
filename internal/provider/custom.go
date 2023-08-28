package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCustom creates a HTTP provider.
func NewCustom(url string) Provider {
	return &protocol.HTTP{
		ProviderName:     "custom",
		Is1111UsedForIP4: false,
		URL: map[ipnet.Type]protocol.Switch{
			ipnet.IP4: protocol.Constant(url),
			ipnet.IP6: protocol.Constant(url),
		},
	}
}
