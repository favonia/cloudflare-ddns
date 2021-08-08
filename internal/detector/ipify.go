package detector

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func NewIpify() Policy {
	return &HTTP{
		policyName: "ipify",
		url: map[ipnet.Type]string{
			ipnet.IP4: "https://api4.ipify.org",
			ipnet.IP6: "https://api6.ipify.org",
		},
	}
}
