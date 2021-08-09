package config_test

import (
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
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
		"full":      {"123456789", "secret account", true},
		"noaccount": {"123456789", "", true},
		"notoken":   {"", "account", false},
		"copycat":   {"YOUR-CLOUDFLARE-API-TOKEN", "", false},
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
				require.True(t, ok)
				require.Equal(t, &api.CloudflareAuth{Token: tc.token, AccountID: tc.account, BaseURL: ""}, field)
			} else {
				require.False(t, ok)
				require.Nil(t, field)
			}
		})
	}
}

func useMemFS(memfs fstest.MapFS) {
	file.FS = memfs
}

func useDirFS() {
	file.FS = os.DirFS("/")
}

//nolint: paralleltest // environment vars and file system are global
func TestReadAuthTokenWithFile(t *testing.T) {
	unset("CF_API_TOKEN")
	unset("CF_API_TOKEN_FILE")
	unset("CF_ACCOUNT_ID")

	for name, tc := range map[string]struct {
		token         string
		tokenFile     string
		account       string
		actualPath    string
		actualContent string
		expected      string
	}{
		"ok":           {"", "test.txt", "secret account", "test.txt", "hello", "hello"},
		"both":         {"123456789", "test.txt", "secret account", "test.txt", "hello", ""},
		"wrong.path":   {"123456789", "test.txt", "secret account", "wrong.txt", "hello", ""},
		"empty":        {"", "test.txt", "secret account", "test.txt", "", ""},
		"invalid path": {"", "dir/test.txt", "secret account", "dir", "hello", ""},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set("CF_API_TOKEN", tc.token)
			set("CF_API_TOKEN_FILE", tc.tokenFile)
			set("CF_ACCOUNT_ID", tc.account)
			defer unset("CF_API_TOKEN")
			defer unset("CF_API_TOKEN_FILE")
			defer unset("CF_ACCOUNT_ID")

			useMemFS(fstest.MapFS{
				tc.actualPath: &fstest.MapFile{
					Data:    []byte(tc.actualContent),
					Mode:    0o644,
					ModTime: time.Unix(1234, 5678),
					Sys:     nil,
				},
			})
			defer useDirFS()

			var field api.Auth
			ok := config.ReadAuth(quiet.QUIET, 2, &field)
			if tc.expected != "" {
				require.True(t, ok)
				require.Equal(t, &api.CloudflareAuth{Token: tc.expected, AccountID: tc.account, BaseURL: ""}, field)
			} else {
				require.False(t, ok)
				require.Nil(t, field)
			}
		})
	}
}

//nolint: paralleltest // environment vars and file system are global
func TestReadDomainMap(t *testing.T) {
	unset("DOMAINS")
	unset("IP4_DOMAINS")
	unset("IP6_DOMAINS")

	for name, tc := range map[string]struct {
		domains    string
		ip4Domains string
		ip6Domains string
		expected   map[ipnet.Type][]api.FQDN
		ok         bool
	}{
		"full": {
			"  a1, a2", "b1,  b2,b2", "c1,c2",
			map[ipnet.Type][]api.FQDN{
				ipnet.IP4: {"a1", "a2", "b1", "b2"},
				ipnet.IP6: {"a1", "a2", "c1", "c2"},
			},
			true,
		},
		"empty": {
			" ", "   ", "",
			map[ipnet.Type][]api.FQDN{
				ipnet.IP4: {},
				ipnet.IP6: {},
			},
			true,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set("DOMAINS", tc.domains)
			set("IP4_DOMAINS", tc.ip4Domains)
			set("IP6_DOMAINS", tc.ip6Domains)
			defer unset("DOMAINS")
			defer unset("IP4_DOMAINS")
			defer unset("IP6_DOMAINS")

			field := map[ipnet.Type][]api.FQDN{}
			ok := config.ReadDomainMap(quiet.QUIET, 2, field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}
