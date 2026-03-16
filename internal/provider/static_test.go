package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestMustStatic(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		ipFamily ipnet.Family
		input    string
		ok       bool
	}{
		{ipnet.IP4, "1.1.1.1", true},
		{ipnet.IP6, "1.1.1.1", false},
		{ipnet.IP4, "1.1.1.1,1::1", false},
		{ipnet.IP6, "1.1.1.1,1::1", false},
		{ipnet.IP4, "  2.2.2.2,1.1.1.1,2.2.2.2 ", true},
		{ipnet.IP6, "1::1%1", false},
		{ipnet.IP4, "1.1.1.1,", false},
		{ipnet.IP4, "", false},
		{ipnet.IP6, "blah", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			if tc.ok {
				require.NotPanics(t, func() { provider.MustNewStatic(tc.ipFamily, tc.input) })
			} else {
				require.Panics(t, func() { provider.MustNewStatic(tc.ipFamily, tc.input) })
			}
		})
	}
}

func TestStaticName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "static:1.1.1.1,2.2.2.2", provider.Name(provider.MustNewStatic(ipnet.IP4, "2.2.2.2,1.1.1.1,2.2.2.2")))
}

func TestStaticEmptyName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "static.empty", provider.Name(provider.NewStaticEmpty()))
}

func TestStaticIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.True(t, provider.NewStaticEmpty().IsExplicitEmpty())
	require.False(t, provider.MustNewStatic(ipnet.IP4, "1.1.1.1").IsExplicitEmpty())
}
