package ipnet

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ErrRawEntryParse indicates that a string could not be parsed as an IP address or CIDR range.
var ErrRawEntryParse = errors.New("failed to parse as an IP address or CIDR range")

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

// Describe returns a human-readable representation of the raw entry.
// The CIDR suffix is omitted only when the entry is a single host
// (prefix length == address bit length) AND that full-host length is also
// the configured default. This keeps output familiar: users who leave the
// default at /32 see bare "1.2.3.4", while users who set /24 always see
// the explicit "/24" or "/32" suffix so the distinction is never ambiguous.
func (r RawEntry) Describe(defaultPrefixLen int) string {
	p := netip.Prefix(r)
	maxBits := p.Addr().BitLen()
	if defaultPrefixLen == maxBits && p.Bits() == maxBits {
		return p.Addr().String()
	}
	return p.String()
}

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

// ParseRawEntry parses s as a CIDR prefix or a bare IP address.
// Bare IPs receive defaultPrefixLen. CIDR notation preserves the stated prefix
// length and the full address (host bits included).
//
// Zoned addresses (e.g. "fe80::1%eth0") are rejected because [netip.PrefixFrom]
// silently strips zones, which would lose information.
func ParseRawEntry(s string, defaultPrefixLen int) (RawEntry, error) {
	if p, err := netip.ParsePrefix(s); err == nil {
		return RawEntryFrom(p.Addr(), p.Bits()), nil
	}
	if addr, err := netip.ParseAddr(s); err == nil {
		if addr.Zone() != "" {
			return RawEntry{}, fmt.Errorf("%w: %q: zones are not supported", ErrRawEntryParse, s)
		}
		return RawEntryFrom(addr, defaultPrefixLen), nil
	}
	return RawEntry{}, fmt.Errorf("%w: %q", ErrRawEntryParse, s)
}

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

// Emit4in6Hint emits the standard IPv4-mapped IPv6 hint message when
// is4in6Hint is true. Safe to call unconditionally; it is a no-op otherwise.
func Emit4in6Hint(ppfmt pp.PP, is4in6Hint bool) {
	if is4in6Hint {
		ppfmt.InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
			"An IPv4-mapped IPv6 address is an IPv4 address in disguise. "+
				"It cannot be used for routing IPv6 traffic. "+
				"If you need to use it for DNS, please open an issue at %s",
			pp.IssueReportingURL)
	}
}

// NormalizeRawEntryIP adjusts the prefix length for IPv4-mapped IPv6 addresses
// and validates the IP for the given family. No messages are emitted; callers
// use the returned problem description and is4in6Hint for their own diagnostics.
//
// On success problem is empty. On failure problem is a predicate phrase
// suitable for "(subject) %s" (e.g., "is not a valid IPv4 address").
//
// The prefix-length adjustment follows RFC 6887 PCP FILTER semantics: when an
// IPv4 prefix is encoded in the ::ffff:0:0/96 mapped form, the encoded prefix
// length is the IPv4 prefix length plus the fixed 96-bit mapping prefix.
func NormalizeRawEntryIP(family Family, entry RawEntry) (
	normalized RawEntry, problem string, is4in6Hint bool, ok bool,
) {
	addr := entry.Addr()
	bits := entry.PrefixLen()

	if family == IP4 && addr.Is4In6() {
		if bits < 96 {
			return RawEntry{}, //nolint:exhaustruct
				"is an IPv4-mapped IPv6 address with a prefix length shorter than /96 and cannot be used",
				false, false
		}
		bits -= 96
	}

	norm, issue, hint, valid := ValidateAndNormalizeIP(family, addr)
	if !valid {
		return RawEntry{}, issue, hint, false //nolint:exhaustruct
	}

	return RawEntryFrom(norm, bits), "", false, true
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

	normalized, problem, is4in6Hint, ok := NormalizeRawEntryIP(t, entry)
	if !ok {
		ppfmt.Noticef(pp.EmojiError, "Detected address %s %s", entry.String(), problem)
		Emit4in6Hint(ppfmt, is4in6Hint)
		return RawEntry{}, false //nolint:exhaustruct
	}

	return normalized, true
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
