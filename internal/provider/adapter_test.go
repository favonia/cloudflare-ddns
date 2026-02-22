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

func TestProviderGetIPsSingleton(t *testing.T) {
	t.Parallel()

	p := provider.MustNewDebugConst("1.1.1.1")
	ips, ok := p.GetIPs(context.Background(), pp.NewDefault(io.Discard), ipnet.IP4)

	require.True(t, ok)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("1.1.1.1")}, ips)
}

func TestProviderGetIPsFailure(t *testing.T) {
	t.Parallel()

	p := provider.MustNewDebugConst("1.1.1.1")
	ips, ok := p.GetIPs(context.Background(), pp.NewDefault(io.Discard), ipnet.IP6)

	require.False(t, ok)
	require.Nil(t, ips)
}
