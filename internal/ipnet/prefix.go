package ipnet

import (
	"net/netip"
	"slices"

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

// LiftValidatedIPsToPrefixes preserves the observed address bits and applies
// the given prefix length to each already-validated address.
func LiftValidatedIPsToPrefixes(ips []netip.Addr, prefixLen int) []netip.Prefix {
	if len(ips) == 0 {
		return nil
	}

	prefixes := make([]netip.Prefix, 0, len(ips))
	for _, ip := range ips {
		prefixes = append(prefixes, netip.PrefixFrom(ip, prefixLen))
	}
	return prefixes
}

// normalizeDetectedPrefix normalizes a detected raw-data CIDR into the
// requested family while preserving host bits.
func normalizeDetectedPrefix(t Family, ppfmt pp.PP, prefix netip.Prefix) (netip.Prefix, bool) {
	if !prefix.IsValid() {
		ppfmt.Noticef(pp.EmojiImpossible,
			`Detected IP prefix is not valid; this should not happen and please report it at %s`,
			pp.IssueReportingURL,
		)
		return netip.Prefix{}, false
	}

	addr := prefix.Addr()
	bits := prefix.Bits()

	switch t {
	case IP4:
		switch {
		case addr.Is4():
		case addr.Is4In6():
			// Inspired by RFC 6887's PCP FILTER semantics: when an IPv4 prefix
			// is encoded in the ::ffff:0:0/96 mapped form, the encoded prefix
			// length is the IPv4 prefix length plus the fixed 96-bit mapping
			// prefix. We reuse that arithmetic here for canonicalization.
			if bits < 96 {
				ppfmt.Noticef(pp.EmojiError,
					"Detected IP prefix %s is an IPv4-mapped IPv6 prefix with a prefix length shorter than /96; it can't be used",
					prefix.String(),
				)
				return netip.Prefix{}, false
			}
			addr = addr.Unmap()
			bits -= 96
		default:
			ppfmt.Noticef(pp.EmojiError,
				"Detected IP prefix %s is not a valid IPv4 prefix; it can't be used",
				prefix.String(),
			)
			return netip.Prefix{}, false
		}
	case IP6:
		if !addr.Is6() {
			ppfmt.Noticef(pp.EmojiError,
				"Detected IP prefix %s is not a valid IPv6 prefix; it can't be used",
				prefix.String(),
			)
			return netip.Prefix{}, false
		}
		if addr.Is4In6() {
			ppfmt.Noticef(pp.EmojiError,
				"Detected IP prefix %s is an IPv4-mapped IPv6 prefix; it can't be used",
				prefix.String(),
			)
			ppfmt.InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
				"An IPv4-mapped IPv6 address is an IPv4 address in disguise. "+
					"It cannot be used for routing IPv6 traffic. "+
					"If you need to use it for DNS, please open an issue at %s",
				pp.IssueReportingURL)
			return netip.Prefix{}, false
		}
	default:
		return netip.Prefix{}, false
	}

	normalized := netip.PrefixFrom(addr, bits)
	switch desc, disposition := checkDetectedAddr(addr); disposition {
	default:
		fallthrough
	case detectedAddrOK:
		return normalized, true
	case detectedAddrReject:
		ppfmt.Noticef(pp.EmojiError,
			"Detected %s prefix %s is %s",
			t.Describe(), prefix.String(), desc,
		)
		return netip.Prefix{}, false
	case detectedAddrWarnNonGlobalUnicast:
		ppfmt.Noticef(
			pp.EmojiWarning,
			`Detected %s prefix %s does not look like a global unicast prefix; still using it`,
			t.Describe(), prefix.String(),
		)
		return normalized, true
	}
}

// NormalizeDetectedPrefixes normalizes a list of detected raw-data CIDRs while
// preserving host bits in the address portion.
//
// Behavior:
// - fail-fast: return false on the first invalid prefix
// - preserve emptiness: empty input returns empty output
// - canonicalize set semantics: output is sorted and deduplicated.
func (t Family) NormalizeDetectedPrefixes(ppfmt pp.PP, prefixes []netip.Prefix) ([]netip.Prefix, bool) {
	if len(prefixes) == 0 {
		return prefixes, true
	}

	normalized := make([]netip.Prefix, 0, len(prefixes))
	for _, prefix := range prefixes {
		prefix, ok := normalizeDetectedPrefix(t, ppfmt, prefix)
		if !ok {
			return nil, false
		}
		normalized = append(normalized, prefix)
	}

	slices.SortFunc(normalized, netip.Prefix.Compare)
	normalized = slices.Compact(normalized)
	return normalized, true
}
