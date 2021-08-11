package detector

import (
	"net"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func NormalizeIP(ppfmt pp.Fmt, ipNet ipnet.Type, ip net.IP) net.IP {
	if ip == nil {
		return nil
	}

	val := ipNet.NormalizeIP(ip)
	if val == nil {
		ppfmt.Warningf(pp.EmojiError, "%q is not a valid %s address", ip, ipNet.Describe())
		return nil
	}

	return val
}
