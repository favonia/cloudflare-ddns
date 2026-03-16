package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestCustomURLName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "url:(redacted)", provider.Name(provider.MustNewCustomURL("https://1.1.1.1/")))
	require.Equal(t, "url.via4:(redacted)", provider.Name(provider.MustNewCustomURLVia4("https://1.1.1.1/")))
	require.Equal(t, "url.via6:(redacted)", provider.Name(provider.MustNewCustomURLVia6("https://1.1.1.1/")))
}

func TestNewCustom(t *testing.T) {
	t.Parallel()

	envKey := "IP4_PROVIDER"

	for _, tc := range []struct {
		name                      string
		create                    func(pp.PP, string, string) (provider.Provider, bool)
		input                     string
		ok                        bool
		expectedProviderName      string
		expectedTransportIPFamily *ipnet.Family
		prepareMockPP             func(*mocks.MockPP)
	}{
		{
			name:                      "strict/https",
			create:                    provider.NewCustomURL,
			input:                     "https://1.2.3.4",
			ok:                        true,
			expectedProviderName:      "url:(redacted)",
			expectedTransportIPFamily: nil,
			prepareMockPP:             nil,
		},
		{
			name:                      "via4/http",
			create:                    provider.NewCustomURLVia4,
			input:                     "http://1.2.3.4",
			ok:                        true,
			expectedProviderName:      "url.via4:(redacted)",
			expectedTransportIPFamily: new(ipnet.IP4),
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, "%s=%s uses HTTP; consider using HTTPS instead", envKey, "url.via4:(redacted)")
			},
		},
		{
			name:                      "via6/https",
			create:                    provider.NewCustomURLVia6,
			input:                     "https://1.2.3.4",
			ok:                        true,
			expectedProviderName:      "url.via6:(redacted)",
			expectedTransportIPFamily: new(ipnet.IP6),
			prepareMockPP:             nil,
		},
		{
			name:                      "strict/parse-error",
			create:                    provider.NewCustomURL,
			input:                     ":::::",
			ok:                        false,
			expectedProviderName:      "",
			expectedTransportIPFamily: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s: failed to parse the URL", envKey, "url:(redacted)")
			},
		},
		{
			name:                      "strict/relative-url",
			create:                    provider.NewCustomURL,
			input:                     "/detect-ip",
			ok:                        false,
			expectedProviderName:      "",
			expectedTransportIPFamily: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s does not contain a valid URL", envKey, "url:(redacted)")
			},
		},
		{
			name:                      "via4/opaque-url",
			create:                    provider.NewCustomURLVia4,
			input:                     "https:1.2.3.4",
			ok:                        false,
			expectedProviderName:      "",
			expectedTransportIPFamily: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s does not contain a valid URL", envKey, "url.via4:(redacted)")
			},
		},
		{
			name:                      "via6/missing-host",
			create:                    provider.NewCustomURLVia6,
			input:                     "https:///detect-ip",
			ok:                        false,
			expectedProviderName:      "",
			expectedTransportIPFamily: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s does not contain a valid URL", envKey, "url.via6:(redacted)")
			},
		},
		{
			name:                      "via6/unsupported-scheme",
			create:                    provider.NewCustomURLVia6,
			input:                     "ftp://1.2.3.4",
			ok:                        false,
			expectedProviderName:      "",
			expectedTransportIPFamily: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s only supports HTTP and HTTPS", envKey, "url.via6:(redacted)")
			},
		},
		{
			name:                      "strict/empty",
			create:                    provider.NewCustomURL,
			input:                     "",
			ok:                        false,
			expectedProviderName:      "",
			expectedTransportIPFamily: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s does not contain a valid URL", envKey, "url:(redacted)")
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
			p, ok := tc.create(mockPP, envKey, tc.input)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.NotNil(t, p)
				httpProvider, ok := p.(protocol.HTTP)
				require.True(t, ok)
				require.Equal(t, tc.expectedProviderName, httpProvider.ProviderName)
				require.Equal(t, tc.input, httpProvider.URL[ipnet.IP4])
				require.Equal(t, tc.input, httpProvider.URL[ipnet.IP6])
				if tc.expectedTransportIPFamily == nil {
					require.Nil(t, httpProvider.ForcedTransportIPFamily)
				} else {
					require.NotNil(t, httpProvider.ForcedTransportIPFamily)
					require.Equal(t, *tc.expectedTransportIPFamily, *httpProvider.ForcedTransportIPFamily)
				}
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
		{"via4/parse-error", provider.MustNewCustomURLVia4, ":::::", false},
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
