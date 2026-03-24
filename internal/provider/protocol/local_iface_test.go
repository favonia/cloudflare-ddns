// vim: nowrap

// Linux only: TestLocalWithInterfaceGetRawData calls net.InterfaceByName("lo"),
// and the loopback interface name "lo" is Linux-specific (e.g., macOS uses "lo0").
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
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse address %q assigned to interface %s", "?0102", "iface")
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
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse address %q assigned to interface %s", "?0102", "iface")
			},
		},
		"dummy": {
			&Dummy{},
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Unexpected address data %q of type %T found in interface %s", "dummy/string", &Dummy{}, "iface")
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

func TestSelectAndNormalizeInterfaceIPs(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ipFamily         ipnet.Family
		defaultPrefixLen int
		input            []net.Addr
		output           protocol.DetectionResult
		prepareMockPP    func(*mocks.MockPP)
	}{
		"ipaddr/4/multiple-global": {
			ipnet.IP4, 32,
			[]net.Addr{
				&net.IPAddr{IP: net.ParseIP("1::1"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("4.3.2.1"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("1.2.3.4"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("2::2"), Zone: ""},
			},
			protocol.NewKnownDetectionResult([]ipnet.RawEntry{
				mustRawEntry("1.2.3.4/32"),
				mustRawEntry("4.3.2.1/32"),
			}),
			nil,
		},
		"ipaddr/4/duplicates": {
			ipnet.IP4, 32,
			[]net.Addr{
				&net.IPAddr{IP: net.ParseIP("4.3.2.1"), Zone: ""},
				&net.IPNet{IP: net.ParseIP("1.2.3.4"), Mask: net.CIDRMask(10, 22)},
				&net.IPAddr{IP: net.ParseIP("::ffff:4.3.2.1"), Zone: ""},
				&net.IPNet{IP: net.ParseIP("4.3.2.1"), Mask: net.CIDRMask(10, 22)},
			},
			protocol.NewKnownDetectionResult([]ipnet.RawEntry{
				mustRawEntry("1.2.3.4/32"),
				mustRawEntry("4.3.2.1/32"),
			}),
			nil,
		},
		"ipaddr/6/mixed-scopes": {
			ipnet.IP6, 64,
			[]net.Addr{
				&net.IPAddr{IP: net.ParseIP("::1"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("fe80::1"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("2001:db8::1"), Zone: "eth0"},
				&net.IPAddr{IP: net.ParseIP("2001:db8::3"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("1.2.3.4"), Zone: ""},
			},
			protocol.NewKnownDetectionResult([]ipnet.RawEntry{
				mustRawEntry("2001:db8::3/64"),
			}),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiWarning,
					"Ignoring zoned address %s assigned to interface %s",
					"2001:db8::1%eth0", "iface",
				)
			},
		},
		"ipaddr/4/no-global-matches": {
			ipnet.IP4, 32,
			[]net.Addr{&net.IPAddr{IP: net.ParseIP("1::1"), Zone: ""}, &net.IPAddr{IP: net.ParseIP("2::2"), Zone: ""}},
			protocol.NewUnavailableDetectionResult(),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find any global unicast %s address among unicast addresses assigned to interface %s", "IPv4", "iface")
			},
		},
		"ipaddr/6/ignore-zoned": {
			ipnet.IP6, 64,
			[]net.Addr{
				&net.IPAddr{IP: net.ParseIP("1::1"), Zone: "eth0"},
				&net.IPAddr{IP: net.ParseIP("2::2"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("3::3"), Zone: ""},
			},
			protocol.NewKnownDetectionResult([]ipnet.RawEntry{
				mustRawEntry("2::2/64"),
				mustRawEntry("3::3/64"),
			}),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiWarning,
					"Ignoring zoned address %s assigned to interface %s",
					"1::1%eth0", "iface",
				)
			},
		},
		"ipaddr/6/all-zoned": {
			ipnet.IP6, 64,
			[]net.Addr{
				&net.IPAddr{IP: net.ParseIP("1::1"), Zone: "eth0"},
				&net.IPAddr{IP: net.ParseIP("2::2"), Zone: "eth1"},
			},
			protocol.NewUnavailableDetectionResult(),
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(
						pp.EmojiWarning,
						"Ignoring zoned address %s assigned to interface %s",
						"1::1%eth0", "iface",
					),
					ppfmt.EXPECT().Noticef(
						pp.EmojiWarning,
						"Ignoring zoned address %s assigned to interface %s",
						"2::2%eth1", "iface",
					),
					ppfmt.EXPECT().Noticef(
						pp.EmojiError,
						"Failed to find any global unicast %s address among unicast addresses assigned to interface %s",
						"IPv6", "iface",
					),
				)
			},
		},
		"ipaddr/4/loopback": {
			ipnet.IP4, 32,
			[]net.Addr{&net.IPAddr{IP: net.ParseIP("127.0.0.1"), Zone: ""}},
			protocol.NewUnavailableDetectionResult(),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find any global unicast %s address among unicast addresses assigned to interface %s", "IPv4", "iface")
			},
		},
		"ipaddr/4/255.255.255.255": {
			ipnet.IP4, 32,
			[]net.Addr{&net.IPAddr{IP: net.ParseIP("255.255.255.255"), Zone: ""}},
			protocol.NewUnavailableDetectionResult(),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiError,
					"Failed to find any global unicast %s address among unicast addresses assigned to interface %s",
					"IPv4", "iface",
				)
			},
		},
		"ipaddr/4/239.1.1.1": {
			ipnet.IP4, 32,
			[]net.Addr{&net.IPAddr{IP: net.ParseIP("239.1.1.1"), Zone: ""}},
			protocol.NewUnavailableDetectionResult(),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiImpossible,
					"Found multicast address %s in net.Interface.Addrs for interface %s (expected unicast addresses only); please report this at %s",
					"239.1.1.1", "iface", pp.IssueReportingURL,
				)
			},
		},
		"ipaddr/6/ff05::2": {
			ipnet.IP6, 64,
			[]net.Addr{&net.IPAddr{IP: net.ParseIP("ff05::2"), Zone: "site"}},
			protocol.NewUnavailableDetectionResult(),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiImpossible,
					"Found multicast address %s in net.Interface.Addrs for interface %s (expected unicast addresses only); please report this at %s",
					"ff05::2%site", "iface", pp.IssueReportingURL,
				)
			},
		},
		"ipaddr/4/dummy": {
			ipnet.IP4, 32,
			[]net.Addr{&Dummy{}},
			protocol.NewUnavailableDetectionResult(),
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Unexpected address data %q of type %T found in interface %s", "dummy/string", &Dummy{}, "iface")
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

			output := protocol.SelectAndNormalizeInterfaceIPs(mockPP, "iface", tc.ipFamily, tc.defaultPrefixLen, tc.input)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestLocalWithInterfaceIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.False(t, protocol.LocalWithInterface{
		ProviderName:  "",
		InterfaceName: "lo",
	}.IsExplicitEmpty())
}

func TestLocalWithInterfaceGetRawData(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		interfaceName string
		ipFamily      ipnet.Family
		ok            bool
		expected      []netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"lo/4": {
			"lo", ipnet.IP4, false,
			nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find any global unicast %s address among unicast addresses assigned to interface %s", "IPv4", "lo")
			},
		},
		"lo/6": {
			"lo", ipnet.IP6, false,
			nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find any global unicast %s address among unicast addresses assigned to interface %s", "IPv6", "lo")
			},
		},
		"non-existent": {
			"non-existent-iface", ipnet.IP4, false,
			nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiUserError, "Failed to find an interface named %q: %v", "non-existent-iface", gomock.Any())
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
			rawData := provider.GetRawData(context.Background(), mockPP, tc.ipFamily, map[ipnet.Family]int{
				ipnet.IP4: 32,
				ipnet.IP6: 64,
			}[tc.ipFamily])
			require.Equal(t, tc.ok, rawData.Available)
			want := liftedRawEntries(tc.ipFamily, tc.expected)
			if len(want) == 0 {
				want = nil
			}
			require.Equal(t, want, rawData.RawEntries)
		})
	}
}
