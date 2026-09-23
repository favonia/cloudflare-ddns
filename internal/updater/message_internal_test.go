package updater

import (
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMessageFailed(t *testing.T) {
	t.Parallel()
	for _, code := range []setter.ResponseCode{setter.ResponseNoop, setter.ResponseUpdated, setter.ResponseUpdating, setter.ResponseFailed} {
		responses := emptySetterWAFListResponses()
		responses.register("account/list", code)
		for _, message := range []Message{generateUpdateWAFListsMessage(responses), generateFinalClearWAFListsMessage(responses)} {
			require.Equal(t, code == setter.ResponseFailed, message.Failed())
		}
	}
	require.False(t, mergeMessages().Failed())
	require.True(t, mergeMessages(generateDetectMessage(ipnet.IP4, false), generateDetectMessage(ipnet.IP6, true)).Failed())
	require.True(t, generateIP6DerivationFailureMessage().Failed())
	require.True(t, generateFilterAbortDetectMessage(ipnet.IP4).Failed())
}
