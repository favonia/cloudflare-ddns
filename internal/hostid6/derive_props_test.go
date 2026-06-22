// Package hostid6_test — property-based invariant tests for Derive.
//
// Design rationale: A build-tagged differential test (added later) compares
// Go Derive against an independent Lean model over near-exhaustive inputs; the
// Lean model is formally proven to satisfy T1/T2/T3. The tests in this file
// therefore add NO independent verification strength: Go tested against a
// hand-copy of Go is circular. We keep only three cheap, always-on regression
// tripwires that assert each contract invariant against an INPUT or a FIXED
// CONSTANT via derivation-agnostic helpers — deliberately omitting any test
// that would re-implement a private function of the package.
package hostid6_test

import (
	"net/netip"
	"testing"

	"pgregory.net/rapid"

	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// genIPv6 draws a random, unzoned IPv6 address from 16 independent random bytes.
// Exported to package-test scope so the later build-tagged differential test
// can reuse it without duplication.
func genIPv6(t *rapid.T) netip.Addr {
	t.Helper()
	var b [16]byte
	for i := range b {
		b[i] = rapid.Byte().Draw(t, "b")
	}
	return netip.AddrFrom16(b)
}

// newRaw constructs an ipnet.RawEntry from an IPv6 address and a prefix length.
func newRaw(addr netip.Addr, p int) ipnet.RawEntry {
	return ipnet.RawEntryFrom(addr, p)
}

// genMAC draws a random 6-byte MAC address.
func genMAC(t *rapid.T) [6]byte {
	t.Helper()
	var m [6]byte
	for i := range m {
		m[i] = rapid.Byte().Draw(t, "mac")
	}
	return m
}

// maskTopBits keeps the top p bits of b and zeroes the rest.
func maskTopBits(b [16]byte, p int) [16]byte {
	var out [16]byte
	for i := range 16 {
		bitsLeft := p - i*8
		switch {
		case bitsLeft >= 8:
			out[i] = b[i]
		case bitsLeft <= 0:
			out[i] = 0
		default:
			out[i] = b[i] & (0xff << (8 - bitsLeft))
		}
	}
	return out
}

// hostBits zeroes the top p bits of b and keeps the low 128-p bits.
func hostBits(b [16]byte, p int) [16]byte {
	mask := maskTopBits([16]byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}, p)
	var out [16]byte
	for i := range out {
		out[i] = b[i] &^ mask[i]
	}
	return out
}

// TestProp_T1_PrefixPreserved asserts that Derive never touches the top p bits
// of the input address (T1: prefix preservation). The assertion compares the
// masked output against the masked input — no recomputation of Derive's logic.
func TestProp_T1_PrefixPreserved(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		p := rapid.IntRange(0, 128).Draw(t, "p")
		addr := genIPv6(t)

		// Build a small sample of derivations: one Preserve, one MAC, one Literal.
		mac := hostid6.MAC(genMAC(t))
		litAddr := genIPv6(t)
		litDeriv, litErr := hostid6.Literal(litAddr)

		derivations := []hostid6.Derivation{hostid6.Preserve(), mac}
		if litErr == nil {
			derivations = append(derivations, litDeriv)
		}

		wantPrefix := maskTopBits(addr.As16(), p)
		for _, d := range derivations {
			out, incompat := hostid6.Derive(newRaw(addr, p), d)
			if incompat != nil {
				continue
			}
			if maskTopBits(out.As16(), p) != wantPrefix {
				t.Fatalf("T1 violated for derivation %v, p=%d: prefix bits changed", d, p)
			}
		}
	})
}

// TestProp_T2_LiteralHost asserts that when a Literal derivation succeeds, the
// host bits of the output equal the host bits of the literal (T2: host
// correctness). The assertion compares host regions of two addresses — no
// recomputation of combine or any other private function.
func TestProp_T2_LiteralHost(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		p := rapid.IntRange(0, 128).Draw(t, "p")
		addr := genIPv6(t)
		lit := genIPv6(t)

		d, err := hostid6.Literal(lit)
		if err != nil {
			// genIPv6 uses AddrFrom16, which always produces a valid unzoned IPv6,
			// so Literal should never error here; skip defensively if it somehow does.
			t.Skip("Literal rejected address (unexpected)")
		}

		out, inc := hostid6.Derive(newRaw(addr, p), d)
		if inc != nil {
			// Prefix too long for this literal — not a T2 scenario.
			return
		}

		if hostBits(out.As16(), p) != hostBits(lit.As16(), p) {
			t.Fatalf("T2 violated: host bits of output differ from host bits of literal (p=%d)", p)
		}
	})
}

// TestProp_T3_MACBoundary asserts that MAC derivation succeeds iff the prefix
// length is exactly 64 (T3: incompatibility characterization). The assertion
// uses only the documented constant 64 — no EUI-64 byte recomputation.
func TestProp_T3_MACBoundary(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		p := rapid.IntRange(0, 128).Draw(t, "p")
		addr := genIPv6(t)

		_, inc := hostid6.Derive(newRaw(addr, p), hostid6.MAC(genMAC(t)))

		switch {
		case p > 64:
			if inc == nil || inc.Kind != hostid6.MACPrefixTooLong || inc.PrefixLenBound != 64 {
				t.Fatalf("T3 violated: expected MACPrefixTooLong with bound 64 for p=%d, got %+v", p, inc)
			}
		case p < 64:
			if inc == nil || inc.Kind != hostid6.MACPrefixTooShort || inc.PrefixLenBound != 64 {
				t.Fatalf("T3 violated: expected MACPrefixTooShort with bound 64 for p=%d, got %+v", p, inc)
			}
		default: // p == 64
			if inc != nil {
				t.Fatalf("T3 violated: expected success for p=64, got incompatibility Kind=%v", inc.Kind)
			}
		}
	})
}
