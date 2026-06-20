package hostid6_test

import (
	"net/netip"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestEmitMACShortPrefixHint(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(mockCtrl)
	ppfmt.EXPECT().NoticeOncef(pp.MessageHostID6MACPrefix, pp.EmojiHint,
		"MAC-based host IDs require a /64 prefix. For %s, look up the subnet bits between /%d and /64; "+
			"the MAC-derived %s %s. If those subnet bits are zero, use hostid6=%s. "+
			"If they are not zero, insert them into the hostid6 literal before the interface identifier. "+
			"Please open an issue at %s if you need direct MAC support for shorter prefixes",
		"2001:db8:1234::abcd/56",
		56,
		"interface identifiers are",
		"::211:22ff:fe33:4455 and ::a8bb:ccff:fedd:eeff",
		"[::211:22ff:fe33:4455,::a8bb:ccff:fedd:eeff]",
		pp.IssueReportingURL)

	hostid6.EmitMACShortPrefixHint(ppfmt, hostid6.NewSet(
		hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}),
		hostid6.MAC([6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}),
	), ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8:1234::abcd"), 56))
}

func TestEmitMACShortPrefixHintSkipsNonMAC(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(mockCtrl)
	ppfmt.EXPECT().NoticeOncef(pp.MessageHostID6MACPrefix, pp.EmojiHint,
		"MAC-based host IDs require a /64 prefix. For %s, look up the subnet bits between /%d and /64; "+
			"the MAC-derived %s %s. If those subnet bits are zero, use hostid6=%s. "+
			"If they are not zero, insert them into the hostid6 literal before the interface identifier. "+
			"Please open an issue at %s if you need direct MAC support for shorter prefixes",
		"2001:db8:1234::abcd/56",
		56,
		"interface identifier is",
		"::211:22ff:fe33:4455",
		"::211:22ff:fe33:4455",
		pp.IssueReportingURL)

	// The preserve derivation is not a MAC and is skipped; only the MAC is quoted.
	hostid6.EmitMACShortPrefixHint(ppfmt, hostid6.NewSet(
		hostid6.Preserve(),
		hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}),
	), ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8:1234::abcd"), 56))
}

func TestEmitMACShortPrefixHintSkipsEmptyMACSet(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(mockCtrl)

	hostid6.EmitMACShortPrefixHint(ppfmt, hostid6.NewSet(hostid6.Preserve()),
		ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8:1234::abcd"), 56))
}
