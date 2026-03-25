package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestDefaultPrefixLenWithinRange(t *testing.T) {
	t.Parallel()

	raw := DefaultRaw()
	for _, tc := range [...]struct {
		ipFamily  ipnet.Family
		prefixLen int
	}{
		{ipnet.IP4, raw.IP4DefaultPrefixLen},
		{ipnet.IP6, raw.IP6DefaultPrefixLen},
	} {
		t.Run(tc.ipFamily.Describe(), func(t *testing.T) {
			t.Parallel()

			lo, hi := prefixLenRange(tc.ipFamily)
			require.GreaterOrEqual(t, tc.prefixLen, lo, "default prefix length is below the minimum")
			require.LessOrEqual(t, tc.prefixLen, hi, "default prefix length is above the maximum")
		})
	}
}

func TestPrefixLenRange(t *testing.T) {
	t.Parallel()

	for _, tc := range [...]struct {
		name     string
		ipFamily ipnet.Family
		lo       int
		hi       int
	}{
		{"ip4", ipnet.IP4, 8, 32},
		{"ip6", ipnet.IP6, 12, 128},
		{"unknown", ipnet.Family(0), 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lo, hi := prefixLenRange(tc.ipFamily)
			require.Equal(t, tc.lo, lo)
			require.Equal(t, tc.hi, hi)
		})
	}
}
