package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestHTTPName(t *testing.T) {
	t.Parallel()

	p := &provider.HTTP{
		ProviderName: "very secret name",
		URL:          nil,
	}

	require.Equal(t, "very secret name", provider.Name(p))
}

//nolint:funlen
func TestHTTPGetIP(t *testing.T) {
	ip4 := netip.MustParseAddr("1.2.3.4")
	ip4As6 := netip.MustParseAddr("::ffff:1.2.3.4")
	ip6 := netip.MustParseAddr("::1:2:3:4:5:6")
	invalidIP := netip.Addr{}

	ip4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ip4.String())
	}))
	defer ip4Server.Close()
	ip6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ip6.String())
	}))
	defer ip6Server.Close()
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not an ip")
	}))
	defer dummy.Close()

	t.Run("group", func(t *testing.T) {
		for name, tc := range map[string]struct {
			urlKey        ipnet.Type
			url           string
			ipNet         ipnet.Type
			expected      netip.Addr
			prepareMockPP func(*mocks.MockPP)
		}{
			"4":    {ipnet.IP4, ip4Server.URL, ipnet.IP4, ip4, nil},
			"6":    {ipnet.IP6, ip6Server.URL, ipnet.IP6, ip6, nil},
			"4to6": {ipnet.IP6, ip4Server.URL, ipnet.IP6, ip4As6, nil},
			"6to4": {
				ipnet.IP4, ip6Server.URL, ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError, "Detected IP address %s is not a valid %s address",
						ip6.String(),
						"IPv4",
					)
				},
			},
			"4-nil1": {
				ipnet.IP4, dummy.URL, ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Errorf(
						pp.EmojiImpossible,
						`Failed to parse the IP address in the response of %q: %s`,
						dummy.URL,
						"not an ip")
				},
			},
			"6-nil1": {
				ipnet.IP6, dummy.URL, ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Errorf(
						pp.EmojiImpossible,
						`Failed to parse the IP address in the response of %q: %s`,
						dummy.URL,
						"not an ip")
				},
			},
			"4-nil2": {
				ipnet.IP4, "", ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError,
						"Failed to send HTTP(S) request to %q: %v",
						"",
						gomock.Any(),
					)
				},
			},
			"6-nil2": {
				ipnet.IP6, "", ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError,
						"Failed to send HTTP(S) request to %q: %v",
						"",
						gomock.Any(),
					)
				},
			},
			"4-nil3": {
				ipnet.IP4, ip4Server.URL, ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv6")
				},
			},
			"6-nil3": {
				ipnet.IP6, ip6Server.URL, ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv4")
				},
			},
		} {
			tc := tc
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				mockCtrl := gomock.NewController(t)

				provider := &provider.HTTP{
					ProviderName: "secret name",
					URL: map[ipnet.Type]string{
						tc.urlKey: tc.url,
					},
				}

				mockPP := mocks.NewMockPP(mockCtrl)
				if tc.prepareMockPP != nil {
					tc.prepareMockPP(mockPP)
				}
				ip, ok := provider.GetIP(context.Background(), mockPP, tc.ipNet)
				require.Equal(t, tc.expected, ip)
				require.Equal(t, tc.expected.IsValid(), ok)
			})
		}
	})
}
