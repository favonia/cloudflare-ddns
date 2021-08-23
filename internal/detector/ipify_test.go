package detector_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
)

func TestIpifyName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "ipify", detector.Name(detector.NewIpify()))
}
