package ipnet_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestParseCloudflareRanges(t *testing.T) {
	t.Parallel()

	// Verify that the embedded text files parse correctly.
	// This catches broken text files at test time.
	require.NotPanics(t, func() {
		// Trigger lazy parsing by calling IsCloudflareIP with any address.
		ipnet.IsCloudflareIP(netip.MustParseAddr("127.0.0.1"))
	})
}

func TestCloudflareIPRejecterRejectRawIP(t *testing.T) {
	t.Parallel()

	r := ipnet.CloudflareIPRejecter{}

	for name, tc := range map[string]struct {
		ip     netip.Addr
		ok     bool
		hasMsg bool
	}{
		"cloudflare-ip-rejected":       {netip.MustParseAddr("104.16.0.1"), false, true},
		"non-cloudflare-ip-accepted":   {netip.MustParseAddr("1.2.3.4"), true, false},
		"cloudflare-ipv6-rejected":     {netip.MustParseAddr("2606:4700::1"), false, true},
		"non-cloudflare-ipv6-accepted": {netip.MustParseAddr("2001:db8::1"), true, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ok, msg := r.RejectRawIP(tc.ip)
			require.Equal(t, tc.ok, ok)
			if tc.hasMsg {
				require.Contains(t, msg, tc.ip.String())
				require.Contains(t, msg, "Cloudflare")
			} else {
				require.Empty(t, msg)
			}
		})
	}
}

func TestIsCloudflareIP(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ip       netip.Addr
		expected bool
	}{
		// Known Cloudflare IPv4 ranges
		"cloudflare-104.16.0.0":     {netip.MustParseAddr("104.16.0.0"), true},
		"cloudflare-104.16.0.1":     {netip.MustParseAddr("104.16.0.1"), true},
		"cloudflare-104.23.255.255": {netip.MustParseAddr("104.23.255.255"), true},
		"cloudflare-173.245.48.1":   {netip.MustParseAddr("173.245.48.1"), true},
		"cloudflare-162.158.0.1":    {netip.MustParseAddr("162.158.0.1"), true},
		"cloudflare-131.0.72.1":     {netip.MustParseAddr("131.0.72.1"), true},

		// Known Cloudflare IPv6 ranges
		"cloudflare-2606:4700::1": {netip.MustParseAddr("2606:4700::1"), true},
		"cloudflare-2400:cb00::1": {netip.MustParseAddr("2400:cb00::1"), true},
		"cloudflare-2a06:98c0::1": {netip.MustParseAddr("2a06:98c0::1"), true},

		// 4-in-6 mapped Cloudflare addresses
		"cloudflare-4in6-104.16.0.1": {netip.MustParseAddr("::ffff:104.16.0.1"), true},

		// Non-Cloudflare addresses
		"non-cf-1.2.3.4":     {netip.MustParseAddr("1.2.3.4"), false},
		"non-cf-8.8.8.8":     {netip.MustParseAddr("8.8.8.8"), false},
		"non-cf-192.168.1.1": {netip.MustParseAddr("192.168.1.1"), false},
		"non-cf-10.0.0.1":    {netip.MustParseAddr("10.0.0.1"), false},
		"non-cf-2001:db8::1": {netip.MustParseAddr("2001:db8::1"), false},
		"non-cf-fe80::1":     {netip.MustParseAddr("fe80::1"), false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, ipnet.IsCloudflareIP(tc.ip))
		})
	}
}
