//go:build lean_oracle

package hostid6_test

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/favonia/cloudflare-ddns/internal/hostid6"
)

// oracle wraps the persistent Lean oracle subprocess: one request line in, one
// response line out.
type oracle struct {
	cmd *exec.Cmd
	in  *bufio.Writer
	out *bufio.Scanner
}

// startOracle launches the Lean oracle named by HOSTID6_LEAN_ORACLE. The test is
// skipped when the variable is empty so the tagged build still compiles and runs
// without the binary present.
func startOracle(t *testing.T) *oracle {
	t.Helper()

	path := os.Getenv("HOSTID6_LEAN_ORACLE")
	if path == "" {
		t.Skip("HOSTID6_LEAN_ORACLE not set; skipping Lean differential test")
	}

	//nolint:gosec // path is a trusted test-harness env var (HOSTID6_LEAN_ORACLE), not user input
	cmd := exec.CommandContext(t.Context(), path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("oracle stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("oracle stdout pipe: %v", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting oracle %q: %v", path, err)
	}

	o := &oracle{
		cmd: cmd,
		in:  bufio.NewWriter(stdin),
		out: bufio.NewScanner(stdout),
	}

	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	})

	return o
}

// ask sends one request line and returns the single trimmed response line.
func (o *oracle) ask(t *testing.T, line string) string {
	t.Helper()
	if _, err := o.in.WriteString(line + "\n"); err != nil {
		t.Fatalf("writing to oracle: %v", err)
	}
	if err := o.in.Flush(); err != nil {
		t.Fatalf("flushing to oracle: %v", err)
	}
	if !o.out.Scan() {
		if err := o.out.Err(); err != nil {
			t.Fatalf("reading oracle response: %v", err)
		}
		t.Fatalf("oracle closed output before responding to %q", line)
	}
	return strings.TrimSpace(o.out.Text())
}

// goResult renders Derive's output in the same wire form the oracle uses.
func goResult(out netip.Addr, inc *hostid6.Incompatibility) string {
	if inc == nil {
		b := out.As16()
		return "ok " + hex.EncodeToString(b[:])
	}
	switch inc.Kind {
	case hostid6.LiteralPrefixTooLong:
		return fmt.Sprintf("err literalPrefixTooLong %d", inc.PrefixLenBound)
	case hostid6.MACPrefixTooLong:
		return "err macPrefixTooLong"
	case hostid6.MACPrefixTooShort:
		return "err macPrefixTooShort"
	default:
		return fmt.Sprintf("err unknown kind %d", inc.Kind)
	}
}

// derivCase is one enumerated derivation paired with its wire kind and payload.
type derivCase struct {
	kind    string
	payload string
	d       hostid6.Derivation
}

// enumDerivs returns a near-exhaustive set of derivations: Preserve; literals
// whose highest set bit covers every position 0..127 (so literalMaxPrefixLen
// ranges 0..127), plus all-zero (max 128) and all-ones literals; and a few MACs.
func enumDerivs(t *testing.T) []derivCase {
	t.Helper()

	var cases []derivCase

	addLiteral := func(b [16]byte) {
		litAddr := netip.AddrFrom16(b)
		d, err := hostid6.Literal(litAddr)
		if err != nil {
			// AddrFrom16 always yields a valid unzoned IPv6, so this should not
			// happen; skip defensively rather than fail.
			return
		}
		la := litAddr.As16()
		cases = append(cases, derivCase{
			kind:    "literal",
			payload: hex.EncodeToString(la[:]),
			d:       d,
		})
	}

	// Preserve.
	cases = append(cases, derivCase{kind: "preserve", payload: "", d: hostid6.Preserve()})

	// Single-bit literals: highest set bit at position k from the MSB end, for
	// k = 0..127. literalMaxPrefixLen of such a literal is exactly k.
	for k := range 128 {
		var b [16]byte
		b[k/8] = 1 << (7 - uint(k%8))
		addLiteral(b)
	}

	// All-zero literal: literalMaxPrefixLen 128.
	addLiteral([16]byte{})

	// All-ones literal: literalMaxPrefixLen 0.
	var ones [16]byte
	for i := range ones {
		ones[i] = 0xff
	}
	addLiteral(ones)

	// MACs: all-zero, all-ones, and a realistic 00:11:22:33:44:55.
	macs := [][6]byte{
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
	}
	for _, m := range macs {
		cases = append(cases, derivCase{
			kind:    "mac",
			payload: hex.EncodeToString(m[:]),
			d:       hostid6.MAC(m),
		})
	}

	return cases
}

