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
	require.Equal(t, "url.via4:(redacted)", provider.Name(provider.MustNewCustomURLVia4("https://1.1.1.1/")))
	require.Equal(t, "url.via6:(redacted)", provider.Name(provider.MustNewCustomURLVia6("https://1.1.1.1/")))
}

func TestNewCustom(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		create        func(pp.PP, string) (provider.Provider, bool)
		input         string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		{"strict/https", provider.NewCustomURL, "https://1.2.3.4", true, nil},
		{"via4/https", provider.NewCustomURLVia4, "https://1.2.3.4", true, nil},
		{"via6/https", provider.NewCustomURLVia6, "https://1.2.3.4", true, nil},
		{
			"strict/parse-error", provider.NewCustomURL, ":::::", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "Failed to parse the provider %s", "url:(redacted)")
			},
		},
		{
			"via4/http", provider.NewCustomURLVia4, "http://1.2.3.4", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, "The provider %s uses HTTP; consider using HTTPS instead", "url.via4:(redacted)")
			},
		},
		{
			"via6/unsupported-scheme", provider.NewCustomURLVia6, "ftp://1.2.3.4", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "The provider %s only supports HTTP and HTTPS", "url.via6:(redacted)")
			},
		},
		{
			"strict/empty", provider.NewCustomURL, "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "The provider %s does not contain a valid URL", "url:(redacted)")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			p, ok := tc.create(mockPP, tc.input)
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
		name   string
		create func(string) provider.Provider
		input  string
		ok     bool
	}{
		{"strict/https", provider.MustNewCustomURL, "https://1.2.3.4", true},
		{"via4/https", provider.MustNewCustomURLVia4, "https://1.2.3.4", true},
		{"via6/https", provider.MustNewCustomURLVia6, "https://1.2.3.4", true},
		{"strict/parse-error", provider.MustNewCustomURL, ":::::", false},
		{"via4/http", provider.MustNewCustomURLVia4, "http://1.2.3.4", true},
		{"via6/unsupported-scheme", provider.MustNewCustomURLVia6, "ftp://1.2.3.4", false},
		{"strict/empty", provider.MustNewCustomURL, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.ok {
				require.NotPanics(t, func() { tc.create(tc.input) })
			} else {
				require.Panics(t, func() { tc.create(tc.input) })
			}
		})
	}
}
