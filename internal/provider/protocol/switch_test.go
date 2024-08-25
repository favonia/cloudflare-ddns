package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestConstant(t *testing.T) {
	t.Parallel()
	s := protocol.Constant("very secret string")
	require.Equal(t, "very secret string", s.Switch(protocol.MethodPrimary))
	require.Equal(t, "very secret string", s.Switch(protocol.MethodAlternative))
	require.False(t, s.HasAlternative())
}

func TestSwitchable(t *testing.T) {
	t.Parallel()
	s := protocol.Switchable{Primary: "very secret string 1", Alternative: "very secret string 2"}
	require.Equal(t, "very secret string 1", s.Switch(protocol.MethodPrimary))
	require.Equal(t, "very secret string 2", s.Switch(protocol.MethodAlternative))
	require.True(t, s.HasAlternative())
}
