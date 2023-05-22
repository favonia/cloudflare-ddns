package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewLocal creates a specialized Local provider that uses Cloudflare as the remote server.
// (No actual UDP packets will be sent out.)
func NewLocal() Provider {
	return &protocol.Local{
		ProviderName: "local",
		RemoteUDPAddr: map[ipnet.Type]protocol.Switch{
			ipnet.IP4: protocol.Switchable{
				Use1111: "1.1.1.1:443",
				Use1001: "1.0.0.1:443",
			},
			ipnet.IP6: protocol.Constant("[2606:4700:4700::1111]:443"),
		},
	}
}
