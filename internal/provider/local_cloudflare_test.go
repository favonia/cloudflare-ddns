package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestLocalCloudflareName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "local", provider.Name(provider.NewLocal()))
}
