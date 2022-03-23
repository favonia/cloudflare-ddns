package ipnet_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
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

func TestNormalizeIP(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ipNet    ipnet.Type
		ip       netip.Addr
		expected netip.Addr
		ok       bool
	}{
		"4-1::2":             {ipnet.IP4, mustIP("1::2"), mustIP("1::2"), false},
		"4-::ffff:0a0a:0a0a": {ipnet.IP4, mustIP("::ffff:0a0a:0a0a"), mustIP("10.10.10.10"), true},
		"6-1::2":             {ipnet.IP6, mustIP("1::2"), mustIP("1::2"), true},
		"6-10.10.10.10":      {ipnet.IP6, mustIP("10.10.10.10"), mustIP("::ffff:10.10.10.10"), true},
		"100-nil":            {100, netip.Addr{}, netip.Addr{}, false},
		"100-10.10.10.10":    {100, mustIP("10.10.10.10"), mustIP("10.10.10.10"), true},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ip, ok := tc.ipNet.NormalizeIP(tc.ip)
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.UDPNetwork())
		})
	}
}
