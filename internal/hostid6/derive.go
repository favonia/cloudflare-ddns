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

	// MACIncompatibility means the observed prefix leaves fewer than 64 host bits.
	MACIncompatibility
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
		const maxPrefixLen = 64
		if raw.PrefixLen() > maxPrefixLen {
			return netip.Addr{}, &Incompatibility{
				Kind:           MACIncompatibility,
				Derivation:     derivation,
				ObservedPrefix: raw,
				MaxPrefixLen:   maxPrefixLen,
			}
		}

		var host [16]byte
		host[8] = derivation.mac[0] ^ 0x02
		host[9] = derivation.mac[1]
		host[10] = derivation.mac[2]
		host[11] = 0xff
		host[12] = 0xfe
		host[13] = derivation.mac[3]
		host[14] = derivation.mac[4]
		host[15] = derivation.mac[5]
		return combine(raw, host), nil

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

func combine(raw ipnet.RawEntry, host [16]byte) netip.Addr {
	combined := raw.Masked().Addr().As16()
	for i := range combined {
		combined[i] |= host[i]
	}
	return netip.AddrFrom16(combined)
}
