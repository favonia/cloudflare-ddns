package detector_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestUnmanagedIsManaged(t *testing.T) {
	t.Parallel()

	require.False(t, detector.NewUnmanaged().IsManaged())
}

func TestUnmanagedString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "unmanaged", detector.NewUnmanaged().String())
}

func TestUnmanagedGetIP(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]ipnet.Type{
		"4": ipnet.IP4,
		"6": ipnet.IP6,
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ip, ok := detector.NewUnmanaged().GetIP(context.Background(), 3, tc)
			require.False(t, ok)
			require.Nil(t, ip)
		})
	}
}
