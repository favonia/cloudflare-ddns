// vim: nowrap
package protocol_test

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"testing"
	"time"

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
		ProviderName: "very secret name",
		URL:          nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestHTTPGetIP(t *testing.T) {
	t.Parallel()

	ip4 := netip.MustParseAddr("1.2.3.4")
	ip6 := netip.MustParseAddr("::1:2:3:4:5:6")
	invalidIP := netip.Addr{}

	ip4Writer := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ip4.String())
	})
	ip6Writer := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ip6.String())
	})
	helloWriter := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "hello")
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
		nilCtx        bool
		urlKey        ipnet.Type
		url           string
		ipNet         ipnet.Type
		expected      netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"nilctx": {
			true, ipnet.IP4, server4.URL, ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", server4.URL, gomock.Any())
			},
		},
		"4": {false, ipnet.IP4, server4.URL, ipnet.IP4, ip4, nil},
		"6": {false, ipnet.IP6, server6.URL, ipnet.IP6, ip6, nil},
		"4to6": {
			false,
			ipnet.IP6, server4via6.URL, ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv6 address", ip4.String())
			},
		},
		"6to4": {
			false,
			ipnet.IP4, server6via4.URL, ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", ip6.String())
			},
		},
		"4/illformed": {
			false,
			ipnet.IP4, illformed4.URL, ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, `Failed to parse the IP address in the response of %q: %s`, illformed4.URL, "hello")
			},
		},
		"6/illformed": {
			false,
			ipnet.IP6, illformed6.URL, ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, `Failed to parse the IP address in the response of %q: %s`, illformed6.URL, "hello")
			},
		},
		"4/request-fail": {
			false,
			ipnet.IP4, "", ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", "", gomock.Any())
			},
		},
		"6/request-fail": {
			false,
			ipnet.IP6, "", ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", "", gomock.Any())
			},
		},
		"4/not-handled": {
			false,
			ipnet.IP4, server4.URL, ipnet.IP6, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv6")
			},
		},
		"6/not-handled": {
			false,
			ipnet.IP6, server6.URL, ipnet.IP4, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv4")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			provider := &protocol.HTTP{
				ProviderName: "secret name",
				URL: map[ipnet.Type]protocol.Switch{
					tc.urlKey: protocol.Constant(tc.url),
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if tc.nilCtx {
				ctx = nil
			}

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			ip, ok := provider.GetIP(ctx, mockPP, tc.ipNet, protocol.MethodPrimary)
			require.Equal(t, tc.expected, ip)
			require.Equal(t, tc.expected.IsValid(), ok)
		})
	}
}

func TestHTTPHasAlternative(t *testing.T) {
	t.Parallel()

	require.True(t, (&protocol.HTTP{
		ProviderName: "",
		URL: map[ipnet.Type]protocol.Switch{
			ipnet.IP4: protocol.Switchable{}, //nolint:exhaustruct
		},
	}).HasAlternative(ipnet.IP4))
}
