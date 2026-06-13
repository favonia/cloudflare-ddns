// Package hostid6 models intentional IPv6 host-ID derivations.
package hostid6

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"net/netip"
	"slices"
)

// Kind identifies one host-ID derivation kind.
type Kind uint8

const (
	kindPreserve Kind = iota
	kindLiteral
	kindMAC
)

var errInvalidLiteral = errors.New("host-ID literal must be an unzoned IPv6 address")

// Derivation identifies one intentional IPv6 host-ID derivation.
type Derivation struct {
	kind    Kind
	literal netip.Addr
	mac     [6]byte
}

// Set is a sorted, deduplicated, non-empty set of derivations.
type Set []Derivation

// Preserve constructs a derivation that preserves observed host bits.
func Preserve() Derivation {
	return Derivation{kind: kindPreserve} //nolint:exhaustruct
}

// Literal constructs a derivation from an IPv6 host-ID literal.
func Literal(addr netip.Addr) (Derivation, error) {
	if !addr.Is6() || addr.Is4In6() || addr.Zone() != "" {
		return Derivation{}, errInvalidLiteral
	}

	return Derivation{kind: kindLiteral, literal: addr}, nil //nolint:exhaustruct
}

// MAC constructs a derivation from a 48-bit MAC address.
func MAC(addr [6]byte) Derivation {
	return Derivation{kind: kindMAC, mac: addr} //nolint:exhaustruct
}

// NewSet constructs a canonical non-empty derivation set.
func NewSet(values ...Derivation) Set {
	if len(values) == 0 {
		panic("hostid6.NewSet requires at least one derivation")
	}

	set := slices.Clone(values)
	slices.SortFunc(set, Compare)
	return slices.Compact(set)
}

// DefaultSet returns the default derivation set.
func DefaultSet() Set {
	return NewSet(Preserve())
}

// Compare compares derivations by intentional identity.
func Compare(left, right Derivation) int {
	if order := cmp.Compare(left.kind, right.kind); order != 0 {
		return order
	}

	switch left.kind {
	case kindPreserve:
		return 0
	case kindLiteral:
		return left.literal.Compare(right.literal)
	case kindMAC:
		return bytes.Compare(left.mac[:], right.mac[:])
	default:
		panic("invalid host-ID derivation kind")
	}
}

// EqualSet reports whether two canonical derivation sets are equal.
func EqualSet(left, right Set) bool {
	return slices.Equal(left, right)
}

// Describe returns the canonical description of a derivation.
func (d Derivation) Describe() string {
	switch d.kind {
	case kindPreserve:
		return "preserve"
	case kindLiteral:
		return d.literal.String()
	case kindMAC:
		return fmt.Sprintf("mac(%02x-%02x-%02x-%02x-%02x-%02x)",
			d.mac[0], d.mac[1], d.mac[2], d.mac[3], d.mac[4], d.mac[5])
	default:
		panic("invalid host-ID derivation kind")
	}
}
