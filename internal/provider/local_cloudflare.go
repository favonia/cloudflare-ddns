package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewLocal creates a specialized Local provider that uses Cloudflare as the remote server.
// (No actual UDP packets will be sent out.)
func NewLocal(useAlternativeIPs bool) Provider {
	ip4Host := "1.1.1.1:443"
	if useAlternativeIPs {
		ip4Host = "1.0.0.1:443"
	}

	return &protocol.Local{
		ProviderName: "local",
		RemoteUDPAddr: map[ipnet.Type]string{
			ipnet.IP4: ip4Host,
			ipnet.IP6: "[2606:4700:4700::1111]:443",
		},
	}
}
