package protocol

import (
	"bytes"
	"net"
	"net/netip"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestDetectInterfaceIPsReadFailure(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	result := detectInterfaceIPs(pp.New(&output, false, pp.Verbose), "test0", ipnet.IP4, 32,
		func() ([]net.Addr, error) {
			return nil, syscall.ENODEV
		})
	require.Equal(t, NewUnavailableDetectionResult(), result)
	require.Contains(t, output.String(), "Failed to list unicast addresses of interface test0")
}

func TestDetectInterfaceIPsSuccessfulRead(t *testing.T) {
	t.Parallel()

	result := detectInterfaceIPs(pp.NewSilent(), "test0", ipnet.IP4, 24,
		func() ([]net.Addr, error) {
			return []net.Addr{
				&net.IPAddr{IP: net.ParseIP("192.0.2.2"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("::ffff:192.0.2.1"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("192.0.2.2"), Zone: ""},
				&net.IPAddr{IP: net.ParseIP("2001:db8::1"), Zone: ""},
			}, nil
		})
	require.Equal(t, NewKnownDetectionResult([]ipnet.RawEntry{
		ipnet.RawEntryFrom(netip.MustParseAddr("192.0.2.1"), 24),
		ipnet.RawEntryFrom(netip.MustParseAddr("192.0.2.2"), 24),
	}), result)
}
