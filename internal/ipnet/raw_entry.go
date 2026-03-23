package ipnet

import (
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// RawEntry carries one detected IP address together with its prefix length.
//
// Unlike [netip.Prefix], the full address is significant: host bits are
// preserved and used downstream (e.g., DNS derivation extracts the address;
// WAF derivation explicitly masks via [RawEntry.Masked]). The prefix length
// rides alongside the address but does not imply that host bits are irrelevant.
//
// Construction: use [RawEntryFrom] or [LiftValidatedIPsToRawEntries].
type RawEntry netip.Prefix

// RawEntryFrom constructs a [RawEntry] from an address and prefix length.
func RawEntryFrom(addr netip.Addr, prefixLen int) RawEntry {
	return RawEntry(netip.PrefixFrom(addr, prefixLen))
}

// Addr returns the IP address, including any host bits.
func (r RawEntry) Addr() netip.Addr { return netip.Prefix(r).Addr() }

// PrefixLen returns the prefix length.
func (r RawEntry) PrefixLen() int { return netip.Prefix(r).Bits() }

// IsValid reports whether the entry was constructed from valid inputs.
func (r RawEntry) IsValid() bool { return netip.Prefix(r).IsValid() }

// String returns the CIDR notation representation (e.g. "1.2.3.4/32").
func (r RawEntry) String() string { return netip.Prefix(r).String() }

// Compare returns an integer comparing two raw entries.
// The result is suitable for use with [slices.SortFunc].
func (r RawEntry) Compare(other RawEntry) int {
	return netip.Prefix(r).Compare(netip.Prefix(other))
}

// Masked returns the network prefix with host bits zeroed.
// This is the explicit derivation step from raw entry to network prefix.
func (r RawEntry) Masked() netip.Prefix { return netip.Prefix(r).Masked() }

// Prefix converts back to [netip.Prefix] for stdlib or external API interop.
func (r RawEntry) Prefix() netip.Prefix { return netip.Prefix(r) }

// LiftValidatedIPsToRawEntries preserves the observed address bits and applies
// the given prefix length to each already-validated address.
func LiftValidatedIPsToRawEntries(ips []netip.Addr, prefixLen int) []RawEntry {
	if len(ips) == 0 {
		return nil
	}

	entries := make([]RawEntry, 0, len(ips))
	for _, ip := range ips {
		entries = append(entries, RawEntryFrom(ip, prefixLen))
	}
	return entries
}

// normalizeDetectedRawEntry normalizes a detected raw-data IP address with
// prefix length into the requested family while preserving host bits.
func normalizeDetectedRawEntry(t Family, ppfmt pp.PP, entry RawEntry) (RawEntry, bool) {
	if !entry.IsValid() {
		ppfmt.Noticef(pp.EmojiImpossible,
			`Detected address is not valid; this should not happen and please report it at %s`,
			pp.IssueReportingURL,
		)
		return RawEntry{}, false //nolint:exhaustruct
	}

	addr := entry.Addr()
	bits := entry.PrefixLen()

	// Raw-entry-specific: prefix-length adjustment for IPv4-mapped IPv6 in IPv4 family.
	// Inspired by RFC 6887's PCP FILTER semantics: when an IPv4 prefix is encoded
	// in the ::ffff:0:0/96 mapped form, the encoded prefix length is the IPv4
	// prefix length plus the fixed 96-bit mapping prefix.
	if t == IP4 && addr.Is4In6() {
		if bits < 96 {
			ppfmt.Noticef(pp.EmojiError,
				"Detected address %s is an IPv4-mapped IPv6 address with a prefix length shorter than /96 and cannot be used",
				entry.String(),
			)
			return RawEntry{}, false //nolint:exhaustruct
		}
		bits -= 96
	}

	normalized, issue, is4in6Hint, ok := ValidateAndNormalizeIP(t, addr)
	if !ok {
		ppfmt.Noticef(pp.EmojiError, "Detected address %s is %s", entry.String(), issue)
		if is4in6Hint {
			ppfmt.InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
				"An IPv4-mapped IPv6 address is an IPv4 address in disguise. "+
					"It cannot be used for routing IPv6 traffic. "+
					"If you need to use it for DNS, please open an issue at %s",
				pp.IssueReportingURL)
		}
		return RawEntry{}, false //nolint:exhaustruct
	}

	return RawEntryFrom(normalized, bits), true
}

// NormalizeDetectedRawEntries normalizes a list of detected raw-data IP
// addresses with prefix lengths while preserving host bits in the address
// portion.
//
// Behavior:
//   - fail-fast: return false on the first invalid entry
//   - preserve emptiness: empty input returns empty output
//   - canonicalize set semantics: output is sorted and deduplicated.
func (t Family) NormalizeDetectedRawEntries(ppfmt pp.PP, entries []RawEntry) ([]RawEntry, bool) {
	if len(entries) == 0 {
		return entries, true
	}

	normalized := make([]RawEntry, 0, len(entries))
	for _, entry := range entries {
		entry, ok := normalizeDetectedRawEntry(t, ppfmt, entry)
		if !ok {
			return nil, false
		}
		normalized = append(normalized, entry)
	}

	slices.SortFunc(normalized, RawEntry.Compare)
	normalized = slices.Compact(normalized)
	return normalized, true
}
