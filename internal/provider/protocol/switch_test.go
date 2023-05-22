package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestConstant(t *testing.T) {
	s := protocol.Constant("very secret string")
	require.Equal(t, s.Switch(false), "very secret string")
	require.Equal(t, s.Switch(true), "very secret string")
}

func TestSwitchable(t *testing.T) {
	s := protocol.Switchable{Use1001: "very secret string 1", Use1111: "very secret string 2"}
	require.Equal(t, s.Switch(false), "very secret string 2")
	require.Equal(t, s.Switch(true), "very secret string 1")
}
