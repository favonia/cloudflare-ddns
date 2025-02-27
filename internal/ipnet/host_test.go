// vim: nowrap
package ipnet_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
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

func TestNormalize(t *testing.T) {
	t.Parallel()
	domain := domain.FQDN("a.b.c")
	for name, tc := range map[string]struct {
		input        ipnet.HostID
		prefixLen    int
		ok           bool
		expected     ipnet.HostID
		prepareMocks func(*mocks.MockPP)
	}{
		"ip6suffix/0": {
			ipnet.IP6Suffix{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			0,
			true,
			ipnet.IP6Suffix{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			nil,
		},
		"ip6suffix/128": {
			ipnet.IP6Suffix{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			128,
			true,
			ipnet.IP6Suffix{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Infof(pp.EmojiTruncate, "The host ID %q of %q was truncated to %q (with %d higher bits removed)", "1:203:405:607:809:a0b:c0d:e0f", "a.b.c", "::", 128)
			},
		},
		"ip6suffix/-1": {
			ipnet.IP6Suffix{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			-1,
			false,
			nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "IP6_PREFIX_LEN (%d) should be in the range 0 to 128", -1)
			},
		},
		"ip6suffix/129": {
			ipnet.IP6Suffix{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
			129,
			false,
			nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "IP6_PREFIX_LEN (%d) should be in the range 0 to 128", 129)
			},
		},
		"mac/0": {
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			0,
			true,
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			nil,
		},
		"mac/128": {
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			128,
			false,
			nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiUserError, "IP6_PREFIX_LEN (%d) is too large (> 64) to use the MAC (EUI-48) address %q as the IPv6 host ID of %q. Converting a MAC address to a host ID requires IPv6 Stateless Address Auto-configuration (SLAAC), which necessitates an IPv6 range of size at least /64 (represented by a prefix length at most 64).", 128, "aa:bb:cc:dd:ee:ff", "a.b.c")
			},
		},
		"mac/-1": {
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			-1,
			false,
			nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "IP6_PREFIX_LEN (%d) should be in the range 0 to 128", -1)
			},
		},
		"mac/96": {
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			96,
			false,
			nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiUserError, "IP6_PREFIX_LEN (%d) is too large (> 64) to use the MAC (EUI-48) address %q as the IPv6 host ID of %q. Converting a MAC address to a host ID requires IPv6 Stateless Address Auto-configuration (SLAAC), which necessitates an IPv6 range of size at least /64 (represented by a prefix length at most 64).", 96, "aa:bb:cc:dd:ee:ff", "a.b.c")
			},
		},
		"mac/64": {
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			64,
			true,
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}

			result, ok := tc.input.Normalize(mockPP, domain, tc.prefixLen)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestHostIDWithPrefix(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  ipnet.HostID
		prefix netip.Prefix
		addr   netip.Addr
	}{
		"ip6suffix": {
			ipnet.IP6Suffix{0x00, 0x00, 0x00, 0x00, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			netip.MustParsePrefix("1122::/40"),
			netip.MustParseAddr("1122::55:6677:8899:aabb:ccdd:eeff"),
		},
		"mac": {
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			netip.MustParsePrefix("1122::/24"),
			netip.MustParseAddr("1122::a8bb:ccff:fedd:eeff"),
		},
		"mac/96": {
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			netip.MustParsePrefix("1122::/96"),
			netip.Addr{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			addr := tc.input.WithPrefix(tc.prefix)
			require.Equal(t, tc.addr, addr)
		})
	}
}

func TestParseHostID(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  string
		err    error
		hostID ipnet.HostID
	}{
		"empty": {"", nil, nil},
		"ip6": {
			"11:2233:4455:6677:8899:aabb:ccdd:eeff",
			nil,
			ipnet.IP6Suffix{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		},
		"ip6/zone": {
			"11:2233:4455:6677:8899:aabb:ccdd:eeff%eth0",
			ipnet.ErrHostIDHasIP6Zone,
			nil,
		},
		"mac": {
			"aa:bb:cc:dd:ee:ff",
			nil,
			ipnet.EUI48{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		},
		"ip4":          {"1.1.1.1", ipnet.ErrIP4AddressAsHostID, nil},
		"eui64":        {"01-02-03-04-05-06-07-08", ipnet.ErrEUI64AsHostID, nil},
		"ipoib":        {"01-02-03-04-05-06-07-08-09-0A-0B-0C-0D-0E-0F-10-11-12-13-14", ipnet.ErrIPOIBAsHostID, nil},
		"ill-formed/1": {"1:1:1:1", nil, nil},
		"ill-formed/2": {"01-02-03-04-05-06-07-08-09", nil, nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hostID, err := ipnet.ParseHost(tc.input)
			require.Equal(t, tc.hostID, hostID)
			if tc.input == "" || hostID != nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				if tc.err != nil {
					require.ErrorIs(t, err, tc.err)
				}
			}
		})
	}
}
