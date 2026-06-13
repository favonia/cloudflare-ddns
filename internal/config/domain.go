package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/sliceutil"
)

type normalizedDomains struct {
	ByFamily map[ipnet.Family][]domain.Domain
	HostID6  map[domain.Domain]hostid6.Set
}

type hostID6Opinion struct {
	set    hostid6.Set
	source string
}

func describeHostID6Set(set hostid6.Set) string {
	values := set.Values()
	descriptions := make([]string, 0, len(values))
	for _, value := range values {
		descriptions = append(descriptions, value.Describe())
	}
	return "[" + strings.Join(descriptions, ",") + "]"
}

func mergeHostID6Opinions(
	ppfmt pp.PP,
	setting string,
	entries []domainexp.Entry,
	opinions map[domain.Domain]hostID6Opinion,
) bool {
	for declarationIndex, entry := range entries {
		for assignmentIndex, set := range entry.HostID6Opinions {
			source := fmt.Sprintf("%s declaration %d hostid6 assignment %d", setting, declarationIndex+1, assignmentIndex+1)
			previous, present := opinions[entry.Domain]
			if present && !hostid6.EqualSet(previous.set, set) {
				ppfmt.Noticef(pp.EmojiUserError,
					"Conflicting hostid6 settings for %s: %s configures %s, while %s configures %s; "+
						"configure exactly the same hostid6 set in every declaration or omit it from partial declarations",
					entry.Domain.Describe(), previous.source, describeHostID6Set(previous.set), source, describeHostID6Set(set))
				return false
			}
			if !present {
				opinions[entry.Domain] = hostID6Opinion{set: set, source: source}
			}
		}
	}
	return true
}

func projectDomains(entries ...[]domainexp.Entry) []domain.Domain {
	var domains []domain.Domain
	for _, settingEntries := range entries {
		for _, entry := range settingEntries {
			domains = append(domains, entry.Domain)
		}
	}
	return sliceutil.SortAndCompact(domains, domain.CompareDomain)
}

func warnSuspiciousMACs(ppfmt pp.PP, policies map[domain.Domain]hostid6.Set) {
	domainsByMAC := map[[6]byte][]domain.Domain{}
	for dom, set := range policies {
		for derivation := range set.All() {
			mac, ok := derivation.MACAddress()
			if ok {
				domainsByMAC[mac] = append(domainsByMAC[mac], dom)
			}
		}
	}

	macs := make([][6]byte, 0, len(domainsByMAC))
	for mac := range domainsByMAC {
		macs = append(macs, mac)
	}
	slices.SortFunc(macs, func(left, right [6]byte) int {
		return slices.Compare(left[:], right[:])
	})

	for _, mac := range macs {
		domains := sliceutil.SortAndCompact(domainsByMAC[mac], domain.CompareDomain)
		domainList := pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, domains, "(none)")
		description := hostid6.MAC(mac).Describe()
		switch {
		case mac == [6]byte{}:
			ppfmt.Noticef(pp.EmojiUserWarning,
				"hostid6=%s for %s uses the all-zero MAC address, which commonly represents an unset, placeholder, "+
					"deliberately configured, or broken identity",
				description, domainList)
		case mac == [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}:
			ppfmt.Noticef(pp.EmojiUserWarning,
				"hostid6=%s for %s uses the Ethernet broadcast destination and cannot identify one host",
				description, domainList)
		case mac[0]&1 != 0:
			ppfmt.Noticef(pp.EmojiUserWarning,
				"hostid6=%s for %s uses a group MAC address; the derived IPv6 address is still unicast, "+
					"but this MAC may not uniquely identify the intended host",
				description, domainList)
		}
	}
}

func normalizeDomains(ppfmt pp.PP, raw *RawConfig) (normalizedDomains, bool) {
	result := normalizedDomains{
		ByFamily: map[ipnet.Family][]domain.Domain{
			ipnet.IP4: projectDomains(raw.IP4Domains, raw.Domains),
			ipnet.IP6: projectDomains(raw.IP6Domains, raw.Domains),
		},
		HostID6: map[domain.Domain]hostid6.Set{},
	}

	opinions := map[domain.Domain]hostID6Opinion{}
	if !mergeHostID6Opinions(ppfmt, "DOMAINS", raw.Domains, opinions) ||
		!mergeHostID6Opinions(ppfmt, "IP6_DOMAINS", raw.IP6Domains, opinions) {
		return normalizedDomains{ByFamily: nil, HostID6: nil}, false
	}

	for _, dom := range result.ByFamily[ipnet.IP6] {
		if opinion, present := opinions[dom]; present {
			result.HostID6[dom] = opinion.set
		} else {
			result.HostID6[dom] = hostid6.DefaultSet()
		}
	}
	warnSuspiciousMACs(ppfmt, result.HostID6)
	return result, true
}
