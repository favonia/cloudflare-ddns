package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestLocalCloudflareName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "local", provider.Name(provider.NewLocal()))
}

func TestNewLocalWithInterfaceRejectsBlankName(t *testing.T) {
	t.Parallel()

	for name, input := range map[string]string{
		"empty":      "",
		"whitespace": " \t\n ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mockPP.EXPECT().Noticef(pp.EmojiUserError, "%s=local.iface: must be followed by a network interface name", "IP_PROVIDER")

			p, ok := provider.NewLocalWithInterface(mockPP, "IP_PROVIDER", input)
			require.False(t, ok)
			require.Nil(t, p)
		})
	}
}

func TestMustNewLocalWithInterface(t *testing.T) {
	t.Parallel()

	p, ok := provider.MustNewLocalWithInterface("eth0").(protocol.LocalWithInterface)
	require.True(t, ok)
	require.Equal(t, "local.iface:eth0", p.ProviderName)
	require.Equal(t, "eth0", p.InterfaceName)
}

func TestMustNewLocalWithInterfacePanicsOnBlankName(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "😡 IP_PROVIDER=local.iface: must be followed by a network interface name\n", func() {
		provider.MustNewLocalWithInterface(" \t\n ")
	})
}
