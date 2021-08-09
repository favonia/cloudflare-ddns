package detector_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
)

func TestCloudflareIsManaged(t *testing.T) {
	t.Parallel()

	require.True(t, detector.NewCloudflare().IsManaged())
}

func TestCloudflareString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "cloudflare", detector.NewCloudflare().String())
}
