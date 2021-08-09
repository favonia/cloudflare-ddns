package detector_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
)

func TestIpifyIsManaged(t *testing.T) {
	t.Parallel()

	require.True(t, detector.NewIpify().IsManaged())
}

func TestIpifyString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "ipify", detector.NewIpify().String())
}
