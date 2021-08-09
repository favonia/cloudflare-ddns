package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/quiet"
)

func TestDefaultConfigNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.Default())
}

//nolint: paralleltest // environment vars are global
func TestReadAuthToken(t *testing.T) {
	set("CF_API_TOKEN", "123456789")
	unset("CF_API_TOKEN_FILE")
	set("CF_ACCOUNT_ID", "secret account")

	var field api.Auth
	ok := config.ReadAuth(quiet.QUIET, 2, &field)
	require.True(t, ok)
	require.Equal(t, &api.CloudflareAuth{Token: "123456789", AccountID: "secret account", BaseURL: ""}, field)
}
