package detector_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
)

func TestLocalCloudflareName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "local", detector.Name(detector.NewLocal()))
}
