package provider_test

import (
	"context"
	"io"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestDebugConstName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "debug.const:1.1.1.1", provider.Name(provider.MustNewDebugConst("1.1.1.1")))
}

func TestMustDebugConst(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		ok    bool
	}{
		{"1.1.1.1", true},
		{"1::1%1", false},
		{"", false},
		{"blah", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			if tc.ok {
				require.NotPanics(t, func() { provider.MustNewDebugConst(tc.input) })
			} else {
				require.Panics(t, func() { provider.MustNewDebugConst(tc.input) })
			}
		})
	}
}

func TestDebugConstGetIPs(t *testing.T) {
	t.Parallel()

	p := provider.MustNewDebugConst("1.1.1.1")
	ips, ok := p.GetIPs(context.Background(), pp.NewDefault(io.Discard), ipnet.IP4)

	require.True(t, ok)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("1.1.1.1")}, ips)
}

func TestDebugConstGetIPsFailure(t *testing.T) {
	t.Parallel()

	p := provider.MustNewDebugConst("1.1.1.1")
	ips, ok := p.GetIPs(context.Background(), pp.NewDefault(io.Discard), ipnet.IP6)

	require.False(t, ok)
	require.Nil(t, ips)
}
