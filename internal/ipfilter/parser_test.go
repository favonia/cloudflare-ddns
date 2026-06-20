package ipfilter_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipfilter"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func parseWithOutput(family ipnet.Family, text string) (ipfilter.Filter, bool, string) {
	var output strings.Builder
	filter, ok := ipfilter.Parse(pp.New(&output, false, pp.Quiet), "TEST_FILTER", family, text)
	return filter, ok, output.String()
}

func TestParseValidExpressions(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"keep-all",
		"addr-in(198.51.100.0/24)",
		"!addr-in(198.51.100.0/24)",
		"addr-in(198.51.100.0/24)||addr-in(203.0.113.0/24)",
		"!(addr-in(10.0.0.0/8) || addr-in(192.168.0.0/16))",
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			filter, ok, output := parseWithOutput(ipnet.IP4, text)
			require.True(t, ok)
			require.NotEmpty(t, filter.String())
			require.Empty(t, output)
		})
	}
}

func TestParseRejectsBareHostWithHint(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in(8.8.8.8)")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(8.8.8.8)") uses bare IP address "8.8.8.8"; use "8.8.8.8/32"`)
}

func TestParseRejectsWrongFamily(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in(2001:db8::/32)")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(2001:db8::/32)") contains IPv6 prefix "2001:db8::/32" in an IPv4 filter`)
}

func TestParseRejectsUnexpectedToken(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in(198.51.100.0/24) addr-in(203.0.113.0/24)")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(198.51.100.0/24) addr-in(203.0.113.0/24)") is not a detection filter expression`)
}

func TestParseRejectsIncompleteExpression(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP6, "addr-in(fc00::/7) &&")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(fc00::/7) &&") is not a detection filter expression`)
}
