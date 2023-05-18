package ipnet_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic("mustURL")
	}
	return u
}

func TestResolveHostname(t *testing.T) {
	t.Parallel()

	check := func(ipNet ipnet.Type, hostname string, ok bool) {
		t.Helper()

		ip, err := ipnet.ResolveHostname(context.Background(), ipNet, hostname)
		if ok {
			require.Nil(t, err)
			require.True(t, ipNet.CheckIPFormat(ip))
		} else {
			require.NotNil(t, err)
			require.Zero(t, ip)
		}
	}

	// The test cases are designed to work around the security hardening
	// of GitHub Actions: an address cannot be resolved into two different
	// IP addresses. An IPv4 address and an IPv6 one count as two addresses.
	for name, tc := range map[string]struct {
		hostname string
		ipNet    ipnet.Type
		ok       bool
	}{
		"one.one.one.one/4":                  {"one.one.one.one", ipnet.IP4, true},
		"1dot1dot1dot1.cloudflare-dns.com/6": {"1dot1dot1dot1.cloudflare-dns.com", ipnet.IP6, true},
		"1.1.1.1/4":                          {"1.1.1.1", ipnet.IP4, true},
		"1.1.1.1/6":                          {"1.1.1.1", ipnet.IP6, false},
		"::1/4":                              {"::1", ipnet.IP4, false},
		"::1/6":                              {"::1", ipnet.IP6, true},
		"..../4":                             {"....", ipnet.IP4, false},
		"..../6":                             {"....", ipnet.IP6, false},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			check(tc.ipNet, tc.hostname, tc.ok)
		})
	}
}

func TestForceResolveURLHost(t *testing.T) {
	t.Parallel()

	for url, tc := range map[string]map[ipnet.Type][]string{
		"https://one.one.one.one": {
			ipnet.IP4: {"https://1.1.1.1", "https://1.0.0.1"},
		},
		"https://1dot1dot1dot1.cloudflare-dns.com": {
			ipnet.IP6: {"https://[2606:4700:4700::1111]", "https://[2606:4700:4700::1001]"},
		},
	} {
		url, tc := url, tc
		t.Run(url, func(t *testing.T) {
			t.Parallel()

			for ipNet, urls := range tc {
				u := mustURL(url)
				ok := ipnet.ForceResolveURLHost(context.Background(), ipNet, u)
				require.True(t, ok)
				require.Contains(t, urls, u.String())
			}
		})
	}
}
