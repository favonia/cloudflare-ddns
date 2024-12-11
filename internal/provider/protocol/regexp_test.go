// vim: nowrap
package protocol_test

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"testing"
	"time"

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

func TestRegexpGetIP(t *testing.T) {
	t.Parallel()

	ip4 := netip.MustParseAddr("1.2.3.4")
	ip6 := netip.MustParseAddr("::1:2:3:4:5:6")
	invalidIP := netip.Addr{}

	ip4Writer := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<<"+ip4.String()+">>")
	})
	ip6Writer := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<<"+ip6.String()+">>")
	})
	helloWriter := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<<hello>>")
	})

	server4 := newSplitServer(ipnet.IP4, ip4Writer)
	t.Cleanup(server4.Close)
	server6 := newSplitServer(ipnet.IP6, ip6Writer)
	t.Cleanup(server6.Close)
	server6via4 := newSplitServer(ipnet.IP4, ip6Writer)
	t.Cleanup(server6via4.Close)
	server4via6 := newSplitServer(ipnet.IP6, ip4Writer)
	t.Cleanup(server4via6.Close)
	illformed4 := newSplitServer(ipnet.IP4, helloWriter)
	t.Cleanup(illformed4.Close)
	illformed6 := newSplitServer(ipnet.IP6, helloWriter)
	t.Cleanup(illformed6.Close)

	for name, tc := range map[string]struct {
		urlKey        ipnet.Type
		url           string
		regexp        *regexp.Regexp
		ipNet         ipnet.Type
		expected      netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"4": {ipnet.IP4, server4.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP4, ip4, nil},
		"6": {ipnet.IP6, server6.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP6, ip6, nil},
		"4to6": {
			ipnet.IP6, server4via6.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv6 address", ip4.String())
			},
		},
		"6to4": {
			ipnet.IP4, server6via4.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", ip6.String())
			},
		},
		"4/illformed": {
			ipnet.IP4, illformed4.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, `Failed to parse the IP address in the response of %q (%q)`, illformed4.URL, "hello")
			},
		},
		"6/illformed": {
			ipnet.IP6, illformed6.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, `Failed to parse the IP address in the response of %q (%q)`, illformed6.URL, "hello")
			},
		},
		"4/request-fail": {
			ipnet.IP4, "", regexp.MustCompile(``), ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", "", gomock.Any())
			},
		},
		"6/request-fail": {
			ipnet.IP6, "", regexp.MustCompile(``), ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", "", gomock.Any())
			},
		},
		"4/not-handled": {
			ipnet.IP4, server4.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv6")
			},
		},
		"6/not-handled": {
			ipnet.IP6, server6.URL, regexp.MustCompile(`<<(.*)>>`), ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv4")
			},
		},
		"4/no-match": {
			ipnet.IP4, illformed4.URL, regexp.MustCompile(`some random string`), ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, `Failed to find the IP address in the response of %q (%q)`, illformed4.URL, []byte("<<hello>>"))
			},
		},
		"6/no-match": {
			ipnet.IP6, illformed6.URL, regexp.MustCompile(`some random string`), ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, `Failed to find the IP address in the response of %q (%q)`, illformed6.URL, []byte("<<hello>>"))
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
						URL:    tc.url,
						Regexp: tc.regexp,
					},
				},
			}

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			ip, ok := provider.GetIP(ctx, mockPP, tc.ipNet)
			require.Equal(t, tc.expected, ip)
			require.Equal(t, tc.expected.IsValid(), ok)
		})
	}
}
