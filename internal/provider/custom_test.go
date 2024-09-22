package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestCustomURLName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "url:(redacted)", provider.Name(provider.MustNewCustomURL("https://1.1.1.1/")))
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
				m.EXPECT().Noticef(pp.EmojiUserError, "Failed to parse the provider url:(redacted)")
			},
		},
		{
			"http://1.2.3.4", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, "The provider url:(redacted) uses HTTP; consider using HTTPS instead")
			},
		},
		{
			"ftp://1.2.3.4", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `The provider url:(redacted) only supports HTTP and HTTPS`)
			},
		},
		{
			"", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `The provider url:(redacted) does not contain a valid URL`)
			},
		},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			p, ok := provider.NewCustomURL(mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.NotNil(t, p)
			} else {
				require.Nil(t, p)
			}
		})
	}
}

func TestMustNewCustom(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		ok    bool
	}{
		{"https://1.2.3.4", true},
		{":::::", false},
		{"http://1.2.3.4", true},
		{"ftp://1.2.3.4", false},
		{"", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			if tc.ok {
				require.NotPanics(t, func() { provider.MustNewCustomURL(tc.input) })
			} else {
				require.Panics(t, func() { provider.MustNewCustomURL(tc.input) })
			}
		})
	}
}
