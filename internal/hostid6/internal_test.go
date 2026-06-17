package hostid6

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// TestInvalidKindGuards pins the invalid-kind guard contract shared by Derive,
// Compare, and Derivation.String. The invalid state is built by setting the
// unexported kind field, which no exported constructor can produce, so the test
// must live in this same-package file. The .golangci.yaml exhaustive linter is
// configured with default-signifies-exhaustive: true, so a future unhandled
// kind would not be flagged statically; these runtime panics are the only net.
func TestInvalidKindGuards(t *testing.T) {
	t.Parallel()

	bad := Derivation{kind: Kind(255)} //nolint:exhaustruct
	raw := ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::"), 64)

	for name, call := range map[string]func(){
		"String":  func() { _ = bad.String() },
		"Compare": func() { Compare(bad, bad) },
		"Derive":  func() { Derive(raw, bad) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithValue(t, "invalid host-ID derivation kind", call)
		})
	}
}
