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

// normalizeDetectedIP normalizes an IP into the requested family.
func normalizeDetectedIP(t Family, ppfmt pp.PP, ip netip.Addr) (netip.Addr, bool) {
	if !ip.IsValid() {
		ppfmt.Noticef(pp.EmojiImpossible,
			`Detected IP address is not valid; this should not happen and please report it at %s`,
			pp.IssueReportingURL,
		)
		return netip.Addr{}, false
	}

	switch t {
	case IP4:
		if !ip.Is4() && !ip.Is4In6() {
			ppfmt.Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", ip.String())
			return netip.Addr{}, false
		}
		// Turns an IPv4-mapped IPv6 address back to an IPv4 address
		ip = ip.Unmap()

	case IP6:
		// Accept only native IPv6 addresses and reject IPv4-mapped IPv6.
		if !ip.Is6() {
			ppfmt.Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv6 address", ip.String())
			return netip.Addr{}, false
		}
		if ip.Is4In6() {
			ppfmt.Noticef(pp.EmojiError, "Detected IP address %s is an IPv4-mapped IPv6 address", ip.String())
			ppfmt.InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
				"An IPv4-mapped IPv6 address is an IPv4 address in disguise. "+
					"It cannot be used for routing IPv6 traffic. "+
					"If you need to use it for DNS, please open an issue at %s",
				pp.IssueReportingURL)
			return netip.Addr{}, false
		}

	default:
		return netip.Addr{}, false
	}

	switch {
	case ip.IsUnspecified():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is an unspecified address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	case ip.IsLoopback():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is a loopback address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	case ip.IsLinkLocalMulticast():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is a link-local multicast address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	case ip.IsMulticast():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is a multicast address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	case ip.IsLinkLocalUnicast():
		ppfmt.Noticef(pp.EmojiError,
			`Detected %s address %s is a link-local address`,
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	}

	// Special-use scoped addresses were rejected above. A remaining
	// zone-qualified address is unusual and often indicates misconfiguration.
	// Independently, Cloudflare DNS record content is validated as an
	// IPv4/IPv6 address, so zone-qualified values must be rejected.
	if ip.Zone() != "" {
		ppfmt.Noticef(pp.EmojiError,
			"Detected %s address %s has a zone identifier and cannot be used as a target address",
			t.Describe(), ip.String(),
		)
		return netip.Addr{}, false
	}

	// Note that netip.IsGlobalUnicast is not equivalent to "public Internet-routable".
	// For example, private/internal ranges can still be global unicast.
	//
	// Current exceptional case after the filters above: IPv4 limited broadcast
	// 255.255.255.255 (including ::ffff:255.255.255.255 before Unmap in IPv4 mode).
	// In practice, the checks above and IsGlobalUnicast should cover all useful
	// DDNS address classes; this warning path is kept as a future-proof guard in
	// case Go or IP standards introduce new edge classes.
	if !ip.IsGlobalUnicast() {
		ppfmt.Noticef(
			pp.EmojiWarning,
			`Detected %s address %s does not look like a global unicast address`,
			t.Describe(), ip.String(),
		)
	}

	return ip, true
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
