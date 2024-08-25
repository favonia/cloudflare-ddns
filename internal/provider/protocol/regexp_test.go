package protocol_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestRegexpName(t *testing.T) {
	t.Parallel()

	p := &protocol.Regexp{
		ProviderName: "very secret name",
		Param:        nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

//nolint:funlen
func TestRegexpGetIP(t *testing.T) {
	ip4 := netip.MustParseAddr("1.2.3.4")
	ip4As6 := netip.MustParseAddr("::ffff:1.2.3.4")
	ip6 := netip.MustParseAddr("::1:2:3:4:5:6")
	invalidIP := netip.Addr{}

	ip4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<<"+ip4.String()+">>")
	}))
	defer ip4Server.Close()
	ip6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<<"+ip6.String()+">>")
	}))
	defer ip6Server.Close()
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<<hello>>")
	}))
	defer dummy.Close()

	t.Run("group", func(t *testing.T) {
		for name, tc := range map[string]struct {
			urlKey        ipnet.Type
			url           string
			regexp        *regexp.Regexp
			ipNet         ipnet.Type
			expected      netip.Addr
			prepareMockPP func(*mocks.MockPP)
		}{
			"4":    {ipnet.IP4, ip4Server.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP4, ip4, nil},
			"6":    {ipnet.IP6, ip6Server.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP6, ip6, nil},
			"4to6": {ipnet.IP6, ip4Server.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP6, ip4As6, nil},
			"6to4": {
				ipnet.IP4, ip6Server.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", ip6.String())
				},
			},
			"4-nil1": {
				ipnet.IP4, dummy.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(
						pp.EmojiError,
						`Failed to parse the IP address in the response of %q: %s`,
						dummy.URL,
						"hello")
				},
			},
			"6-nil1": {
				ipnet.IP6, dummy.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(
						pp.EmojiError,
						`Failed to parse the IP address in the response of %q: %s`,
						dummy.URL,
						"hello")
				},
			},
			"4-nil2": {
				ipnet.IP4, "", regexp.MustCompile(``), ipnet.IP4, invalidIP,
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
				ipnet.IP6, "", regexp.MustCompile(``), ipnet.IP6, invalidIP,
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
				ipnet.IP4, ip4Server.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv6")
				},
			},
			"6-nil3": {
				ipnet.IP6, ip6Server.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv4")
				},
			},
			"4-nil4": {
				ipnet.IP4, dummy.URL, regexp.MustCompile(`some random string`), ipnet.IP4, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(pp.EmojiError,
						`Failed to find the IP address in the response of %q: %s`,
						dummy.URL,
						[]byte("<<hello>>"))
				},
			},
			"6-nil4": {
				ipnet.IP6, dummy.URL, regexp.MustCompile(`some random string`), ipnet.IP6, invalidIP,
				func(m *mocks.MockPP) {
					m.EXPECT().Noticef(pp.EmojiError,
						`Failed to find the IP address in the response of %q: %s`,
						dummy.URL,
						[]byte("<<hello>>"))
				},
			},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				mockCtrl := gomock.NewController(t)

				provider := &protocol.Regexp{
					ProviderName: "secret name",
					Param: map[ipnet.Type]protocol.RegexpParam{
						tc.urlKey: {
							URL:    protocol.Constant(tc.url),
							Regexp: tc.regexp,
						},
					},
				}

				mockPP := mocks.NewMockPP(mockCtrl)
				if tc.prepareMockPP != nil {
					tc.prepareMockPP(mockPP)
				}
				ip, ok := provider.GetIP(context.Background(), mockPP, tc.ipNet, protocol.MethodPrimary)
				require.Equal(t, tc.expected, ip)
				require.Equal(t, tc.expected.IsValid(), ok)
			})
		}
	})
}

func TestRegexpHasAlternative(t *testing.T) {
	t.Parallel()

	require.True(t, (&protocol.Regexp{
		ProviderName: "",
		Param: map[ipnet.Type]protocol.RegexpParam{
			ipnet.IP4: {
				URL:    protocol.Switchable{}, //nolint:exhaustruct
				Regexp: regexp.MustCompile(``),
			},
		},
	}).HasAlternative(ipnet.IP4))
}
