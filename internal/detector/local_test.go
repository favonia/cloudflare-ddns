package detector_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
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
		addrKey   ipnet.Type
		addr      string
		ipNet     ipnet.Type
		expected  net.IP
		ppRecords []pp.Record
	}{
		"4": {ipnet.IP4, "127.0.0.1:80", ipnet.IP4, ip4Loopback, nil},
		"6": {ipnet.IP6, "[::1]:80", ipnet.IP6, ip6Loopback, nil},
		"4-nil1": {
			ipnet.IP4, "", ipnet.IP4, nil,
			[]pp.Record{
				pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to detect a local IPv4 address: dial udp4: missing address`),
			},
		},
		"6-nil1": {
			ipnet.IP6, "", ipnet.IP6, nil,
			[]pp.Record{
				pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to detect a local IPv6 address: dial udp6: missing address`),
			},
		},
		"4-nil2": {
			ipnet.IP4, "127.0.0.1:80", ipnet.IP6, nil,
			[]pp.Record{
				pp.NewRecord(0, pp.Warning, pp.EmojiImpossible, `Unhandled IP network: IPv6`),
			},
		},
		"6-nil2": {
			ipnet.IP6, "::1:80", ipnet.IP4, nil,
			[]pp.Record{
				pp.NewRecord(0, pp.Warning, pp.EmojiImpossible, `Unhandled IP network: IPv4`),
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			policy := &detector.Local{
				PolicyName: "",
				RemoteUDPAddr: map[ipnet.Type]string{
					tc.addrKey: tc.addr,
				},
			}

			ppmock := pp.NewMock()
			ip := policy.GetIP(context.Background(), ppmock, tc.ipNet)
			require.Equal(t, tc.expected, ip)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}
