package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewLocal creates a specialized Local provider that uses Cloudflare as the remote server.
// (No actual UDP packets will be sent to Cloudflare.)
func NewLocal() Provider {
	return &protocol.Local{
		ProviderName:     "local",
		Is1111UsedForIP4: false, // 1.0.0.1 is used in case 1.1.1.1 is hijacked by the router
		RemoteUDPAddr: map[ipnet.Type]protocol.Switch{
			// 1.0.0.1 is used in case 1.1.1.1 is hijacked by the router
			ipnet.IP4: protocol.Constant("1.0.0.1:443"),
			ipnet.IP6: protocol.Constant("[2606:4700:4700::1111]:443"),
		},
	}
}
