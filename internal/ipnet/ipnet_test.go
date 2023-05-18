package ipnet_test

import (
	"net/netip"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func mustIP(ip string) netip.Addr {
	return netip.MustParseAddr(ip)
}

func TestDescribe(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Type
		expected string
	}{
		"4":   {ipnet.IP4, "IPv4"},
		"6":   {ipnet.IP6, "IPv6"},
		"100": {ipnet.Type(100), "<unrecognized IP network>"},
	} {
		tc := tc
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.RecordType())
		})
	}
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.Int())
		})
	}
}

//nolint:funlen
func TestNormalizeDetectedIP(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ipNet         ipnet.Type
		ip            netip.Addr
		expected      netip.Addr
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"4-1::2": {
			ipnet.IP4, mustIP("1::2"),
			netip.Addr{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiError, "Detected IP address %s is not a valid %s address", "1::2", "IPv4")
			},
		},
		"4-::ffff:0a0a:0a0a": {ipnet.IP4, mustIP("::ffff:0a0a:0a0a"), mustIP("10.10.10.10"), true, nil},
		"6-1::2":             {ipnet.IP6, mustIP("1::2"), mustIP("1::2"), true, nil},
		"6-10.10.10.10":      {ipnet.IP6, mustIP("10.10.10.10"), mustIP("::ffff:10.10.10.10"), true, nil},
		"6-invalid": {
			ipnet.IP6,
			netip.Addr{},
			netip.Addr{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiImpossible, "Detected IP address is not valid")
			},
		},
		"100-10.10.10.10": {
			100, mustIP("10.10.10.10"),
			netip.Addr{},
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiImpossible, "Detected IP address %s is not a valid %s address", "10.10.10.10", "<unrecognized IP network>"), //nolint:lll
					m.EXPECT().Warningf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new"),               //nolint:lll
				)
			},
		},
		"4-0.0.0.0": {
			ipnet.IP4, mustIP("0.0.0.0"),
			netip.Addr{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiImpossible, "Detected IP address %s is an unspicifed %s address", "0.0.0.0", "IPv4") //nolint:lll
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ip, ok := tc.ipNet.NormalizeDetectedIP(mockPP, tc.ip)
			require.Equal(t, tc.expected, ip)
			require.Equal(t, tc.ok, ok)
		})
	}
}

func TestCheckIPFormat(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ip  netip.Addr
		ok4 bool
		ok6 bool
	}{
		"1::2":    {mustIP("1::2"), false, true},
		"1.1.1.1": {mustIP("1.1.1.1"), true, false},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.ok4, ipnet.IP4.CheckIPFormat(tc.ip))
			require.Equal(t, tc.ok6, ipnet.IP6.CheckIPFormat(tc.ip))
			require.False(t, ipnet.Type(0).CheckIPFormat(tc.ip))
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.UDPNetwork())
		})
	}
}

func TestIPNetwork(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Type
		expected string
	}{
		"4":   {ipnet.IP4, "ip4"},
		"6":   {ipnet.IP6, "ip6"},
		"100": {ipnet.Type(100), ""},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.IPNetwork())
		})
	}
}
