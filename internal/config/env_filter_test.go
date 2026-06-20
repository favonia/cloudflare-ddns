package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipfilter"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestReadDetectionFilterDefault(t *testing.T) {
	t.Setenv("TEST_FILTER", "")
	filter := ipfilter.KeepAll()
	require.True(t, readDetectionFilter(pp.NewSilent(), "TEST_FILTER", ipnet.IP4, &filter))
	require.Equal(t, "keep-all", filter.String())
}

func TestReadDetectionFilterValid(t *testing.T) {
	t.Setenv("TEST_FILTER", "addr-in(198.51.100.0/24)")
	filter := ipfilter.KeepAll()
	require.True(t, readDetectionFilter(pp.NewSilent(), "TEST_FILTER", ipnet.IP4, &filter))
	require.Equal(t, "addr-in(198.51.100.0/24)", filter.String())
}

func TestReadDetectionFilterInvalid(t *testing.T) {
	t.Setenv("TEST_FILTER", "addr-in(2001:db8::/32)")
	filter := ipfilter.KeepAll()
	var output strings.Builder
	require.False(t, readDetectionFilter(pp.New(&output, false, pp.Quiet), "TEST_FILTER", ipnet.IP4, &filter))
	require.Contains(t, output.String(), `TEST_FILTER ("addr-in(2001:db8::/32)") contains IPv6 prefix`)
}
