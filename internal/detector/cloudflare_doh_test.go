package detector_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
)

func TestCloudflareName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "cloudflare.doh", detector.Name(detector.NewCloudflareDOH()))
}
