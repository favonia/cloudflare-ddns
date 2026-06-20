package hostid6

import (
	"fmt"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// EmitMACShortPrefixHint advises operators whose MAC host IDs were rejected for
// a detected prefix shorter than /64. It quotes the deterministic interface
// identifier of each MAC derivation (with the subnet bits assumed zero) and
// asks the operator to supply the real subnet bits, which the MAC alone does
// not determine. The set should contain only MAC derivations; others are
// skipped.
func EmitMACShortPrefixHint(ppfmt pp.PP, macs Set, observed netip.Prefix) {
	hosts := make([]string, 0, macs.Len())
	literals := make([]Derivation, 0, macs.Len())
	for derivation := range macs.All() {
		host, ok := MACHostID(derivation)
		if !ok {
			continue
		}
		hosts = append(hosts, host.String())
		literal, err := Literal(host)
		if err != nil {
			panic(fmt.Sprintf("invalid MAC-derived host-ID literal %s", host))
		}
		literals = append(literals, literal)
	}
	if len(hosts) == 0 {
		return
	}

	hostList := pp.EnglishJoinOrEmptyLabel(hosts, "(none)")
	configString := NewSet(literals...).ConfigString()
	if len(hosts) == 1 {
		ppfmt.NoticeOncef(pp.MessageHostID6MACPrefix, pp.EmojiHint,
			"MAC-based host IDs require a /64 prefix. For %s, look up the subnet bits between /%d and /64; "+
				"the MAC-derived interface identifier is %s. If those subnet bits are zero, use hostid6=%s. "+
				"If they are not zero, insert them into the hostid6 literal before the interface identifier. "+
				"Please open an issue at %s if you need direct MAC support for shorter prefixes",
			observed.String(), observed.Bits(), hostList, configString, pp.IssueReportingURL)
		return
	}

	ppfmt.NoticeOncef(pp.MessageHostID6MACPrefix, pp.EmojiHint,
		"MAC-based host IDs require a /64 prefix. For %s, look up the subnet bits between /%d and /64; "+
			"the MAC-derived interface identifiers are %s. If those subnet bits are zero, use hostid6=%s. "+
			"If they are not zero, insert them into the hostid6 literal before the interface identifier. "+
			"Please open an issue at %s if you need direct MAC support for shorter prefixes",
		observed.String(), observed.Bits(), hostList, configString, pp.IssueReportingURL)
}
