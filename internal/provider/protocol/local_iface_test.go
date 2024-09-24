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

type Dummy struct{}

func (*Dummy) Network() string { return "dummy/network" }
func (*Dummy) String() string  { return "dummy/string" }

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
			&net.IPAddr{IP: net.ParseIP("1.2.3.4"), Zone: ""},
			true, netip.MustParseAddr("1.2.3.4"),
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
					"Failed to parse address %q assigned to interface %s",
					"?0102", "iface")
			},
		},
		"ipnet/4": {
			&net.IPNet{IP: net.ParseIP("1.2.3.4"), Mask: net.CIDRMask(10, 22)},
			true, netip.MustParseAddr("1.2.3.4"),
			nil,
		},
		"ipnet/illformed": {
			&net.IPNet{IP: net.IP([]byte{0x01, 0x02}), Mask: net.CIDRMask(10, 22)},
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
					"Failed to parse address %q assigned to interface %s",
					"?0102", "iface")
			},
		},
		"dummy": {
			&Dummy{},
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
					"Unexpected address data %q of type %T found in interface %s",
					"dummy/string", &Dummy{}, "iface")
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

			output, ok := protocol.ExtractInterfaceAddr(mockPP, "iface", tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestSelectInterfaceIP(t *testing.T) {
	t.Parallel()

	var invalidIP netip.Addr

	for name, tc := range map[string]struct {
		ipNet         ipnet.Type
		input         []net.Addr
		ok            bool
		method        protocol.Method
		output        netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"ipaddr/4/6+4": {
			ipnet.IP4,
			[]net.Addr{
				&net.IPAddr{IP: net.ParseIP("1::1"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("1.2.3.4"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("4.3.2.1"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("2::2"), Zone: ""},
			},
			true, protocol.MethodPrimary, netip.MustParseAddr("1.2.3.4"),
			nil,
		},
		"ipaddr/4/none": {
			ipnet.IP4,
			[]net.Addr{&net.IPAddr{IP: net.ParseIP("1::1"), Zone: ""}, &net.IPAddr{IP: net.ParseIP("2::2"), Zone: ""}},
			false, protocol.MethodUnspecified, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to find any global unicast %s address assigned to interface %s",
					"IPv4", "iface",
				)
			},
		},
		"ipaddr/4/loopback": {
			ipnet.IP4,
			[]net.Addr{&net.IPAddr{IP: net.ParseIP("127.0.0.1"), Zone: ""}},
			false, protocol.MethodUnspecified, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to find any global unicast %s address assigned to interface %s",
					"IPv4", "iface",
				)
			},
		},
		"ipaddr/6/ff05::2": {
			ipnet.IP6,
			[]net.Addr{&net.IPAddr{IP: net.ParseIP("ff05::2"), Zone: "site"}},
			true, protocol.MethodPrimary, netip.MustParseAddr("ff05::2%site"),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiWarning,
					"Failed to find any global unicast %s address assigned to interface %s, "+
						"but found an address %s with a scope larger than the link-local scope",
					"IPv6", "iface", "ff05::2%site",
				)
			},
		},
		"ipaddr/4/dummy": {
			ipnet.IP6,
			[]net.Addr{&Dummy{}},
			false, protocol.MethodUnspecified, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
					"Unexpected address data %q of type %T found in interface %s",
					"dummy/string", &Dummy{}, "iface")
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

			output, method, ok := protocol.SelectInterfaceIP(mockPP, "iface", tc.ipNet, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.method, method)
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
					"Failed to find any global unicast %s address assigned to interface %s",
					"IPv4", "lo")
			},
		},
		"lo/6": {
			"lo", ipnet.IP6, false,
			netip.Addr{},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to find any global unicast %s address assigned to interface %s",
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
