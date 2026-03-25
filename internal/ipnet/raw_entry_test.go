// vim: nowrap

package ipnet_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestParseRawEntry(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input            string
		defaultPrefixLen int
		ok               bool
		expected         string // CIDR notation of the resulting RawEntry
	}{
		"bare-ipv4":        {"1.2.3.4", 32, true, "1.2.3.4/32"},
		"bare-ipv6":        {"2001:db8::1", 64, true, "2001:db8::1/64"},
		"bare-ipv4-custom": {"10.0.0.1", 24, true, "10.0.0.1/24"},
		"cidr-ipv4":        {"10.0.0.5/24", 32, true, "10.0.0.5/24"},
		"cidr-ipv6":        {"2001:db8:1::1/48", 64, true, "2001:db8:1::1/48"},
		"cidr-ipv4-host":   {"192.168.1.1/32", 32, true, "192.168.1.1/32"},
		"cidr-ipv6-host":   {"::1/128", 64, true, "::1/128"},
		"zoned":            {"fe80::1%eth0", 64, false, ""},
		"invalid":          {"not-an-ip", 32, false, ""},
		"empty":            {"", 32, false, ""},
		"cidr-bad-prefix":  {"1.2.3.4/abc", 32, false, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry, err := ipnet.ParseRawEntry(tc.input, tc.defaultPrefixLen)
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expected, entry.String())
			} else {
				require.Error(t, err)
			}
		})
	}
}
