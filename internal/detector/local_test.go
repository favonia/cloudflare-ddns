package detector_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestLocalIsManaged(t *testing.T) {
	t.Parallel()

	policy := detector.Local{
		PolicyName:    "",
		RemoteUDPAddr: nil,
	}

	require.True(t, policy.IsManaged())
}

func TestLocalString(t *testing.T) {
	t.Parallel()

	policy := detector.Local{
		PolicyName:    "very secret name",
		RemoteUDPAddr: nil,
	}

	require.Equal(t, "very secret name", policy.String())
}

func TestLocalGetIP(t *testing.T) {
	t.Parallel()

	ip4Loopback := net.ParseIP("127.0.0.1").To4()
	ip6Loopback := net.ParseIP("::1").To16()

	for name, tc := range map[string]struct {
		addrKey  ipnet.Type
		addr     string
		ipNet    ipnet.Type
		expected net.IP
	}{
		"4":      {ipnet.IP4, "127.0.0.1:80", ipnet.IP4, ip4Loopback},
		"6":      {ipnet.IP6, "[::1]:80", ipnet.IP6, ip6Loopback},
		"4-nil1": {ipnet.IP4, "", ipnet.IP4, nil},
		"6-nil1": {ipnet.IP6, "", ipnet.IP6, nil},
		"4-nil2": {ipnet.IP4, "127.0.0.1:80", ipnet.IP6, nil},
		"6-nil2": {ipnet.IP6, "::1:80", ipnet.IP4, nil},
	} {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			policy := &detector.Local{
				PolicyName: "",
				RemoteUDPAddr: map[ipnet.Type]string{
					tc.addrKey: tc.addr,
				},
			}

			ip := policy.GetIP(context.Background(), 3, tc.ipNet)
			require.Equal(t, tc.expected, ip)
		})
	}
}
