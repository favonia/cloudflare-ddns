package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestCloudflareTraceName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "cloudflare.trace", provider.Name(provider.NewCloudflareTrace()))
}
