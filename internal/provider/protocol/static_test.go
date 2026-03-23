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

func liftedRawEntries(ipFamily ipnet.Family, ips []netip.Addr) []ipnet.RawEntry {
	return ipnet.LiftValidatedIPsToRawEntries(ips, protocol.DefaultRawDataPrefixLen(ipFamily))
}

func mustRawEntry(s string) ipnet.RawEntry {
	return ipnet.RawEntry(netip.MustParsePrefix(s))
}

func TestDefaultRawDataPrefixLen(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ipFamily ipnet.Family
		expected int
	}{
		"ipv4":    {ipnet.IP4, 32},
		"ipv6":    {ipnet.IP6, 64},
		"unknown": {ipnet.Family(100), 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, protocol.DefaultRawDataPrefixLen(tc.ipFamily))
		})
	}
}

func TestNewStatic(t *testing.T) {
	t.Parallel()

	original := []ipnet.RawEntry{mustRawEntry("1.1.1.1/32"), mustRawEntry("2.2.2.2/32")}
	p := protocol.NewStatic("test", original)

	require.Equal(t, "test", p.ProviderName)
	require.Equal(t, original, p.RawEntries)

	// Verify defensive copy: mutating the original slice should not affect the provider.
	original[0] = mustRawEntry("3.3.3.3/32")
	require.Equal(t, mustRawEntry("1.1.1.1/32"), p.RawEntries[0])
}

func TestNewStaticNil(t *testing.T) {
	t.Parallel()

	p := protocol.NewStatic("empty", nil)
	require.Equal(t, "empty", p.ProviderName)
	require.Empty(t, p.RawEntries)
}

func TestStaticName(t *testing.T) {
	t.Parallel()

	p := &protocol.Static{
		ProviderName: "very secret name",
		RawEntries:   nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestStaticGetRawData(t *testing.T) {
	t.Parallel()

	var invalidEntry ipnet.RawEntry

	for name, tc := range map[string]struct {
		savedRawEntries []ipnet.RawEntry
		ipFamily        ipnet.Family
		ok              bool
		expected        []ipnet.RawEntry
		prepareMockPP   func(*mocks.MockPP)
	}{
		"valid/4": {
			[]ipnet.RawEntry{mustRawEntry("1.1.1.1/32")},
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{mustRawEntry("1.1.1.1/32")},
			nil,
		},
		"valid/6": {
			[]ipnet.RawEntry{mustRawEntry("1::1/64")},
			ipnet.IP6,
			true,
			[]ipnet.RawEntry{mustRawEntry("1::1/64")},
			nil,
		},
		"valid/4/deduplicate-sort": {
			[]ipnet.RawEntry{
				mustRawEntry("2.2.2.2/32"),
				mustRawEntry("1.1.1.1/32"),
				mustRawEntry("2.2.2.2/32"),
			},
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{
				mustRawEntry("1.1.1.1/32"),
				mustRawEntry("2.2.2.2/32"),
			},
			nil,
		},
		"valid/4/mapped-prefix": {
			[]ipnet.RawEntry{mustRawEntry("::ffff:10.10.10.10/128")},
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{mustRawEntry("10.10.10.10/32")},
			nil,
		},
		"error/invalid": {
			[]ipnet.RawEntry{invalidEntry},
			ipnet.IP6,
			false, nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Detected address is not valid; this should not happen and please report it at %s", pp.IssueReportingURL)
			},
		},
		"error/6-as-4": {
			[]ipnet.RawEntry{mustRawEntry("1::1/64")},
			ipnet.IP4,
			false, nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Detected address %s is not a valid IPv4 address and cannot be used", "1::1/64")
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
				RawEntries:   tc.savedRawEntries,
			}
			rawData := provider.GetRawData(context.Background(), mockPP, tc.ipFamily, map[ipnet.Family]int{
				ipnet.IP4: 32,
				ipnet.IP6: 64,
			}[tc.ipFamily])
			require.Equal(t, tc.ok, rawData.Available)
			if tc.ok {
				require.Equal(t, tc.expected, rawData.RawEntries)
			} else {
				require.Empty(t, rawData.RawEntries)
			}
		})
	}
}

func TestHasUsableRawData(t *testing.T) {
	t.Parallel()

	require.True(t, protocol.NewKnownDetectionResult([]ipnet.RawEntry{ipnet.RawEntryFrom(netip.MustParseAddr("1.1.1.1"), 32)}).HasUsableRawData())
	require.True(t, protocol.NewKnownDetectionResult(nil).HasUsableRawData())
	require.False(t, protocol.NewUnavailableDetectionResult().HasUsableRawData())
}

func TestStaticIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.True(t, protocol.Static{ProviderName: "", RawEntries: nil}.IsExplicitEmpty())
	require.True(t, protocol.Static{ProviderName: "", RawEntries: []ipnet.RawEntry{}}.IsExplicitEmpty())
	require.False(t, protocol.Static{ProviderName: "", RawEntries: []ipnet.RawEntry{
		mustRawEntry("1.1.1.1/32"),
	}}.IsExplicitEmpty())
}
