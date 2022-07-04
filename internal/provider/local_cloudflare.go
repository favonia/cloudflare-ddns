package provider

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func NewLocal() Provider {
	return &Local{
		ProviderName: "local",
		RemoteUDPAddr: map[ipnet.Type]string{
			ipnet.IP4: "1.1.1.1:443",
			ipnet.IP6: "[2606:4700:4700::1111]:443",
		},
	}
}
