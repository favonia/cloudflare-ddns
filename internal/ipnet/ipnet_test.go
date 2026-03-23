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

func mustRawEntry(s string) ipnet.RawEntry {
	return ipnet.RawEntry(netip.MustParsePrefix(s))
}

func TestInt(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Family
		expected int
	}{
		"4":   {ipnet.IP4, 4},
		"6":   {ipnet.IP6, 6},
		"100": {ipnet.Family(100), 0},
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
		input    ipnet.Family
		expected string
	}{
		"4":   {ipnet.IP4, "IPv4"},
		"6":   {ipnet.IP6, "IPv6"},
		"100": {ipnet.Family(100), ""},
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
		input    ipnet.Family
		expected string
	}{
		"4":   {ipnet.IP4, "A"},
		"6":   {ipnet.IP6, "AAAA"},
		"100": {ipnet.Family(100), ""},
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
		input    ipnet.Family
		expected string
	}{
		"4":   {ipnet.IP4, "udp4"},
		"6":   {ipnet.IP6, "udp6"},
		"100": {ipnet.Family(100), ""},
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
		ipFamily      ipnet.Family
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
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address; it can't be used", "1::2")
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
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv4", "0.0.0.0", "an unspecified address")
			},
		},
		"singleton/4-127.0.0.1": {
			ipnet.IP4, singleton(mustIP("127.0.0.1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv4", "127.0.0.1", "a loopback address")
			},
		},
		"singleton/4-169.254.1.1": {
			ipnet.IP4, singleton(mustIP("169.254.1.1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv4", "169.254.1.1", "a link-local address")
			},
		},
		"singleton/4-224.0.0.1": {
			ipnet.IP4, singleton(mustIP("224.0.0.1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv4", "224.0.0.1", "a link-local multicast address")
			},
		},
		"singleton/4-239.1.1.1": {
			ipnet.IP4, singleton(mustIP("239.1.1.1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv4", "239.1.1.1", "a multicast address")
			},
		},
		"singleton/4-255.255.255.255": {
			ipnet.IP4, singleton(mustIP("255.255.255.255")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv4", "255.255.255.255", "a broadcast address")
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
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv6", "1::2%eth0", "an address with a zone identifier")
			},
		},
		"singleton/6-10.10.10.10": {
			ipnet.IP6, singleton(mustIP("10.10.10.10")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv6 address; it can't be used", "10.10.10.10")
			},
		},
		"singleton/6-::ffff:10.10.10.10": {
			ipnet.IP6, singleton(mustIP("::ffff:10.10.10.10")),
			false, nil,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is an IPv4-mapped IPv6 address; it can't be used", "::ffff:10.10.10.10"),
					m.EXPECT().InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint, "An IPv4-mapped IPv6 address is an IPv4 address in disguise. It cannot be used for routing IPv6 traffic. If you need to use it for DNS, please open an issue at %s", pp.IssueReportingURL),
				)
			},
		},
		"singleton/6-::1": {
			ipnet.IP6, singleton(mustIP("::1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv6", "::1", "a loopback address")
			},
		},
		"singleton/6-ff01::1": {
			ipnet.IP6, singleton(mustIP("ff01::1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv6", "ff01::1", "a multicast address")
			},
		},
		"singleton/6-ff02::1": {
			ipnet.IP6, singleton(mustIP("ff02::1")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv6", "ff02::1", "a link-local multicast address")
			},
		},
		"singleton/6-ff05::2": {
			ipnet.IP6, singleton(mustIP("ff05::2")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is %s", "IPv6", "ff05::2", "a multicast address")
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

			ips, ok := tc.ipFamily.NormalizeDetectedIPs(mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, ips)
		})
	}
}

func TestRawEntryFrom(t *testing.T) {
	t.Parallel()

	entry := ipnet.RawEntryFrom(mustIP("192.168.1.5"), 24)
	require.Equal(t, mustIP("192.168.1.5"), entry.Addr())
	require.Equal(t, 24, entry.PrefixLen())
	require.True(t, entry.IsValid())
	require.Equal(t, "192.168.1.5/24", entry.String())
	require.Equal(t, netip.MustParsePrefix("192.168.1.0/24"), entry.Masked())
	require.Equal(t, netip.MustParsePrefix("192.168.1.5/24"), entry.Prefix())
}

