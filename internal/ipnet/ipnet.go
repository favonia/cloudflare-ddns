// Package ipnet contains utility functions for IPv4 and IPv6 families.
package ipnet

import (
	"fmt"
	"iter"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Family identifies an IP family.
type Family int

const (
	// IP4 is IP version 4.
	IP4 Family = 4

	// IP6 is IP version 6.
	IP6 Family = 6

	// FamilyCount is the number of IP families.
	FamilyCount = 2
)

// Int returns the IP version. It is either 4 or 6.
func (t Family) Int() int {
	switch t {
	case IP4, IP6:
		return int(t)
	default:
		return 0
	}
}

// Describe returns a human-readable description of the IP family.
func (t Family) Describe() string {
	switch t {
	case IP4, IP6:
		return fmt.Sprintf("IPv%d", t)
	default:
		return ""
	}
}

// RecordType prints out the DNS record type for the IP family. For IPv4, it is A; for IPv6, it is AAAA.
func (t Family) RecordType() string {
	switch t {
	case IP4:
		return "A"
	case IP6:
		return "AAAA"
	default:
		return ""
	}
}

// UDPNetwork returns the net.Dial network name for this IP family.
func (t Family) UDPNetwork() string {
	switch t {
	case IP4:
		return "udp4"
	case IP6:
		return "udp6"
	default:
		return ""
	}
}

// Matches reports whether an IP belongs to this family.
func (t Family) Matches(ip netip.Addr) bool {
	ip = ip.Unmap()
	switch t {
	case IP4:
		return ip.Is4()
	case IP6:
		return ip.Is6()
	default:
		return false
	}
}

// All enumerates [IP4] and then [IP6].
func All(yield func(Family) bool) {
	_ = yield(IP4) && yield(IP6)
}

// Bindings enumerates the key [IP4] and then [IP6] for a map.
func Bindings[V any](m map[Family]V) iter.Seq2[Family, V] {
	return func(yield func(Family, V) bool) {
		for ipFamily := range All {
			v, ok := m[ipFamily]
			if ok {
				if !yield(ipFamily, v) {
					return
				}
			}
		}
	}
}

// DescribeBadAddress reports whether the address is unsuitable as a DNS/WAF target.
// If unsuitable, it returns a predicate phrase suitable for "(subject) %s"
// (e.g., "is a loopback address") and true.
// The caller is responsible for formatting the full message with context.
func DescribeBadAddress(ip netip.Addr) (string, bool) {
	switch {
	case ip.IsUnspecified():
		return "is an unspecified address", true
	case ip.IsLoopback():
		return "is a loopback address", true
	case ip.IsLinkLocalMulticast():
		return "is a link-local multicast address", true
	case ip.IsMulticast():
		return "is a multicast address", true
	case ip.IsLinkLocalUnicast():
		return "is a link-local address", true
	case ip.Zone() != "":
		return "is an address with a zone identifier", true
	case ip == netip.AddrFrom4([4]byte{255, 255, 255, 255}):
		return "is a broadcast address", true
	case !ip.IsGlobalUnicast():
		// Safety net: all known non-global-unicast cases are handled above.
		// If this fires, consider adding an explicit case.
		return "is not a global unicast address", true
	default:
		return "", false
	}
}

// ValidateAndNormalizeIP validates ip for ipFamily.
//   - On success, it returns the canonical normalized address, whether
//     normalization unmapped an IPv4-mapped IPv6 address (unmapped),
//     and ok=true.
//   - On failure, it returns an issue phrase suitable for "(subject) %s"
//     (for example, "is a loopback address") and ok=false. When wants4in6Hint
//     is true, callers should also show [pp.MessageIP4MappedIP6Address].
//
// The function does not emit messages; callers report errors in their own context.
func ValidateAndNormalizeIP(ipFamily Family, ip netip.Addr) (
	normalized netip.Addr, unmapped bool, issue string, wants4in6Hint bool, ok bool,
) {
	switch ipFamily {
	case IP4:
		if !ip.Is4() && !ip.Is4In6() {
			return netip.Addr{}, false, fmt.Sprintf("is not a valid %s address", ipFamily.Describe()), false, false
		}
		if ip.Is4In6() {
			// Turns an IPv4-mapped IPv6 address back to an IPv4 address.
			ip = ip.Unmap()
			unmapped = true
		}

	case IP6:
		if !ip.Is6() {
			return netip.Addr{}, false, fmt.Sprintf("is not a valid %s address", ipFamily.Describe()), false, false
		}
		if ip.Is4In6() {
			return netip.Addr{}, false, "is an IPv4-mapped IPv6 address", true, false
		}

	default:
		return netip.Addr{}, false, "is not in a recognized IP family", false, false
	}

	if desc, bad := DescribeBadAddress(ip); bad {
		return netip.Addr{}, false, desc, false, false
	}

	return ip, unmapped, "", false, true
}

// normalizeDetectedIP normalizes an IP into the requested family.
func normalizeDetectedIP(t Family, ppfmt pp.PP, ip netip.Addr) (netip.Addr, bool) {
	if !ip.IsValid() {
		ppfmt.Noticef(pp.EmojiImpossible,
			`Detected IP address is not valid; this should not happen and please report it at %s`,
			pp.IssueReportingURL,
		)
		return netip.Addr{}, false
	}

	normalized, _, issue, wants4in6Hint, ok := ValidateAndNormalizeIP(t, ip)
	if !ok {
		ppfmt.Noticef(pp.EmojiError, "Detected IP address %s %s", ip.String(), issue)
		if wants4in6Hint {
			ppfmt.InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
				"An IPv4-mapped IPv6 address is an IPv4 address in disguise. "+
					"It cannot be used for routing IPv6 traffic. "+
					"If you need to use it for DNS, please open an issue at %s",
				pp.IssueReportingURL)
		}
		return netip.Addr{}, false
	}

	return normalized, true
}

// NormalizeDetectedIPs normalizes a list of detected IPs.
//
// Behavior:
// - fail-fast: return false on the first invalid IP
// - preserve emptiness: empty input returns empty output
// - canonicalize set semantics: output is sorted and deduplicated.
func (t Family) NormalizeDetectedIPs(ppfmt pp.PP, ips []netip.Addr) ([]netip.Addr, bool) {
	if len(ips) == 0 {
		return ips, true
	}

	normalized := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		ip, ok := normalizeDetectedIP(t, ppfmt, ip)
		if !ok {
			return nil, false
		}
		normalized = append(normalized, ip)
	}

	slices.SortFunc(normalized, netip.Addr.Compare)
	normalized = slices.Compact(normalized)
	return normalized, true
}

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
