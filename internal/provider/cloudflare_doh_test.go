package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestCloudflareName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "cloudflare.doh", provider.Name(provider.NewCloudflareDOH(true)))
	require.Equal(t, "cloudflare.doh", provider.Name(provider.NewCloudflareDOH(false)))
}
