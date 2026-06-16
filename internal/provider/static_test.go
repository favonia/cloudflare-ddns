package provider_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestMustStatic(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		ipFamily         ipnet.Family
		defaultPrefixLen int
		input            string
		ok               bool
	}{
		{ipnet.IP4, 32, "1.1.1.1", true},
		{ipnet.IP6, 64, "1.1.1.1", false},
		{ipnet.IP4, 32, "1.1.1.1,1::1", false},
		{ipnet.IP6, 64, "1.1.1.1,1::1", false},
		{ipnet.IP4, 32, "  2.2.2.2,1.1.1.1,2.2.2.2 ", true},
		{ipnet.IP6, 64, "1::1%1", false},
		{ipnet.IP4, 32, "1.1.1.1,", true},
		{ipnet.IP4, 32, "1.1.1.1,,2.2.2.2", false},
		{ipnet.IP4, 32, "", false},
		{ipnet.IP6, 64, "blah", false},
		{ipnet.IP4, 32, "127.0.0.1", false},
		{ipnet.IP4, 32, "0.0.0.0", false},
		{ipnet.IP4, 32, "169.254.1.1", false},
		{ipnet.IP6, 64, "::ffff:1.1.1.1", false},
		{ipnet.IP4, 32, "255.255.255.255", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			if tc.ok {
				require.NotPanics(t, func() { provider.MustNewStatic(tc.ipFamily, tc.defaultPrefixLen, tc.input) })
			} else {
				require.Panics(t, func() { provider.MustNewStatic(tc.ipFamily, tc.defaultPrefixLen, tc.input) })
			}
		})
	}
}

func TestStaticName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "static:1.1.1.1,2.2.2.2", provider.Name(provider.MustNewStatic(ipnet.IP4, 32, "2.2.2.2,1.1.1.1,2.2.2.2")))
	require.Equal(t, "static:1.1.1.1", provider.Name(provider.MustNewStatic(ipnet.IP4, 32, "1.1.1.1,")))
}

func TestStaticEmptyName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "static.empty", provider.Name(provider.NewStaticEmpty()))
}

func TestStaticIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.True(t, provider.NewStaticEmpty().IsExplicitEmpty())
	require.False(t, provider.MustNewStatic(ipnet.IP4, 32, "1.1.1.1").IsExplicitEmpty())
}

func TestStaticRawData(t *testing.T) {
	t.Parallel()

	static := provider.MustNewStatic(ipnet.IP6, 64, "2001:db8::1/65")
	sp, ok := static.(provider.StaticProvider)
	require.True(t, ok)

	want := []ipnet.RawEntry{ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::1"), 65)}
	require.Equal(t, want, sp.StaticRawData())

	// The returned slice is a defensive clone: mutating it must not affect the provider.
	got := sp.StaticRawData()
	got[0] = ipnet.RawEntry{}
	require.Equal(t, want, sp.StaticRawData())

	// static.empty is a StaticProvider exposing an empty set.
	empty, ok := provider.NewStaticEmpty().(provider.StaticProvider)
	require.True(t, ok)
	require.Empty(t, empty.StaticRawData())

	// Dynamic providers do not implement StaticProvider.
	_, ok = provider.NewCloudflareTrace().(provider.StaticProvider)
	require.False(t, ok)
}
