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

// DescribeSet returns the canonical compact syntax for a non-empty set.
func DescribeSet(set Set) string {
	if set.IsZero() {
		panic("hostid6.DescribeSet requires a non-empty set")
	}

	return "[" + strings.Join(describeSetValues(set), ",") + "]"
}

// DescribeSetOrScalar returns canonical scalar syntax for a singleton set and
// canonical compact set syntax otherwise.
func DescribeSetOrScalar(set Set) string {
	if set.IsZero() {
		panic("hostid6.DescribeSetOrScalar requires a non-empty set")
	}
	if set.Len() == 1 {
		return set.values[0].Describe()
	}
	return DescribeSet(set)
}

func describeSetValues(set Set) []string {
	descriptions := make([]string, 0, set.Len())
	for derivation := range set.All() {
		descriptions = append(descriptions, derivation.Describe())
	}
	return descriptions
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

// MACAddress returns the configured MAC address when this is a MAC derivation.
func (d Derivation) MACAddress() ([6]byte, bool) {
	return d.mac, d.kind == kindMAC
}