func TestRawEntryZeroValue(t *testing.T) {
	t.Parallel()

	var entry ipnet.RawEntry
	require.False(t, entry.IsValid())
	require.Equal(t, "invalid Prefix", entry.String())
}

func TestRawEntryMasked(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    string
		expected string
	}{
		"4-host-bits":    {"192.168.1.100/24", "192.168.1.0/24"},
		"4-full":         {"10.0.0.1/32", "10.0.0.1/32"},
		"6-host-bits":    {"2001:db8::cafe/48", "2001:db8::/48"},
		"6-full":         {"2001:db8::1/128", "2001:db8::1/128"},
		"4-no-mask":      {"192.168.1.100/0", "0.0.0.0/0"},
		"6-no-mask":      {"2001:db8::1/0", "::/0"},
		"4-single-bit":   {"128.0.0.1/1", "128.0.0.0/1"},
		"6-single-bit":   {"8000::1/1", "8000::/1"},
		"4-partial-byte": {"192.168.255.255/20", "192.168.240.0/20"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry := mustRawEntry(tc.input)
			require.Equal(t, netip.MustParsePrefix(tc.expected), entry.Masked())
		})
	}
}

func TestRawEntryPrefix(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
	}{
		"4-with-host-bits": {"192.168.1.100/24"},
		"6-with-host-bits": {"2001:db8::cafe/48"},
		"4-full":           {"10.0.0.1/32"},
		"6-full":           {"2001:db8::1/128"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry := mustRawEntry(tc.input)
			// Prefix preserves host bits (unlike Masked)
			require.Equal(t, netip.MustParsePrefix(tc.input), entry.Prefix())
		})
	}
}

func TestRawEntryCompare(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		a, b     string
		expected int
	}{
		"equal":              {"10.0.0.1/24", "10.0.0.1/24", 0},
		"less-by-addr":       {"10.0.0.1/24", "10.0.0.2/24", -1},
		"greater-by-addr":    {"10.0.0.2/24", "10.0.0.1/24", 1},
		"less-by-prefix":     {"10.0.0.1/24", "10.0.0.1/32", -1},
		"greater-by-prefix":  {"10.0.0.1/32", "10.0.0.1/24", 1},
		"4-before-6":         {"10.0.0.1/32", "2001:db8::1/128", -1},
		"6-after-4":          {"2001:db8::1/128", "10.0.0.1/32", 1},
		"same-addr-diff-len": {"192.168.1.0/24", "192.168.1.0/16", 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := mustRawEntry(tc.a)
			b := mustRawEntry(tc.b)
			require.Equal(t, tc.expected, a.Compare(b))
		})
	}
}

