package hostid6_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestEmitMACShortPrefixHint(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(mockCtrl)
	ppfmt.EXPECT().NoticeOncef(pp.MessageHostID6MACPrefix, pp.EmojiHint,
		"Modified EUI-64 host IDs are only defined within a /64 prefix. "+
			"Assuming the subnet bits are all zero, %s; look up the subnet bits between your prefix and /64 "+
			"(often zero on a single-subnet network), prepend them, and use the result as a literal hostid6 "+
			"until shorter prefixes are supported. Please open an issue at %s if you need this",
		"mac(00-11-22-33-44-55) gives ::211:22ff:fe33:4455 and mac(aa-bb-cc-dd-ee-ff) gives ::a8bb:ccff:fedd:eeff",
		pp.IssueReportingURL)

	hostid6.EmitMACShortPrefixHint(ppfmt, hostid6.NewSet(
		hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}),
		hostid6.MAC([6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}),
	))
}

func TestEmitMACShortPrefixHintSkipsNonMAC(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(mockCtrl)
	ppfmt.EXPECT().NoticeOncef(pp.MessageHostID6MACPrefix, pp.EmojiHint,
		"Modified EUI-64 host IDs are only defined within a /64 prefix. "+
			"Assuming the subnet bits are all zero, %s; look up the subnet bits between your prefix and /64 "+
			"(often zero on a single-subnet network), prepend them, and use the result as a literal hostid6 "+
			"until shorter prefixes are supported. Please open an issue at %s if you need this",
		"mac(00-11-22-33-44-55) gives ::211:22ff:fe33:4455",
		pp.IssueReportingURL)

	// The preserve derivation is not a MAC and is skipped; only the MAC is quoted.
	hostid6.EmitMACShortPrefixHint(ppfmt, hostid6.NewSet(
		hostid6.Preserve(),
		hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}),
	))
}
