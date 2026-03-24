// vim: nowrap

package protocol_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestNewUnavailable(t *testing.T) {
	t.Parallel()

	p := protocol.NewUnavailable("test-unavailable")
	require.Equal(t, "test-unavailable", p.ProviderName)
}

func TestUnavailableName(t *testing.T) {
	t.Parallel()

	p := &protocol.Unavailable{ProviderName: "very secret name"}
	require.Equal(t, "very secret name", p.Name())
}

func TestUnavailableIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.False(t, protocol.Unavailable{ProviderName: ""}.IsExplicitEmpty())
}

func TestUnavailableGetRawData(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ipFamily ipnet.Family
	}{
		"ipv4": {ipnet.IP4},
		"ipv6": {ipnet.IP6},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mockPP.EXPECT().Infof(pp.EmojiError,
				"The provider %s simulates detection failure (no real detection is attempted)", "debug.unavailable")

			p := protocol.NewUnavailable("debug.unavailable")
			rawData := p.GetRawData(context.Background(), mockPP, tc.ipFamily,
				protocol.DefaultRawDataPrefixLen(tc.ipFamily))
			require.False(t, rawData.Available)
			require.Empty(t, rawData.RawEntries)
		})
	}
}
