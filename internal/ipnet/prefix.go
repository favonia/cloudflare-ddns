package ipnet

import (
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ParsePrefixOrIP parses a prefix or an IP.
func ParsePrefixOrIP(ppfmt pp.PP, s string) (netip.Prefix, bool) {
	p, errPrefix := netip.ParsePrefix(s)
	if errPrefix != nil {
		ip, errAddr := netip.ParseAddr(s)
		if errAddr != nil {
			// The context is an IP list from Cloudflare. Theoretically, it's impossible to have
			// invalid IP ranges/addresses.
			ppfmt.Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP range: %v", s, errPrefix)
			ppfmt.Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP address as well: %v", s, errAddr)
			return netip.Prefix{}, false
		}
		p = netip.PrefixFrom(ip, ip.BitLen())
	}
	return p, true
}

// DescribePrefixOrIP is similar to [netip.Prefix.String] but prints out
// the IP directly if the input range only contains one IP.
func DescribePrefixOrIP(p netip.Prefix) string {
	if p.IsSingleIP() {
		return p.Addr().String()
	} else {
		return p.Masked().String()
	}
}
