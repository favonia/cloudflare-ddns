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

func TestParseRejectsIPv6BareHostWithHint(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP6, "addr-in(2001:db8::1)")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(2001:db8::1)") uses bare IP address "2001:db8::1"; use "2001:db8::1/128"`)
}

func TestParseRejectsWrongFamily(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in(2001:db8::/32)")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(2001:db8::/32)") contains IPv6 prefix "2001:db8::/32" in an IPv4 filter`)
}

func TestParseRejectsIPv4PrefixInIPv6Filter(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP6, "addr-in(198.51.100.0/24)")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(198.51.100.0/24)") contains IPv4 prefix "198.51.100.0/24" in an IPv6 filter`)
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

func TestParseRejectsMalformedCIDR(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in(not-a-prefix)")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(not-a-prefix)") is malformed: failed to parse "not-a-prefix" as a CIDR prefix`)
}

func TestParseRejectsUnrecognizedSymbol(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "keep-all & keep-all")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("keep-all & keep-all") is malformed: unrecognized symbol starting with '&'`)
}

func TestParseRejectsTokenWhereOpenParenExpected(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in 198.51.100.0/24")
	require.False(t, ok)
	require.Contains(t, output, `has unexpected token`)
	require.Contains(t, output, `when "(" is expected`)
}

func TestParseRejectsMissingClosingParen(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in(198.51.100.0/24")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(198.51.100.0/24") is missing ")" at the end`)
}

func TestParseRejectsBareTopLevelPrefix(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "198.51.100.0/24")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("198.51.100.0/24") is not a detection filter expression`)
}

func TestParseRejectsNonPrefixAddrInArgument(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in(keep-all)")
	require.False(t, ok)
	require.Contains(t, output, `TEST_FILTER ("addr-in(keep-all)") is not a detection filter expression`)
}

func TestParseSurfacesErrorInsideNegation(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "!addr-in(2001:db8::/32)")
	require.False(t, ok)
	require.Contains(t, output, `contains IPv6 prefix "2001:db8::/32" in an IPv4 filter`)
}

func TestParseSurfacesErrorInLeftOperand(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "addr-in(2001:db8::/32) && keep-all")
	require.False(t, ok)
	require.Contains(t, output, `contains IPv6 prefix "2001:db8::/32" in an IPv4 filter`)
}

func TestParseSurfacesErrorInRightOperand(t *testing.T) {
	t.Parallel()

	_, ok, output := parseWithOutput(ipnet.IP4, "keep-all && addr-in(2001:db8::/32)")
	require.False(t, ok)
	require.Contains(t, output, `contains IPv6 prefix "2001:db8::/32" in an IPv4 filter`)
}
