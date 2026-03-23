package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

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
				m.EXPECT().Noticef(pp.EmojiUserError,
					"The path %q in %s is not absolute; use an absolute path",
					"relative/path.txt", "IP4_PROVIDER")
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
