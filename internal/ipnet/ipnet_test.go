package ipnet_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestString(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.Type
		expected string
	}{
		"4":   {ipnet.IP4, "IPv4"},
		"6":   {ipnet.IP6, "IPv6"},
		"100": {ipnet.Type(100), "(unrecognized IP network)"},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.String())
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
		ip       net.IP
		expected net.IP
	}{
		"4-1::2":             {ipnet.IP4, net.ParseIP("1::2").To16(), nil},
		"4-::ffff:0a0a:0a0a": {ipnet.IP4, net.ParseIP("::ffff:0a0a:0a0a").To16(), net.ParseIP("10.10.10.10").To4()},
		"6-1::2":             {ipnet.IP6, net.ParseIP("1::2").To16(), net.ParseIP("1::2").To16()},
		"6-10.10.10.10":      {ipnet.IP6, net.ParseIP("10.10.10.10").To4(), net.ParseIP("10.10.10.10").To16()},
		"100-nil":            {100, nil, nil},
		"100-10.10.10.10":    {100, net.ParseIP("10.10.10.10").To4(), net.ParseIP("10.10.10.10").To4()},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.ipNet.NormalizeIP(tc.ip))
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
