// Package hostid6 models intentional IPv6 host-ID derivations.
package hostid6

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"iter"
	"net/netip"
	"slices"
	"strings"
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

// Set is an immutable canonical set of derivations.
//
// The zero value is empty and represents omission or internal absence.
// Sets returned by [NewSet] and [DefaultSet] are sorted, deduplicated,
// and non-empty.
type Set struct {
	values []Derivation
}

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
// NewSet panics if no derivations are provided.
func NewSet(values ...Derivation) Set {
	if len(values) == 0 {
		panic("hostid6.NewSet requires at least one derivation")
	}

	set := slices.Clone(values)
	slices.SortFunc(set, Compare)
	set = slices.Compact(set)
	return Set{values: slices.Clip(set)}
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
	return slices.Equal(left.values, right.values)
}

// IsZero reports whether a set represents omission or internal absence.
func (s Set) IsZero() bool {
	return len(s.values) == 0
}

// Len returns the number of derivations in the set.
func (s Set) Len() int {
	return len(s.values)
}

// Values returns a copy of the canonical derivations in the set.
func (s Set) Values() []Derivation {
	return slices.Clone(s.values)
}

// All enumerates the canonical derivations in the set.
func (s Set) All() iter.Seq[Derivation] {
	return slices.Values(s.values)
}

// String returns the canonical compact syntax for a non-empty set.
func (s Set) String() string {
	if s.IsZero() {
		panic("hostid6.Set.String requires a non-empty set")
	}

	return "[" + strings.Join(setSyntaxValues(s), ",") + "]"
}

// ConfigString returns canonical hostid6 field-value syntax for a non-empty set.
// Singleton sets use scalar syntax; larger sets use bracketed set syntax.
func (s Set) ConfigString() string {
	switch len(s.values) {
	case 0:
		panic("hostid6.Set.ConfigString requires a non-empty set")
	case 1:
		return s.values[0].String()
	default:
		return s.String()
	}
}

func setSyntaxValues(set Set) []string {
	syntaxes := make([]string, 0, set.Len())
	for derivation := range set.All() {
		syntaxes = append(syntaxes, derivation.String())
	}
	return syntaxes
}

// String returns the canonical syntax of a derivation.
func (d Derivation) String() string {
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

// Describe returns a human-readable description of a derivation. It matches
// [Derivation.String] except that the host-bit-preserving derivation is
// annotated for the human-facing configuration summary.
func (d Derivation) Describe() string {
	if d.kind == kindPreserve {
		return "preserve (using detected)"
	}
	return d.String()
}

// MACAddress returns the configured MAC address when this is a MAC derivation.
func (d Derivation) MACAddress() ([6]byte, bool) {
	return d.mac, d.kind == kindMAC
}
