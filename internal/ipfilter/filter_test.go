package ipfilter_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipfilter"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func raw(text string) ipnet.RawEntry {
	prefix := netip.MustParsePrefix(text)
	return ipnet.RawEntryFrom(prefix.Addr(), prefix.Bits())
}

func mustParse(t *testing.T, family ipnet.Family, text string) ipfilter.Filter {
	t.Helper()
	filter, ok := ipfilter.Parse(pp.NewSilent(), "TEST_FILTER", family, text)
	require.True(t, ok)
	return filter
}

func TestKeepAll(t *testing.T) {
	t.Parallel()

	filter := ipfilter.KeepAll()
	require.Equal(t, "keep-all", filter.String())
	require.True(t, filter.Evaluate(raw("198.51.100.2/32")))
	require.True(t, filter.Evaluate(raw("2001:db8::1/64")))
}

func TestZeroValueKeepsAll(t *testing.T) {
	t.Parallel()

	var filter ipfilter.Filter
	input := []ipnet.RawEntry{raw("198.51.100.2/32")}
	require.Equal(t, "keep-all", filter.String())
	require.True(t, filter.IsDefault())
	require.True(t, filter.Evaluate(input[0]))
	require.Equal(t, input, filter.Apply(input))
}

func TestAddrInIgnoresDetectedPrefixLength(t *testing.T) {
	t.Parallel()

	filter := mustParse(t, ipnet.IP6, "addr-in(2001:db8::/32)")
	require.True(t, filter.Evaluate(raw("2001:db8:1::abcd/64")))
	require.True(t, filter.Evaluate(raw("2001:db8:2::abcd/128")))
	require.False(t, filter.Evaluate(raw("2001:db9::abcd/64")))
}

func TestBooleanOperators(t *testing.T) {
	t.Parallel()

	filter := mustParse(t, ipnet.IP4, "addr-in(198.51.100.0/24) && !addr-in(198.51.100.7/32)")
	require.True(t, filter.Evaluate(raw("198.51.100.8/32")))
	require.False(t, filter.Evaluate(raw("198.51.100.7/32")))
	require.False(t, filter.Evaluate(raw("203.0.113.7/32")))
}

func TestFilterPreservesOrder(t *testing.T) {
	t.Parallel()

	filter := mustParse(t, ipnet.IP4, "addr-in(198.51.100.0/24)")
	input := []ipnet.RawEntry{
		raw("198.51.100.8/32"),
		raw("203.0.113.1/32"),
		raw("198.51.100.9/32"),
	}
	require.Equal(t, []ipnet.RawEntry{
		raw("198.51.100.8/32"),
		raw("198.51.100.9/32"),
	}, filter.Apply(input))
}

func TestApplyEmptyReturnsEmpty(t *testing.T) {
	t.Parallel()

	filter := mustParse(t, ipnet.IP4, "addr-in(198.51.100.0/24)")
	require.Empty(t, filter.Apply(nil))
}
