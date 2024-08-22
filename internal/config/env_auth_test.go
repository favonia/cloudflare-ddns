package config_test

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestReadAuth(t *testing.T) {
	unset(t, "CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID")

	for name, tc := range map[string]struct {
		token         string
		account       string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {"123456789", "", true, nil},
		"empty-token": {
			"", "account", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "Needs either CF_API_TOKEN or CF_API_TOKEN_FILE")
			},
		},
		"invalid": {
			"!!!", "", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, "The API token does not look like a valid OAuth2 bearer token")
			},
		},
		"account": {
			"123456789", "secret account", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, "CF_ACCOUNT_ID is ignored since 1.14.0")
			},
		},
		"copycat": {
			"YOUR-CLOUDFLARE-API-TOKEN", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "You need to provide a real API token as CF_API_TOKEN")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "CF_API_TOKEN", tc.token)
			store(t, "CF_ACCOUNT_ID", tc.account)

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			var field api.Auth
			ok := config.ReadAuth(mockPP, &field)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, &api.CloudflareAuth{Token: tc.token, BaseURL: ""}, field)
			} else {
				require.Nil(t, field)
			}
		})
	}
}

func useMemFS(memfs fstest.MapFS) {
	file.FS = memfs
}

//nolint:funlen,paralleltest // environment vars and file system are global
func TestReadAuthWithFile(t *testing.T) {
	unset(t, "CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID")

	for name, tc := range map[string]struct {
		token         string
		tokenFile     string
		actualPath    string
		actualContent string
		expected      string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"ok": {"", "test.txt", "test.txt", "hello", "hello", true, nil},
		"both": {
			"123456789", "test.txt", "test.txt", "hello", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set")
			},
		},
		"wrong.path": {
			"", "wrong.txt", "actual.txt", "hello", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "wrong.txt", gomock.Any())
			},
		},
		"empty": {
			"", "test.txt", "test.txt", "", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "The token in the file specified by CF_API_TOKEN_FILE is empty")
			},
		},
		"invalid path": {
			"", "dir", "dir/test.txt", "hello", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "dir", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "CF_API_TOKEN", tc.token)
			store(t, "CF_API_TOKEN_FILE", tc.tokenFile)

			useMemFS(fstest.MapFS{
				tc.actualPath: &fstest.MapFile{
					Data:    []byte(tc.actualContent),
					Mode:    0o644,
					ModTime: time.Unix(1234, 5678),
					Sys:     nil,
				},
			})

			var field api.Auth
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadAuth(mockPP, &field)
			require.Equal(t, tc.ok, ok)
			if tc.expected != "" {
				require.Equal(t, &api.CloudflareAuth{Token: tc.expected, BaseURL: ""}, field)
			} else {
				require.Nil(t, field)
			}
		})
	}
}
