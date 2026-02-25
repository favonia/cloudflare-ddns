// vim: nowrap
package ipnet_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func mustIP(ip string) netip.Addr {
	return netip.MustParseAddr(ip)
}

func TestInt(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Type
		expected int
	}{
		"4":   {ipnet.IP4, 4},
		"6":   {ipnet.IP6, 6},
		"100": {ipnet.Type(100), 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.Int())
		})
	}
}

func TestDescribe(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Type
		expected string
	}{
		"4":   {ipnet.IP4, "IPv4"},
		"6":   {ipnet.IP6, "IPv6"},
		"100": {ipnet.Type(100), ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.Describe())
		})
	}
}

func TestRecordType(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Type
		expected string
	}{
		"4":   {ipnet.IP4, "A"},
		"6":   {ipnet.IP6, "AAAA"},
		"100": {ipnet.Type(100), ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.RecordType())
		})
	}
}

func TestUDPNetwork(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Type
		expected string
	}{
		"4":   {ipnet.IP4, "udp4"},
		"6":   {ipnet.IP6, "udp6"},
		"100": {ipnet.Type(100), ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.UDPNetwork())
		})
	}
}

func TestNormalizeDetectedIPs(t *testing.T) {
	t.Parallel()

	var invalidIP netip.Addr
	singleton := func(ip netip.Addr) []netip.Addr { return []netip.Addr{ip} }

	for name, tc := range map[string]struct {
		ipNet         ipnet.Type
		input         []netip.Addr
		ok            bool
		expected      []netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"4-empty-nil": {
			ipnet.IP4, nil,
			true, nil,
			nil,
		},
		"4-empty-list": {
			ipnet.IP4,
			[]netip.Addr{},
			true,
			[]netip.Addr{},
			nil,
		},
		"singleton/4-invalid": {
			ipnet.IP4, singleton(invalidIP),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Detected IP address is not valid; this should not happen and please report it at %s`, pp.IssueReportingURL)
			},
		},
		"singleton/4-1::2": {
			ipnet.IP4, singleton(mustIP("1::2")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", "1::2")
			},
		},
		"singleton/4-::ffff:0a0a:0a0a": {
			ipnet.IP4, singleton(mustIP("::ffff:0a0a:0a0a")),
			true, singleton(mustIP("10.10.10.10")),
			nil,
		},
		"singleton/4-0.0.0.0": {
			ipnet.IP4, singleton(mustIP("0.0.0.0")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is an unspecified address", "IPv4", "0.0.0.0")
			},
		},
		"singleton/4-127.0.0.1": {
			ipnet.IP4, singleton(mustIP("127.0.0.1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a loopback address", "IPv4", "127.0.0.1")
			},
		},
		"singleton/4-169.254.1.1": {
			ipnet.IP4, singleton(mustIP("169.254.1.1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a link-local address", "IPv4", "169.254.1.1")
			},
		},
		"singleton/4-224.0.0.1": {
			ipnet.IP4, singleton(mustIP("224.0.0.1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a link-local multicast address", "IPv4", "224.0.0.1")
			},
		},
		"singleton/4-239.1.1.1": {
			ipnet.IP4, singleton(mustIP("239.1.1.1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a multicast address", "IPv4", "239.1.1.1")
			},
		},
		"singleton/4-255.255.255.255": {
			ipnet.IP4, singleton(mustIP("255.255.255.255")),
			true, singleton(mustIP("255.255.255.255")),
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiWarning, "Detected %s address %s does not look like a global unicast address", "IPv4", "255.255.255.255")
			},
		},
		"singleton/6-invalid": {
			ipnet.IP6, singleton(invalidIP),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Detected IP address is not valid; this should not happen and please report it at %s`, pp.IssueReportingURL)
			},
		},
		"singleton/6-1::2": {
			ipnet.IP6, singleton(mustIP("1::2")),
			true, singleton(mustIP("1::2")),
			nil,
		},
		"singleton/6-1::2%eth0": {
			ipnet.IP6, singleton(mustIP("1::2%eth0")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiError,
					"Detected %s address %s has a zone identifier and cannot be used as a target address",
					"IPv6", "1::2%eth0",
				)
			},
		},
		"singleton/6-10.10.10.10": {
			ipnet.IP6, singleton(mustIP("10.10.10.10")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv6 address", "10.10.10.10")
			},
		},
		"singleton/6-::ffff:10.10.10.10": {
			ipnet.IP6, singleton(mustIP("::ffff:10.10.10.10")),
			false, nil,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is an IPv4-mapped IPv6 address", "::ffff:10.10.10.10"),
					m.EXPECT().InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint, "An IPv4-mapped IPv6 address is an IPv4 address in disguise. It cannot be used for routing IPv6 traffic. If you need to use it for DNS, please open an issue at %s", pp.IssueReportingURL),
				)
			},
		},
		"singleton/6-::1": {
			ipnet.IP6, singleton(mustIP("::1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a loopback address", "IPv6", "::1")
			},
		},
		"singleton/6-ff01::1": {
			ipnet.IP6, singleton(mustIP("ff01::1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a multicast address", "IPv6", "ff01::1")
			},
		},
		"singleton/6-ff02::1": {
			ipnet.IP6, singleton(mustIP("ff02::1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a link-local multicast address", "IPv6", "ff02::1")
			},
		},
		"singleton/6-ff05::2": {
			ipnet.IP6, singleton(mustIP("ff05::2")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a multicast address", "IPv6", "ff05::2")
			},
		},
		"singleton/100-10.10.10.10": {
			100, singleton(mustIP("10.10.10.10")),
			false, nil,
			nil,
		},
		"4-sort-dedup-unmap": {
			ipnet.IP4,
			[]netip.Addr{
				mustIP("10.0.0.2"),
				mustIP("::ffff:10.0.0.1"),
				mustIP("10.0.0.2"),
			},
			true,
			[]netip.Addr{
				mustIP("10.0.0.1"),
				mustIP("10.0.0.2"),
			},
			nil,
		},
		"list/4-fail-fast": {
			ipnet.IP4,
			[]netip.Addr{
				invalidIP,
				mustIP("1::2"),
			},
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiImpossible,
					`Detected IP address is not valid; this should not happen and please report it at %s`,
					pp.IssueReportingURL,
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

			ips, ok := tc.ipNet.NormalizeDetectedIPs(mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, ips)
		})
	}
}

func TestMatches(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ipNet    ipnet.Type
		ip       netip.Addr
		expected bool
	}{
		"4/yes": {ipnet.IP4, netip.IPv4Unspecified(), true},
		"4/no":  {ipnet.IP4, netip.IPv6Unspecified(), false},
		"6/yes": {ipnet.IP6, netip.IPv6Unspecified(), true},
		"6/no":  {ipnet.IP6, netip.IPv4Unspecified(), false},
		"100":   {ipnet.Type(100), netip.Addr{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.ipNet.Matches(tc.ip))
		})
	}
}

func TestBindings(t *testing.T) {
	t.Parallel()

	count := 0
	for ipNet := range ipnet.Bindings(map[ipnet.Type]int{
		ipnet.IP4: 400,
		ipnet.IP6: 600,
	}) {
		count++
		require.Equal(t, ipnet.IP4, ipNet)
		break
	}
	require.Equal(t, 1, count)
}
