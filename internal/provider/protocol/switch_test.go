package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestConstant(t *testing.T) {
	t.Parallel()
	s := protocol.Constant("very secret string")
	require.Equal(t, "very secret string", s.Switch(false))
	require.Equal(t, "very secret string", s.Switch(true))
}

func TestSwitchable(t *testing.T) {
	t.Parallel()
	s := protocol.Switchable{ValueFor1001: "very secret string 1", ValueFor1111: "very secret string 2"}
	require.Equal(t, "very secret string 2", s.Switch(false))
	require.Equal(t, "very secret string 1", s.Switch(true))
}
