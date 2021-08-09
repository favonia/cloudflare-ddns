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
	unset("CF_API_TOKEN")
	unset("CF_API_TOKEN_FILE")
	unset("CF_ACCOUNT_ID")

	for name, tc := range map[string]struct {
		token   string
		account string
		ok      bool
	}{
		"full":      {"123456789", "secret account", false},
		"noaccount": {"123456789", "", false},
		"notoken":   {"", "account", true},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set("CF_API_TOKEN", tc.token)
			set("CF_ACCOUNT_ID", tc.account)
			defer unset("CF_API_TOKEN")
			defer unset("CF_ACCOUNT_ID")

			var field api.Auth
			ok := config.ReadAuth(quiet.QUIET, 2, &field)
			if tc.ok {
				require.False(t, ok)
				require.Nil(t, field)
			} else {
				require.True(t, ok)
				require.Equal(t, &api.CloudflareAuth{Token: tc.token, AccountID: tc.account, BaseURL: ""}, field)
			}

		})
	}
}
