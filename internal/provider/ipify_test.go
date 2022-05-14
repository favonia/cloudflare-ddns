package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestIpifyName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "ipify", provider.Name(provider.NewIpify()))
}