func TestLiftValidatedIPsToRawEntries(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ips       []netip.Addr
		prefixLen int
		expected  []ipnet.RawEntry
	}{
		"nil": {
			nil, 24, nil,
		},
		"empty": {
			[]netip.Addr{}, 24, nil,
		},
		"single-4": {
			[]netip.Addr{mustIP("10.0.0.1")},
			24,
			[]ipnet.RawEntry{mustRawEntry("10.0.0.1/24")},
		},
		"single-6": {
			[]netip.Addr{mustIP("2001:db8::1")},
			48,
			[]ipnet.RawEntry{mustRawEntry("2001:db8::1/48")},
		},
		"multiple": {
			[]netip.Addr{mustIP("10.0.0.1"), mustIP("10.0.0.2"), mustIP("10.0.0.3")},
			32,
			[]ipnet.RawEntry{mustRawEntry("10.0.0.1/32"), mustRawEntry("10.0.0.2/32"), mustRawEntry("10.0.0.3/32")},
		},
		"full-prefix-4": {
			[]netip.Addr{mustIP("192.168.1.1")},
			32,
			[]ipnet.RawEntry{mustRawEntry("192.168.1.1/32")},
		},
		"zero-prefix": {
			[]netip.Addr{mustIP("10.0.0.1")},
			0,
			[]ipnet.RawEntry{mustRawEntry("10.0.0.1/0")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result := ipnet.LiftValidatedIPsToRawEntries(tc.ips, tc.prefixLen)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeDetectedRawEntries(t *testing.T) {
	t.Parallel()

	var invalidEntry ipnet.RawEntry
	singleton := func(entry ipnet.RawEntry) []ipnet.RawEntry { return []ipnet.RawEntry{entry} }

	for name, tc := range map[string]struct {
		ipFamily      ipnet.Family
		input         []ipnet.RawEntry
		ok            bool
		expected      []ipnet.RawEntry
		prepareMockPP func(*mocks.MockPP)
	}{
		"4-empty-nil": {
			ipnet.IP4, nil,
			true, nil,
			nil,
		},
		"4-empty-list": {
			ipnet.IP4,
			[]ipnet.RawEntry{},
			true,
			[]ipnet.RawEntry{},
			nil,
		},
		"singleton/4-invalid": {
			ipnet.IP4, singleton(invalidEntry),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Detected address is not valid; this should not happen and please report it at %s`, pp.IssueReportingURL)
			},
		},
		"singleton/4-native": {
			ipnet.IP4, singleton(mustRawEntry("10.0.0.1/32")),
			true, singleton(mustRawEntry("10.0.0.1/32")),
			nil,
		},
		"singleton/4-mapped-128": {
			ipnet.IP4, singleton(mustRawEntry("::ffff:10.10.10.10/128")),
			true, singleton(mustRawEntry("10.10.10.10/32")),
			nil,
		},
		"singleton/4-mapped-120": {
			ipnet.IP4, singleton(mustRawEntry("::ffff:10.10.10.10/120")),
			true, singleton(mustRawEntry("10.10.10.10/24")),
			nil,
		},
		"singleton/4-mapped-short": {
			ipnet.IP4, singleton(mustRawEntry("::ffff:10.10.10.10/80")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"Detected address %s is an IPv4-mapped IPv6 address with a prefix length shorter than /96 and cannot be used",
					"::ffff:10.10.10.10/80",
				)
			},
		},
		"singleton/4-6-prefix": {
			ipnet.IP4, singleton(mustRawEntry("2001:db8::1/64")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"Detected address %s is not a valid IPv4 address and cannot be used",
					"2001:db8::1/64",
				)
			},
		},
		"singleton/4-broadcast-rejected": {
			ipnet.IP4, singleton(mustRawEntry("255.255.255.255/32")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"Detected %s address %s is %s",
					"IPv4", "255.255.255.255", "a broadcast address",
				)
			},
		},
		"singleton/6-native": {
			ipnet.IP6, singleton(mustRawEntry("2001:db8::1/64")),
			true, singleton(mustRawEntry("2001:db8::1/64")),
			nil,
		},
		"singleton/6-mapped": {
			ipnet.IP6, singleton(mustRawEntry("::ffff:10.10.10.10/128")),
			false, nil,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiError,
						"Detected address %s is an IPv4-mapped IPv6 address and cannot be used",
						"::ffff:10.10.10.10/128",
					),
					m.EXPECT().InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
						"An IPv4-mapped IPv6 address is an IPv4 address in disguise. It cannot be used for routing IPv6 traffic. If you need to use it for DNS, please open an issue at %s",
						pp.IssueReportingURL,
					),
				)
			},
		},
		"singleton/4-mapped-96": {
			ipnet.IP4, singleton(mustRawEntry("::ffff:10.10.10.10/96")),
			true, singleton(mustRawEntry("10.10.10.10/0")),
			nil,
		},
		"singleton/4-native-with-prefix": {
			ipnet.IP4, singleton(mustRawEntry("10.0.0.1/24")),
			true, singleton(mustRawEntry("10.0.0.1/24")),
			nil,
		},
		"singleton/4-loopback-rejected": {
			ipnet.IP4, singleton(mustRawEntry("127.0.0.1/32")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"Detected %s address %s is %s",
					"IPv4", "127.0.0.1", "a loopback address",
				)
			},
		},
		"singleton/6-ipv4-not-is6": {
			ipnet.IP6, singleton(mustRawEntry("10.10.10.10/32")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"Detected address %s is not a valid IPv6 address and cannot be used",
					"10.10.10.10/32",
				)
			},
		},
		"singleton/6-loopback-rejected": {
			ipnet.IP6, singleton(mustRawEntry("::1/128")),
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"Detected %s address %s is %s",
					"IPv6", "::1", "a loopback address",
				)
			},
		},
		"singleton/100-default-family": {
			100, singleton(mustRawEntry("10.0.0.1/32")),
			false, nil,
			nil,
		},
		"list/4-fail-fast": {
			ipnet.IP4,
			[]ipnet.RawEntry{
				invalidEntry,
				mustRawEntry("10.0.0.1/32"),
			},
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiImpossible,
					`Detected address is not valid; this should not happen and please report it at %s`,
					pp.IssueReportingURL,
				)
			},
		},
		"sort-dedup-4-mapped": {
			ipnet.IP4,
			[]ipnet.RawEntry{
				mustRawEntry("10.0.0.2/32"),
				mustRawEntry("::ffff:10.0.0.1/128"),
				mustRawEntry("10.0.0.2/32"),
			},
			true,
			[]ipnet.RawEntry{
				mustRawEntry("10.0.0.1/32"),
				mustRawEntry("10.0.0.2/32"),
			},
			nil,
		},
		"sort-dedup-6": {
			ipnet.IP6,
			[]ipnet.RawEntry{
				mustRawEntry("2001:db8::2/64"),
				mustRawEntry("2001:db8::1/64"),
				mustRawEntry("2001:db8::2/64"),
			},
			true,
			[]ipnet.RawEntry{
				mustRawEntry("2001:db8::1/64"),
				mustRawEntry("2001:db8::2/64"),
			},
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			entries, ok := tc.ipFamily.NormalizeDetectedRawEntries(mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, entries)
		})
	}
}

func TestMatches(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ipFamily ipnet.Family
		ip       netip.Addr
		expected bool
	}{
		"4/yes": {ipnet.IP4, netip.IPv4Unspecified(), true},
		"4/no":  {ipnet.IP4, netip.IPv6Unspecified(), false},
		"6/yes": {ipnet.IP6, netip.IPv6Unspecified(), true},
		"6/no":  {ipnet.IP6, netip.IPv4Unspecified(), false},
		"100":   {ipnet.Family(100), netip.Addr{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.ipFamily.Matches(tc.ip))
		})
	}
}

func TestDescribeAddressIssue(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ip          netip.Addr
		description string
		bad         bool
	}{
		"unspecified/4":        {mustIP("0.0.0.0"), "an unspecified address", true},
		"unspecified/6":        {mustIP("::"), "an unspecified address", true},
		"loopback/4":           {mustIP("127.0.0.1"), "a loopback address", true},
		"loopback/6":           {mustIP("::1"), "a loopback address", true},
		"link-local-multicast": {mustIP("ff02::1"), "a link-local multicast address", true},
		"multicast/4":          {mustIP("239.1.1.1"), "a multicast address", true},
		"multicast/6":          {mustIP("ff05::2"), "a multicast address", true},
		"link-local/4":         {mustIP("169.254.1.1"), "a link-local address", true},
		"link-local/6":         {mustIP("fe80::1"), "a link-local address", true},
		"zone":                 {mustIP("1::2%eth0"), "an address with a zone identifier", true},
		"global-unicast/4":     {mustIP("1.1.1.1"), "", false},
		"global-unicast/6":     {mustIP("2001:db8::1"), "", false},
		"broadcast":            {mustIP("255.255.255.255"), "a broadcast address", true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			desc, bad := ipnet.DescribeAddressIssue(tc.ip)
			require.Equal(t, tc.bad, bad)
			require.Equal(t, tc.description, desc)
		})
	}
}

func TestBindings(t *testing.T) {
	t.Parallel()

	count := 0
	for ipFamily := range ipnet.Bindings(map[ipnet.Family]int{
		ipnet.IP4: 400,
		ipnet.IP6: 600,
	}) {
		count++
		require.Equal(t, ipnet.IP4, ipFamily)
		break
	}
	require.Equal(t, 1, count)
}