// enumAddrs returns a small set of structurally interesting IPv6 addresses.
func enumAddrs() []netip.Addr {
	var zero, ones, aa, five, gua [16]byte
	for i := range ones {
		ones[i] = 0xff
	}
	for i := range aa {
		aa[i] = 0xaa
	}
	for i := range five {
		five[i] = 0x55
	}
	// 2001:db8:: with a realistic-looking host part.
	gua = [16]byte{
		0x20, 0x01, 0x0d, 0xb8, 0x12, 0x34, 0x56, 0x78,
		0x9a, 0xbc, 0xde, 0xf0, 0x11, 0x22, 0x33, 0x44,
	}
	return []netip.Addr{
		netip.AddrFrom16(zero),
		netip.AddrFrom16(ones),
		netip.AddrFrom16(aa),
		netip.AddrFrom16(five),
		netip.AddrFrom16(gua),
	}
}

// drawDeriv draws a random derivation case for the randomized fill.
func drawDeriv(t *rapid.T) derivCase {
	switch rapid.IntRange(0, 2).Draw(t, "derivKind") {
	case 0:
		return derivCase{kind: "preserve", payload: "", d: hostid6.Preserve()}
	case 1:
		litAddr := genIPv6(t)
		d, err := hostid6.Literal(litAddr)
		if err != nil {
			// Fall back to preserve if Literal somehow rejects the address.
			return derivCase{kind: "preserve", payload: "", d: hostid6.Preserve()}
		}
		la := litAddr.As16()
		return derivCase{kind: "literal", payload: hex.EncodeToString(la[:]), d: d}
	default:
		m := genMAC(t)
		return derivCase{kind: "mac", payload: hex.EncodeToString(m[:]), d: hostid6.MAC(m)}
	}
}

// The oracle is a single persistent subprocess shared across all requests, so
// this test must run serially; it deliberately does not call t.Parallel().
//
//nolint:paralleltest // serial by design: one shared oracle subprocess
func TestDiff_DeriveMatchesOracle(t *testing.T) {
	o := startOracle(t)

	check := func(addr netip.Addr, p int, kind, payload string, d hostid6.Derivation) {
		got := goResult(hostid6.Derive(newRaw(addr, p), d))

		a := addr.As16()
		addrHex := hex.EncodeToString(a[:])
		line := strings.TrimSpace(fmt.Sprintf("%s %d %s %s", kind, p, addrHex, payload))

		want := o.ask(t, line)
		if got != want {
			t.Fatalf("mismatch\n  line: %q\n  got:  %q\n  want: %q", line, got, want)
		}
	}

	// Near-exhaustive enumeration: every prefix length crossed with every
	// enumerated derivation and address.
	derivs := enumDerivs(t)
	addrs := enumAddrs()
	for p := range 129 {
		for _, dc := range derivs {
			for _, addr := range addrs {
				check(addr, p, dc.kind, dc.payload, dc.d)
			}
		}
	}

	// Randomized fill over fully random addresses, prefix lengths, and derivations.
	rapid.Check(t, func(t *rapid.T) {
		p := rapid.IntRange(0, 128).Draw(t, "p")
		addr := genIPv6(t)
		dc := drawDeriv(t)
		check(addr, p, dc.kind, dc.payload, dc.d)
	})
}
