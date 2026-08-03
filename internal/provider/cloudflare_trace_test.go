package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestCloudflareTraceName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "cloudflare.trace", provider.Name(provider.NewCloudflareTrace()))
}

func TestMustNewCloudflareTrace(t *testing.T) {
	t.Parallel()

	p, ok := provider.NewCloudflareTrace().(protocol.CloudflareTrace)
	require.True(t, ok)
	require.Equal(t, "cloudflare.trace", p.ProviderName)
	wantEndpoints := []string{
		"https://api.cloudflare.com/cdn-cgi/trace",
		"https://www.cloudflare.com/cdn-cgi/trace",
		"https://connectivity.cloudflareclient.com/cdn-cgi/trace",
	}
	// Mutation caught: changing, reordering, or omitting a built-in endpoint.
	require.Equal(t, wantEndpoints, p.URLs[ipnet.IP4])
	require.Equal(t, wantEndpoints, p.URLs[ipnet.IP6])
}
