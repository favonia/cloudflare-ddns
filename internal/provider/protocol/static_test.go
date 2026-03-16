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

func TestNewStatic(t *testing.T) {
	t.Parallel()

	original := []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("2.2.2.2")}
	p := protocol.NewStatic("test", original)

	require.Equal(t, "test", p.ProviderName)
	require.Equal(t, original, p.IPs)

	// Verify defensive copy: mutating the original slice should not affect the provider.
	original[0] = netip.MustParseAddr("3.3.3.3")
	require.Equal(t, netip.MustParseAddr("1.1.1.1"), p.IPs[0])
}

func TestNewStaticNil(t *testing.T) {
	t.Parallel()

	p := protocol.NewStatic("empty", nil)
	require.Equal(t, "empty", p.ProviderName)
	require.Empty(t, p.IPs)
}

func TestStaticName(t *testing.T) {
	t.Parallel()

	p := &protocol.Static{
		ProviderName: "very secret name",
		IPs:          nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestStaticGetIPs(t *testing.T) {
	t.Parallel()

	var invalidIP netip.Addr

	for name, tc := range map[string]struct {
		savedIPs      []netip.Addr
		ipFamily      ipnet.Family
		ok            bool
		expected      []netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"valid/4": {
			[]netip.Addr{netip.MustParseAddr("1.1.1.1")},
			ipnet.IP4,
			true,
			[]netip.Addr{netip.MustParseAddr("1.1.1.1")},
			nil,
		},
		"valid/6": {
			[]netip.Addr{netip.MustParseAddr("1::1")},
			ipnet.IP6,
			true,
			[]netip.Addr{netip.MustParseAddr("1::1")},
			nil,
		},
		"valid/4/deduplicate-sort": {
			[]netip.Addr{
				netip.MustParseAddr("2.2.2.2"),
				netip.MustParseAddr("1.1.1.1"),
				netip.MustParseAddr("2.2.2.2"),
			},
			ipnet.IP4,
			true,
			[]netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
				netip.MustParseAddr("2.2.2.2"),
			},
			nil,
		},
		"error/zoned": {
			[]netip.Addr{netip.MustParseAddr("1::1%1")},
			ipnet.IP6,
			false, nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiError,
					"Detected %s address %s has a zone identifier and cannot be used as a target address",
					"IPv6", "1::1%1",
				)
			},
		},
		"error/invalid": {
			[]netip.Addr{invalidIP},
			ipnet.IP6,
			false, nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Detected IP address is not valid; this should not happen and please report it at %s", pp.IssueReportingURL)
			},
		},
		"error/6-as-4": {
			[]netip.Addr{netip.MustParseAddr("1::1")},
			ipnet.IP4,
			false, nil,
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

			provider := &protocol.Static{
				ProviderName: "",
				IPs:          tc.savedIPs,
			}
			targets := provider.GetIPs(context.Background(), mockPP, tc.ipFamily)
			require.Equal(t, tc.ok, targets.Available)
			if tc.ok {
				require.Equal(t, tc.expected, targets.IPs)
			} else {
				require.Empty(t, targets.IPs)
			}
		})
	}
}

func TestHasUsableTargets(t *testing.T) {
	t.Parallel()

	require.True(t, protocol.NewAvailableTargets([]netip.Addr{netip.MustParseAddr("1.1.1.1")}).HasUsableTargets())
	require.True(t, protocol.NewAvailableTargets(nil).HasUsableTargets())
	require.False(t, protocol.NewUnavailableTargets().HasUsableTargets())
}

func TestStaticIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.True(t, protocol.Static{ProviderName: "", IPs: nil}.IsExplicitEmpty())
	require.True(t, protocol.Static{ProviderName: "", IPs: []netip.Addr{}}.IsExplicitEmpty())
	require.False(t, protocol.Static{ProviderName: "", IPs: []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
	}}.IsExplicitEmpty())
}
