package config_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestProbeURLTrue(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// an "empty" HTTP server is good enough
	}))
	defer server.Close()
	require.True(t, config.ProbeURL(context.Background(), server.URL))
}

func TestProbeURLFalse(t *testing.T) {
	t.Parallel()
	require.False(t, config.ProbeURL(context.Background(), "http://127.0.0.1:0"))
}

func TestProbeURLInvalidURL(t *testing.T) {
	t.Parallel()
	require.False(t, config.ProbeURL(context.Background(), "://"))
}

func TestProbeCloudflareIPs(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Checking 1.1.1.1 . . ."),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		innerMockPP.EXPECT().Infof(pp.EmojiGood, "1.1.1.1 appears to be working"),
	)
	c := config.Default()
	// config.ShouldWeUse1001 must return false on GitHub.
	require.True(t, c.ShouldWeUse1001(context.Background(), mockPP))
	require.False(t, c.Use1001)
}

func TestProbeCloudflareIPsNoIP4(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	c := config.Default()
	c.Provider[ipnet.IP4] = nil
	require.True(t, c.ShouldWeUse1001(context.Background(), mockPP))
	require.False(t, c.Use1001)
}
