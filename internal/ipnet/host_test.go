// vim: nowrap
package ipnet_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestHostIDDescribe(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input    ipnet.HostID
		expected string
	}{
		"ip6suffix": {
			ipnet.IP6Suffix{0x00, 0x00, 0x00, 0x00, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			"::4455:6677:8899:aabb:ccdd:eeff",
		},
		"mac": {
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			"aa:bb:cc:dd:ee:ff",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.Describe())
		})
	}
}
