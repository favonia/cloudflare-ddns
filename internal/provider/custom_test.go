package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestCustomName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "custom", provider.Name(provider.MustNewCustom("https://1.1.1.1/")))
}

func TestNewCustom(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input         string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		{"https://1.2.3.4", true, nil},
		{
			":::::", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse the custom provider (redacted)")
			},
		},
		{
			"http://1.2.3.4", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning, "The custom provider (redacted) uses HTTP; consider using HTTPS instead")
			},
		},
		{
			"ftp://1.2.3.4", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `The custom provider (redacted) must use HTTP or HTTPS`)
			},
		},
		{
			"", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `The custom provider (redacted) does not look like a valid URL`)
			},
		},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			p, ok := provider.NewCustom(mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.NotNil(t, p)
			} else {
				require.Nil(t, p)
			}
		})
	}
}
