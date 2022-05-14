package provider

import (
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func NormalizeIP(ppfmt pp.PP, ipNet ipnet.Type, ip netip.Addr) netip.Addr {
	var invalidIP netip.Addr

	if !ip.IsValid() {
		return invalidIP
	}

	ip, ok := ipNet.NormalizeIP(ip)
	if !ok {
		ppfmt.Warningf(pp.EmojiError, "%q is not a valid %s address", ip, ipNet.Describe())
		return invalidIP
	}

	return ip
}
