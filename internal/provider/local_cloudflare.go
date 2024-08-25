package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewLocal creates a specialized Local provider that uses Cloudflare as the remote server.
// (No actual UDP packets will be sent to Cloudflare.)
func NewLocal() Provider {
	return protocol.Local{
		ProviderName: "local",
		RemoteUDPAddr: map[ipnet.Type]string{
			// 1.0.0.1 is used in case 1.1.1.1 is hijacked by the router
			ipnet.IP4: "1.0.0.1:443",
			ipnet.IP6: "[2606:4700:4700::1111]:443",
		},
	}
}
