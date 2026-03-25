package ipnet

import (
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ParseAddrOrPrefix parses a network prefix or a bare IP address.
// This is used for parsing Cloudflare WAF list items, not raw detection data.
func ParseAddrOrPrefix(ppfmt pp.PP, s string) (netip.Prefix, bool) {
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
