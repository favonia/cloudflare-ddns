package hostid6

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMustLiteralPanicsForInvalidHostLiteral(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "invalid MAC-derived host-ID literal 192.0.2.1", func() {
		_ = mustLiteral(netip.MustParseAddr("192.0.2.1"))
	})
}
