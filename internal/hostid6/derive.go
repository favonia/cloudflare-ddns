package hostid6

import (
	"math/bits"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// IncompatibilityKind identifies why a derivation is incompatible with an observed prefix.
type IncompatibilityKind uint8

const (
	// LiteralIncompatibility means configured literal bits overlap the observed prefix.
	LiteralIncompatibility IncompatibilityKind = iota

	// MACPrefixTooLong means the observed prefix is longer than /64, leaving fewer
	// than 64 host bits for the Modified EUI-64 interface identifier.
	MACPrefixTooLong

	// MACPrefixTooShort means the observed prefix is shorter than /64, leaving the
	// subnet bits between the prefix and /64 undefined by the MAC derivation.
	MACPrefixTooShort
)

// Incompatibility describes a derivation that cannot be applied to an observed prefix.
type Incompatibility struct {
	Kind           IncompatibilityKind
	Derivation     Derivation
	ObservedPrefix ipnet.RawEntry
	MaxPrefixLen   int
}

// Derive applies one intentional host-ID derivation to an observed IPv6 prefix.
// The raw entry must be valid and contain an IPv6 address.
// Derive panics when this internal precondition is violated.
func Derive(raw ipnet.RawEntry, derivation Derivation) (netip.Addr, *Incompatibility) {
	if !raw.IsValid() {
		panic("hostid6.Derive received an invalid raw entry; this should not happen; please report it")
	}
	if !raw.Addr().Is6() || raw.Addr().Is4In6() {
		panic("hostid6.Derive received a non-IPv6 raw entry; this should not happen; please report it")
	}

	switch derivation.kind {
	case kindPreserve:
		return raw.Addr(), nil

	case kindLiteral:
		maxPrefixLen := literalMaxPrefixLen(derivation.literal)
		if raw.PrefixLen() > maxPrefixLen {
			return netip.Addr{}, &Incompatibility{
				Kind:           LiteralIncompatibility,
				Derivation:     derivation,
				ObservedPrefix: raw,
				MaxPrefixLen:   maxPrefixLen,
			}
		}
		return combine(raw, derivation.literal.As16()), nil

	case kindMAC:
		// A Modified EUI-64 host ID is a 64-bit interface identifier that only has
		// a defined meaning within a /64: a longer prefix leaves fewer than 64 host
		// bits, and a shorter prefix leaves the subnet bits between the prefix and
		// /64 undefined. So the MAC derivation requires exactly a /64.
		const exactPrefixLen = 64
		switch {
		case raw.PrefixLen() > exactPrefixLen:
			return netip.Addr{}, &Incompatibility{
				Kind:           MACPrefixTooLong,
				Derivation:     derivation,
				ObservedPrefix: raw,
				MaxPrefixLen:   exactPrefixLen,
			}
		case raw.PrefixLen() < exactPrefixLen:
			return netip.Addr{}, &Incompatibility{
				Kind:           MACPrefixTooShort,
				Derivation:     derivation,
				ObservedPrefix: raw,
				MaxPrefixLen:   exactPrefixLen,
			}
		}
		return combine(raw, macHost(derivation.mac)), nil

	default:
		panic("invalid host-ID derivation kind")
	}
}

func literalMaxPrefixLen(literal netip.Addr) int {
	for i, octet := range literal.As16() {
		if octet != 0 {
			return i*8 + bits.LeadingZeros8(octet)
		}
	}
	return 128
}

// MACHostID returns the Modified EUI-64 host ID of a MAC derivation as an
// address with zero network and subnet bits (`::<interface-identifier>`), and
// whether the derivation is a MAC derivation. The caller is responsible for
// the subnet bits between an observed prefix and /64, which the MAC alone does
// not determine.
func MACHostID(derivation Derivation) (netip.Addr, bool) {
	if derivation.kind != kindMAC {
		return netip.Addr{}, false
	}
	return netip.AddrFrom16(macHost(derivation.mac)), true
}

// macHost lays out the 64-bit Modified EUI-64 interface identifier of a 48-bit
// MAC address in the lower half of a 128-bit host-bit block.
func macHost(mac [6]byte) [16]byte {
	var host [16]byte
	host[8] = mac[0] ^ 0x02
	host[9] = mac[1]
	host[10] = mac[2]
	host[11] = 0xff
	host[12] = 0xfe
	host[13] = mac[3]
	host[14] = mac[4]
	host[15] = mac[5]
	return host
}

func combine(raw ipnet.RawEntry, host [16]byte) netip.Addr {
	combined := raw.Masked().Addr().As16()
	for i := range combined {
		combined[i] |= host[i]
	}
	return netip.AddrFrom16(combined)
}
