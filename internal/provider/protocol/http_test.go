package protocol_test

// vim: nowrap

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
		ProviderName:            "very secret name",
		URL:                     nil,
		ForcedTransportIPFamily: nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestHTTPGetRawData(t *testing.T) {
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
	malformed4 := newSplitServer(ipnet.IP4, helloWriter)
	t.Cleanup(malformed4.Close)
	malformed6 := newSplitServer(ipnet.IP6, helloWriter)
	t.Cleanup(malformed6.Close)
	empty4 := newSplitServer(ipnet.IP4, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(empty4.Close)
	commentOnly4 := newSplitServer(ipnet.IP4, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "# just a comment\n")
	}))
	t.Cleanup(commentOnly4.Close)

	for name, tc := range map[string]struct {
		nilCtx        bool
		urlKey        ipnet.Family
		url           string
		ipFamily      ipnet.Family
		transportIP   *ipnet.Family
		expected      netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"nilctx": {
			true, ipnet.IP4, server4.URL, ipnet.IP4, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", server4.URL, gomock.Any())
			},
		},
		"4": {false, ipnet.IP4, server4.URL, ipnet.IP4, nil, ip4, nil},
		"6": {false, ipnet.IP6, server6.URL, ipnet.IP6, nil, ip6, nil},
		"4to6": {
			false, ipnet.IP6, server4via6.URL, ipnet.IP6, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Line %d in the response from %q (%q) %s", 1, server4via6.URL, ip4.String(), "is not a valid IPv6 address")
			},
		},
		"6to4": {
			false, ipnet.IP4, server6via4.URL, ipnet.IP4, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Line %d in the response from %q (%q) %s", 1, server6via4.URL, ip6.String(), "is not a valid IPv4 address")
			},
		},
		"4/malformed": {
			false, ipnet.IP4, malformed4.URL, ipnet.IP4, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to parse line %d in the response from %q (%q) as an IP address or an IP address in CIDR notation", 1, malformed4.URL, "hello")
			},
		},
		"6/malformed": {
			false, ipnet.IP6, malformed6.URL, ipnet.IP6, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to parse line %d in the response from %q (%q) as an IP address or an IP address in CIDR notation", 1, malformed6.URL, "hello")
			},
		},
		"4/empty-response": {
			false, ipnet.IP4, empty4.URL, ipnet.IP4, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "No IP addresses were found in the response from %q", empty4.URL)
			},
		},
		"4/comment-only-response": {
			false, ipnet.IP4, commentOnly4.URL, ipnet.IP4, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "No IP addresses were found in the response from %q", commentOnly4.URL)
			},
		},
		"4/request-fail": {
			false, ipnet.IP4, "", ipnet.IP4, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", "", gomock.Any())
			},
		},
		"6/request-fail": {
			false,
			ipnet.IP6, "", ipnet.IP6, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", "", gomock.Any())
			},
		},
		"ip6-detected-via4-transport": {
			false,
			ipnet.IP6, server6via4.URL, ipnet.IP6, new(ipnet.IP4), ip6,
			nil,
		},
		"ip4-detected-via6-transport": {
			false,
			ipnet.IP4, server4via6.URL, ipnet.IP4, new(ipnet.IP6), ip4,
			nil,
		},
		"4/not-handled": {
			false,
			ipnet.IP4, server4.URL, ipnet.IP6, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", "IPv6")
			},
		},
		"6/not-handled": {
			false,
			ipnet.IP6, server6.URL, ipnet.IP4, nil, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", "IPv4")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			provider := &protocol.HTTP{
				ProviderName: "secret name",
				URL: map[ipnet.Family]string{
					tc.urlKey: tc.url,
				},
				ForcedTransportIPFamily: tc.transportIP,
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

			rawData := provider.GetRawData(ctx, mockPP, tc.ipFamily, map[ipnet.Family]int{
				ipnet.IP4: 32,
				ipnet.IP6: 64,
			}[tc.ipFamily])
			require.Equal(t, tc.expected.IsValid(), rawData.Available)
			if tc.expected.IsValid() {
				require.Equal(t, []ipnet.RawEntry{ipnet.RawEntryFrom(tc.expected, map[ipnet.Family]int{
					ipnet.IP4: 32,
					ipnet.IP6: 64,
				}[tc.ipFamily])}, rawData.RawEntries)
			} else {
				require.Empty(t, rawData.RawEntries)
			}
		})
	}
}

func TestHTTPIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.False(t, protocol.HTTP{
		ProviderName:            "",
		URL:                     map[ipnet.Family]string{},
		ForcedTransportIPFamily: nil,
	}.IsExplicitEmpty())
}
