package detector_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
)

func TestLocalCloudflareIsManaged(t *testing.T) {
	t.Parallel()

	require.True(t, detector.NewLocal().IsManaged())
}

func TestLocalCloudflareString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "local", detector.NewLocal().String())
}
