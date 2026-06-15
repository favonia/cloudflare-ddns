package hostid6

import (
	"fmt"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// EmitMACShortPrefixHint advises operators whose MAC host IDs were rejected for
// a detected prefix shorter than /64. It quotes the deterministic interface
// identifier of each MAC derivation (with the subnet bits assumed zero) and
// asks the operator to supply the real subnet bits, which the MAC alone does
// not determine. The set should contain only MAC derivations; others are
// skipped.
func EmitMACShortPrefixHint(ppfmt pp.PP, macs Set) {
	hosts := make([]string, 0, macs.Len())
	for derivation := range macs.All() {
		host, ok := MACHostID(derivation)
		if !ok {
			continue
		}
		hosts = append(hosts, fmt.Sprintf("%s gives %s", derivation.String(), host))
	}

	ppfmt.NoticeOncef(pp.MessageHostID6MACPrefix, pp.EmojiHint,
		"Modified EUI-64 host IDs are only defined within a /64 prefix. "+
			"Assuming the subnet bits are all zero, %s; look up the subnet bits between your prefix and /64 "+
			"(often zero on a single-subnet network), prepend them, and use the result as a literal hostid6 "+
			"until shorter prefixes are supported. Please open an issue at %s if you need this",
		pp.EnglishJoinOrEmptyLabel(hosts, "(none)"), pp.IssueReportingURL)
}
