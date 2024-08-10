package protocol_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestLocalName(t *testing.T) {
	t.Parallel()

	p := &protocol.Local{
		ProviderName:     "very secret name",
		Is1111UsedForIP4: false,
		RemoteUDPAddr:    nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

//nolint:funlen
func TestLocalGetIP(t *testing.T) {
	t.Parallel()

	ip4Loopback := netip.MustParseAddr("127.0.0.1")
	ip6Loopback := netip.MustParseAddr("::1")
	invalidIP := netip.Addr{}

	for name, tc := range map[string]struct {
		addrKey       ipnet.Type
		addr          string
		ipNet         ipnet.Type
		expected      gomock.Matcher
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"4": {
			ipnet.IP4, "127.0.0.1:80", ipnet.IP4, gomock.Eq(ip4Loopback), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning,
					"Detected IP address %s does not look like a global unicast IP address.", "127.0.0.1")
			},
		},
		"6": {
			ipnet.IP6, "[::1]:80", ipnet.IP6,
			gomock.AnyOf(
				ip6Loopback,
				gomock.Cond(func(x any) bool {
					a, ok := x.(netip.Addr)
					if !ok {
						return false
					}
					return a.IsLinkLocalUnicast()
				})),
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning,
					"Detected IP address %s does not look like a global unicast IP address.",
					gomock.AnyOf(
						"::1",
						gomock.Cond(func(x any) bool {
							s, ok := x.(string)
							if !ok {
								return false
							}
							return netip.MustParseAddr(s).IsLinkLocalUnicast()
						})))
			},
		},
		"4-nil1": {
			ipnet.IP4, "", ipnet.IP4, gomock.Eq(invalidIP), false,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiError, "Failed to detect a local %s address: %v", "IPv4", gomock.Any())
			},
		},
		"6-nil1": {
			ipnet.IP6, "", ipnet.IP6, gomock.Eq(invalidIP), false,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiError, "Failed to detect a local %s address: %v", "IPv6", gomock.Any())
			},
		},
		"4-nil2": {
			ipnet.IP4, "127.0.0.1:80", ipnet.IP6, gomock.Eq(invalidIP), false,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv6")
			},
		},
		"6-nil2": {
			ipnet.IP6, "::1:80", ipnet.IP4, gomock.Eq(invalidIP), false,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv4")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			provider := &protocol.Local{
				ProviderName:     "",
				Is1111UsedForIP4: false,
				RemoteUDPAddr: map[ipnet.Type]protocol.Switch{
					tc.addrKey: protocol.Constant(tc.addr),
				},
			}

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ip, ok := provider.GetIP(context.Background(), mockPP, tc.ipNet, true)
			require.True(t, tc.expected.Matches(ip))
			require.Equal(t, tc.ok, ok)
		})
	}
}

func TestLocalShouldWeCheck1111(t *testing.T) {
	t.Parallel()

	require.True(t, (&protocol.Local{
		ProviderName:     "",
		Is1111UsedForIP4: true,
		RemoteUDPAddr:    nil,
	}).ShouldWeCheck1111())

	require.False(t, (&protocol.Local{
		ProviderName:     "",
		Is1111UsedForIP4: false,
		RemoteUDPAddr:    nil,
	}).ShouldWeCheck1111())
}
