package provider_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestMustStatic(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		ok    bool
	}{
		{"1.1.1.1", true},
		{"  2.2.2.2,1.1.1.1,2.2.2.2 ", true},
		{"1::1%1", false},
		{"1.1.1.1,", false},
		{"", false},
		{"blah", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			if tc.ok {
				require.NotPanics(t, func() { provider.MustNewStatic(tc.input) })
			} else {
				require.Panics(t, func() { provider.MustNewStatic(tc.input) })
			}
		})
	}
}

func TestStaticName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "static:1.1.1.1,2.2.2.2", provider.Name(provider.MustNewStatic("2.2.2.2,1.1.1.1,2.2.2.2")))
}

func TestStaticEmptyName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "static.empty", provider.Name(provider.NewStaticEmpty()))
}

func TestStaticGetIPs(t *testing.T) {
	t.Parallel()

	p := provider.MustNewStatic("2.2.2.2,1.1.1.1,2.2.2.2")
	targets := p.GetIPs(context.Background(), pp.NewSilent(), ipnet.IP4)

	require.True(t, targets.Available)
	require.Equal(t, []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2.2.2.2"),
	}, targets.IPs)
}

func TestStaticEmptyGetIPs(t *testing.T) {
	t.Parallel()

	p := provider.NewStaticEmpty()
	targets := p.GetIPs(context.Background(), pp.NewSilent(), ipnet.IP6)

	require.True(t, targets.Available)
	require.Empty(t, targets.IPs)
}

func TestStaticGetIPsFailure(t *testing.T) {
	t.Parallel()

	p := provider.MustNewStatic("1.1.1.1,1::1")
	targets := p.GetIPs(context.Background(), pp.NewSilent(), ipnet.IP6)

	require.False(t, targets.Available)
	require.Nil(t, targets.IPs)
}

func TestStaticGetIPsBoundaryFailure(t *testing.T) {
	t.Parallel()

	p := provider.MustNewStatic("127.0.0.1")
	targets := p.GetIPs(context.Background(), pp.NewSilent(), ipnet.IP4)

	require.False(t, targets.Available)
	require.Nil(t, targets.IPs)
}
