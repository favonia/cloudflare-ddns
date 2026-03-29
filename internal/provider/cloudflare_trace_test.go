package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestCloudflareTraceName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "cloudflare.trace", provider.Name(provider.NewCloudflareTrace()))
}

func TestNewCloudflareTraceCustom(t *testing.T) {
	t.Parallel()

	p, ok := provider.MustNewCloudflareTraceCustom("https://trace.example/cdn-cgi/trace").(protocol.CloudflareTrace)
	require.True(t, ok)
	require.Equal(t, "cloudflare.trace", p.ProviderName)
	require.Equal(t, "https://trace.example/cdn-cgi/trace", p.URL[ipnet.IP4])
	require.Equal(t, "https://trace.example/cdn-cgi/trace", p.URL[ipnet.IP6])
}

func TestNewCloudflareTraceCustomRejectsBlankURL(t *testing.T) {
	t.Parallel()

	for name, input := range map[string]string{
		"empty":      "",
		"whitespace": " \t\n ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mockPP.EXPECT().Noticef(pp.EmojiUserError, "%s=cloudflare.trace: must be followed by a URL", "IP_PROVIDER")

			p, ok := provider.NewCloudflareTraceCustom(mockPP, "IP_PROVIDER", input)
			require.False(t, ok)
			require.Nil(t, p)
		})
	}
}

func TestMustNewCloudflareTrace(t *testing.T) {
	t.Parallel()

	p, ok := provider.NewCloudflareTrace().(protocol.CloudflareTrace)
	require.True(t, ok)
	require.Equal(t, "cloudflare.trace", p.ProviderName)
	require.Equal(t, "https://api.cloudflare.com/cdn-cgi/trace", p.URL[ipnet.IP4])
	require.Equal(t, "https://api.cloudflare.com/cdn-cgi/trace", p.URL[ipnet.IP6])
}

func TestMustNewCloudflareTraceCustomPanicsOnBlankURL(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "😡 IP_PROVIDER=cloudflare.trace: must be followed by a URL\n", func() {
		provider.MustNewCloudflareTraceCustom(" \t\n ")
	})
}
