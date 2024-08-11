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

func TestDescribe(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Type
		expected string
	}{
		"4":   {ipnet.IP4, "IPv4"},
		"6":   {ipnet.IP6, "IPv6"},
		"100": {ipnet.Type(100), "<unrecognized IP version 100>"},
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
				m.EXPECT().Warningf(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", "1::2")
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
				m.EXPECT().Warningf(pp.EmojiImpossible,
					"Unrecognized IP version %d was used; please report this at %s", 100, pp.IssueReportingURL)
			},
		},
		"4-0.0.0.0": {
			ipnet.IP4, mustIP("0.0.0.0"),
			netip.Addr{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiImpossible,
					"Detected IP address %s is an unspecified %s address", "0.0.0.0", "IPv4")
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
			ip, ok := tc.ipNet.NormalizeDetectedIP(mockPP, tc.ip)
			require.Equal(t, tc.expected, ip)
			require.Equal(t, tc.ok, ok)
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
