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

func useMemFS(memfs fstest.MapFS) {
	file.FS = memfs
}

//nolint:paralleltest // environment vars and file system are global
func TestReadAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		mapFS          map[string]string
		token1         string
		token2         string
		fileToken1Path string
		fileToken2Path string
		account        string
		ok             bool
		expected       string
		prepareMockPP  func(*mocks.MockPP)
	}{
		"success": {
			map[string]string{"token.txt": "hello"},
			"123456789", "", "", "", "",
			true, "123456789", nil,
		},
		"empty-token": {
			map[string]string{},
			"", "", "", "", "account",
			false, "",
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"Needs either %s or %s", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_API_TOKEN_FILE")
			},
		},
		"invalid": {
			map[string]string{},
			"!!!", "", "", "", "",
			true, "!!!",
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning,
					"The API token appears to be invalid. It does not follow the OAuth2 bearer token format.")
			},
		},
		"account": {
			map[string]string{},
			"123456789", "", "", "", "secret account",
			true, "123456789",
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, "CF_ACCOUNT_ID is ignored since 1.14.0")
			},
		},
		"copycat": {
			map[string]string{},
			"YOUR-CLOUDFLARE-API-TOKEN", "", "", "", "",
			false, "",
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"You need to provide a real API token as %s", "CLOUDFLARE_API_TOKEN")
			},
		},
		"file/success": {
			map[string]string{"token.txt": "hello"},
			"", "", "token.txt", "", "",
			true, "hello", nil,
		},
		"file/wrong.path": {
			map[string]string{},
			"", "", "wrong.txt", "", "",
			false, "",
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "wrong.txt", gomock.Any())
			},
		},
		"file/empty": {
			map[string]string{"empty.txt": ""},
			"", "", "empty.txt", "", "",
			false, "",
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"The file specified by %s does not contain an API token",
					"CLOUDFLARE_API_TOKEN_FILE")
			},
		},
		"file/invalid-path": {
			map[string]string{"dir/file.txt": ""},
			"", "", "dir", "", "",
			false, "",
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "dir", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "CLOUDFLARE_API_TOKEN", tc.token1)
			store(t, "CLOUDFLARE_API_TOKEN_FILE", tc.fileToken1Path)
			store(t, "CF_API_TOKEN", tc.token2)
			store(t, "CF_API_TOKEN_FILE", tc.fileToken2Path)
			store(t, "CF_ACCOUNT_ID", tc.account)

			mapFS := fstest.MapFS{}
			for path, content := range tc.mapFS {
				mapFS[path] = &fstest.MapFile{
					Data:    []byte(content),
					Mode:    0o644,
					ModTime: time.Unix(1234, 5678),
					Sys:     nil,
				}
			}
			useMemFS(mapFS)

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
