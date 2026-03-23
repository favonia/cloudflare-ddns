package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestMustNewFile(t *testing.T) {
	t.Parallel()

	t.Run("absolute", func(t *testing.T) {
		t.Parallel()
		p := provider.MustNewFile("/etc/ips.txt")
		require.Equal(t, "file:/etc/ips.txt", provider.Name(p))
	})

	t.Run("relative", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			provider.MustNewFile("relative/path.txt")
		})
	})
}

func TestNewFile(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		path          string
		ok            bool
		expectedName  string
		prepareMockPP func(*mocks.MockPP)
	}{
		"absolute": {
			"/etc/ips.txt", true, "file:/etc/ips.txt", nil,
		},
		"relative": {
			"relative/path.txt", false, "",
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserError,
						"The path %q is not absolute; to use an absolute path, prefix it with /",
						"relative/path.txt"),
					m.EXPECT().Noticef(pp.EmojiHint,
						"Try setting %s=file:%s", "IP4_PROVIDER", "/relative/path.txt"),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			p, ok := provider.NewFile(mockPP, "IP4_PROVIDER", tc.path)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.expectedName, provider.Name(p))
			}
		})
	}
}
