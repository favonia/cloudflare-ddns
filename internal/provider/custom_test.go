package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestCustomName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "custom", provider.Name(provider.NewCustom("")))
}
