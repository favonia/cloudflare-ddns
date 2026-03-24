package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestCloudflareDOHName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "cloudflare.doh", provider.Name(provider.NewCloudflareDOH()))
}

func TestCloudflareDOHIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.False(t, provider.NewCloudflareDOH().IsExplicitEmpty())
}
