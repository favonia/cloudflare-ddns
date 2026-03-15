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

func TestMustLiteral(t *testing.T) {
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
				require.NotPanics(t, func() { provider.MustNewLiteral(tc.input) })
			} else {
				require.Panics(t, func() { provider.MustNewLiteral(tc.input) })
			}
		})
	}
}

func TestLiteralName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "literal:1.1.1.1,2.2.2.2", provider.Name(provider.MustNewLiteral("2.2.2.2,1.1.1.1,2.2.2.2")))
}

func TestLiteralGetIPs(t *testing.T) {
	t.Parallel()

	p := provider.MustNewLiteral("2.2.2.2,1.1.1.1,2.2.2.2")
	targets := p.GetIPs(context.Background(), pp.NewSilent(), ipnet.IP4)

	require.True(t, targets.Available)
	require.Equal(t, []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2.2.2.2"),
	}, targets.IPs)
}

func TestLiteralGetIPsFailure(t *testing.T) {
	t.Parallel()

	p := provider.MustNewLiteral("1.1.1.1,1::1")
	targets := p.GetIPs(context.Background(), pp.NewSilent(), ipnet.IP6)

	require.False(t, targets.Available)
	require.Nil(t, targets.IPs)
}

func TestLiteralGetIPsBoundaryFailure(t *testing.T) {
	t.Parallel()

	p := provider.MustNewLiteral("127.0.0.1")
	targets := p.GetIPs(context.Background(), pp.NewSilent(), ipnet.IP4)

	require.False(t, targets.Available)
	require.Nil(t, targets.IPs)
}
