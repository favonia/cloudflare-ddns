package protocol_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestHTTPName(t *testing.T) {
	t.Parallel()

	p := &protocol.HTTP{
		ProviderName:     "very secret name",
		Is1111UsedForIP4: false,
		URL:              nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

//nolint:funlen
func TestHTTPGetIP(t *testing.T) {
	ip4 := netip.MustParseAddr("1.2.3.4")
	ip4As6 := netip.MustParseAddr("::ffff:1.2.3.4")
	ip6 := netip.MustParseAddr("::1:2:3:4:5:6")
	invalidIP := netip.Addr{}

	ip4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ip4.String())
	}))
	defer ip4Server.Close()
	ip6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ip6.String())
	}))
	defer ip6Server.Close()
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not an ip")
	}))
	defer dummy.Close()

	t.Run("group", func(t *testing.T) {
		for name, tc := range map[string]struct {
			nilCtx        bool
			urlKey        ipnet.Type
			url           string
			ipNet         ipnet.Type
			expected      netip.Addr
			prepareMockPP func(*mocks.MockPP)
		}{
			"4":    {false, ipnet.IP4, ip4Server.URL, ipnet.IP4, ip4, nil},
			"6":    {false, ipnet.IP6, ip6Server.URL, ipnet.IP6, ip6, nil},
			"4to6": {false, ipnet.IP6, ip4Server.URL, ipnet.IP6, ip4As6, nil},
			"nilctx": {
				true, ipnet.IP4, ip4Server.URL, ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(
						pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v",
						ip4Server.URL,
						gomock.Any(),
					)
				},
			},
			"6to4": {
				false, ipnet.IP4, ip6Server.URL, ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", ip6.String())
				},
			},
			"4-nil1": {
				false, ipnet.IP4, dummy.URL, ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(
						pp.EmojiImpossible,
						`Failed to parse the IP address in the response of %q: %s`,
						dummy.URL,
						"not an ip")
				},
			},
			"6-nil1": {
				false, ipnet.IP6, dummy.URL, ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(
						pp.EmojiImpossible,
						`Failed to parse the IP address in the response of %q: %s`,
						dummy.URL,
						"not an ip")
				},
			},
			"4-nil2": {
				false, ipnet.IP4, "", ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(
						pp.EmojiError,
						"Failed to send HTTP(S) request to %q: %v",
						"",
						gomock.Any(),
					)
				},
			},
			"6-nil2": {
				false, ipnet.IP6, "", ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(
						pp.EmojiError,
						"Failed to send HTTP(S) request to %q: %v",
						"",
						gomock.Any(),
					)
				},
			},
			"4-nil3": {
				false, ipnet.IP4, ip4Server.URL, ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv6")
				},
			},
			"6-nil3": {
				false, ipnet.IP6, ip6Server.URL, ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv4")
				},
			},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				mockCtrl := gomock.NewController(t)

				provider := &protocol.HTTP{
					ProviderName:     "secret name",
					Is1111UsedForIP4: false,
					URL: map[ipnet.Type]protocol.Switch{
						tc.urlKey: protocol.Constant(tc.url),
					},
				}
				ctx := context.Background()
				if tc.nilCtx {
					ctx = nil
				}

				mockPP := mocks.NewMockPP(mockCtrl)
				if tc.prepareMockPP != nil {
					tc.prepareMockPP(mockPP)
				}
				ip, ok := provider.GetIP(ctx, mockPP, tc.ipNet, true)
				require.Equal(t, tc.expected, ip)
				require.Equal(t, tc.expected.IsValid(), ok)
			})
		}
	})
}

func TestHTTPShouldWeCheck1111(t *testing.T) {
	t.Parallel()

	require.True(t, (&protocol.HTTP{
		ProviderName:     "",
		Is1111UsedForIP4: true,
		URL:              nil,
	}).ShouldWeCheck1111())

	require.False(t, (&protocol.HTTP{
		ProviderName:     "",
		Is1111UsedForIP4: false,
		URL:              nil,
	}).ShouldWeCheck1111())
}
