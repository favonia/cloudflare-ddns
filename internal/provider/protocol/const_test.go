// vim: nowrap
//go:build linux

package protocol_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestConstName(t *testing.T) {
	t.Parallel()

	p := &protocol.Const{
		ProviderName: "very secret name",
		IP:           netip.Addr{},
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestConstGetIP(t *testing.T) {
	t.Parallel()

	var invalidIP netip.Addr

	for name, tc := range map[string]struct {
		savedIP       netip.Addr
		ipNet         ipnet.Type
		ok            bool
		expected      netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"valid/4": {
			netip.MustParseAddr("1.1.1.1"), ipnet.IP4,
			true, netip.MustParseAddr("1.1.1.1"), nil,
		},
		"valid/6": {
			netip.MustParseAddr("1::1"), ipnet.IP6,
			true, netip.MustParseAddr("1::1"), nil,
		},
		"error/zoned": {
			netip.MustParseAddr("1::1%1"), ipnet.IP6,
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiError,
					"Detected %s address %s has a zone identifier and cannot be used as a target address",
					"IPv6", "1::1%1",
				)
			},
		},
		"error/invalid": {
			invalidIP, ipnet.IP6,
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Detected IP address is not valid; this should not happen and please report it at %s", pp.IssueReportingURL)
			},
		},
		"error/6-as-4": {
			netip.MustParseAddr("1::1"), ipnet.IP4,
			false, invalidIP,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s is not a valid IPv4 address", "1::1")
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

			provider := &protocol.Const{
				ProviderName: "",
				IP:           tc.savedIP,
			}
			ip, ok := provider.GetIP(context.Background(), mockPP, tc.ipNet)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, ip)
		})
	}
}
