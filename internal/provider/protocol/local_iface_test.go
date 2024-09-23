//go:build linux

package protocol_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestLocalWithInterfaceName(t *testing.T) {
	t.Parallel()

	p := &protocol.LocalWithInterface{
		ProviderName:  "very secret name",
		InterfaceName: "lo",
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestExtractInterfaceAddr(t *testing.T) {
	t.Parallel()

	var invalidIP netip.Addr

	for name, tc := range map[string]struct {
		input         net.Addr
		ok            bool
		output        netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"ipaddr/4": {
			&net.IPAddr{IP: net.ParseIP("127.0.0.1"), Zone: ""},
			true, netip.MustParseAddr("127.0.0.1"),
			nil,
		},
		"ipaddr/6/zone-123": {
			&net.IPAddr{IP: net.ParseIP("::1"), Zone: "123"},
			true, netip.MustParseAddr("::1%123"),
			nil,
		},
		"ipaddr/illformed": {
			&net.IPAddr{IP: net.IP([]byte{0x01, 0x02}), Zone: ""},
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
					"Failed to parse address %q assigned to interface %q",
					"?0102", "iface")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			output, ok := protocol.ExtractInterfaceAddr(mockPP, tc.input, "iface")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestLocalWithInterfaceGetIP(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		interfaceName string
		ipNet         ipnet.Type
		ok            bool
		expected      netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"lo/4": {
			"lo", ipnet.IP4, false,
			netip.Addr{},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to find any global unicast %s address assigned to interface %q",
					"IPv4", "lo")
			},
		},
		"lo/6": {
			"lo", ipnet.IP6, false,
			netip.Addr{},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to find any global unicast %s address assigned to interface %q",
					"IPv6", "lo")
			},
		},
		"non-existent": {
			"non-existent-iface", ipnet.IP4, false,
			netip.Addr{},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiUserError,
					"Failed to find an interface named %q: %v",
					"non-existent-iface", gomock.Any(),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			provider := &protocol.LocalWithInterface{
				ProviderName:  "",
				InterfaceName: tc.interfaceName,
			}
			ip, method, ok := provider.GetIP(context.Background(), mockPP, tc.ipNet)
			require.Equal(t, tc.ok, ok)
			require.NotEqual(t, protocol.MethodAlternative, method)
			require.Equal(t, tc.expected, ip)
		})
	}
}
