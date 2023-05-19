package config_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
)

func TestProbeURLTrue(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
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
	// config.ShouldWeUse1001 must return false on GitHub.
	require.False(t, config.ShouldWeUse1001(context.Background(), mockPP))
}
