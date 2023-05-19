package config_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
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
