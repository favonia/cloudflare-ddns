package protocol_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestLocalAuteName(t *testing.T) {
	t.Parallel()

	p := &protocol.LocalAuto{
		ProviderName:  "very secret name",
		RemoteUDPAddr: "",
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestExtractUDPAddr(t *testing.T) {
	t.Parallel()

	var invalidIP netip.Addr

	for name, tc := range map[string]struct {
		input         net.Addr
		ok            bool
		output        netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"udpaddr/4": {
			&net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Zone: "", Port: 123},
			true, netip.MustParseAddr("1.2.3.4"),
			nil,
		},
		"udpaddr/6/zone-123": {
			&net.UDPAddr{IP: net.ParseIP("::1"), Zone: "123", Port: 123},
			true, netip.MustParseAddr("::1%123"),
			nil,
		},
		"udpaddr/illformed": {
			&net.UDPAddr{IP: net.IP([]byte{0x01, 0x02}), Zone: "", Port: 123},
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse UDP source address %q", "?0102")
			},
		},
		"dummy": {
			&Dummy{},
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Unexpected UDP source address data %q of type %T", "dummy/string", &Dummy{})
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			output, ok := protocol.ExtractUDPAddr(mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestLocalAuteGetIP(t *testing.T) {
	t.Parallel()

	invalidIP := netip.Addr{}

	for name, tc := range map[string]struct {
		addr          string
		ipNet         ipnet.Type
		ok            bool
		expected      netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"loopback/4": {
			"127.0.0.1:80", ipnet.IP4,
			false, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a loopback address", "IPv4", "127.0.0.1")
			},
		},
		"loopback/6": {
			"[::1]:80", ipnet.IP6,
			false, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected %s address %s is a loopback address", "IPv6", "::1")
			},
		},
		"empty/4": {
			"", ipnet.IP4,
			false, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to detect a local %s address: %v", "IPv4", gomock.Any())
			},
		},
		"empty/6": {
			"", ipnet.IP6,
			false, invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to detect a local %s address: %v", "IPv6", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			provider := &protocol.LocalAuto{
				ProviderName:  "",
				RemoteUDPAddr: tc.addr,
			}
			ip, method, ok := provider.GetIP(context.Background(), mockPP, tc.ipNet)
			require.Equal(t, tc.expected, ip)
			require.NotEqual(t, protocol.MethodAlternative, method)
			require.Equal(t, tc.ok, ok)
		})
	}
}
