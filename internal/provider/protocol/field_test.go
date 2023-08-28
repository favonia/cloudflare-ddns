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

func TestFieldName(t *testing.T) {
	t.Parallel()

	p := &protocol.Field{
		ProviderName:     "very secret name",
		Is1111UsedforIP4: false,
		Param:            nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

//nolint:funlen
func TestFieldGetIP(t *testing.T) {
	ip4 := netip.MustParseAddr("1.2.3.4")
	ip4As6 := netip.MustParseAddr("::ffff:1.2.3.4")
	ip6 := netip.MustParseAddr("::1:2:3:4:5:6")
	invalidIP := netip.Addr{}

	ip4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hi=123\nhello4="+ip4.String()+"\naloha=456")
	}))
	defer ip4Server.Close()
	ip6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hi=123\nhello6="+ip6.String()+"\naloha=456")
	}))
	defer ip6Server.Close()
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ip=none")
	}))
	defer dummy.Close()

	t.Run("group", func(t *testing.T) {
		for name, tc := range map[string]struct {
			urlKey        ipnet.Type
			url           string
			field         string
			ipNet         ipnet.Type
			expected      netip.Addr
			prepareMockPP func(*mocks.MockPP)
		}{
			"4":    {ipnet.IP4, ip4Server.URL, "hello4", ipnet.IP4, ip4, nil},
			"6":    {ipnet.IP6, ip6Server.URL, "hello6", ipnet.IP6, ip6, nil},
			"4to6": {ipnet.IP6, ip4Server.URL, "hello4", ipnet.IP6, ip4As6, nil},
			"6to4": {
				ipnet.IP4, ip6Server.URL, "hello6", ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError, "Detected IP address %s is not a valid %s address",
						ip6.String(),
						"IPv4",
					)
				},
			},
			"4-nil1": {
				ipnet.IP4, dummy.URL, "ip", ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError,
						`Failed to parse the IP address in the response of %q: %s`,
						dummy.URL,
						"none")
				},
			},
			"6-nil1": {
				ipnet.IP6, dummy.URL, "ip", ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError,
						`Failed to parse the IP address in the response of %q: %s`,
						dummy.URL,
						"none")
				},
			},
			"4-nil2": {
				ipnet.IP4, "", "", ipnet.IP4, invalidIP,
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
				ipnet.IP6, "", "", ipnet.IP6, invalidIP,
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
				ipnet.IP4, ip4Server.URL, "hello4", ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv6")
				},
			},
			"6-nil3": {
				ipnet.IP6, ip6Server.URL, "hello6", ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv4")
				},
			},
			"4-nil4": {
				ipnet.IP4, dummy.URL, "nonexisting4", ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(pp.EmojiError,
						`Failed to find the IP address in the response of %q: %s`,
						dummy.URL,
						[]byte("ip=none"))
				},
			},
			"6-nil4": {
				ipnet.IP6, dummy.URL, "nonexisting6", ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(pp.EmojiError,
						`Failed to find the IP address in the response of %q: %s`,
						dummy.URL,
						[]byte("ip=none"))
				},
			},
		} {
			tc := tc
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				mockCtrl := gomock.NewController(t)

				provider := &protocol.Field{
					ProviderName:     "secret name",
					Is1111UsedforIP4: false,
					Param: map[ipnet.Type]struct {
						URL   protocol.Switch
						Field string
					}{
						tc.urlKey: {
							URL:   protocol.Constant(tc.url),
							Field: tc.field,
						},
					},
				}

				mockPP := mocks.NewMockPP(mockCtrl)
				if tc.prepareMockPP != nil {
					tc.prepareMockPP(mockPP)
				}
				ip, ok := provider.GetIP(context.Background(), mockPP, tc.ipNet, true)
				require.Equal(t, tc.expected, ip)
				require.Equal(t, tc.expected.IsValid(), ok)
			})
		}
	})
}

func TestFieldShouldWeCheck1111(t *testing.T) {
	t.Parallel()

	require.True(t, (&protocol.Field{
		ProviderName:     "",
		Is1111UsedforIP4: true,
		Param:            nil,
	}).ShouldWeCheck1111())

	require.False(t, (&protocol.Field{
		ProviderName:     "",
		Is1111UsedforIP4: false,
		Param:            nil,
	}).ShouldWeCheck1111())
}
