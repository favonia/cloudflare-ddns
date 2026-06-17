package config

import (
	"fmt"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainentry"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/sliceutil"
)

// normalizedDomains is the resolved view of all domain settings. HostID6 covers
// exactly the IPv6 domains in ByFamily[ipnet.IP6] and is always populated, using
// the default set for domains that carry no explicit hostid6 opinion.
// ExplicitHostID6 marks the subset of those domains whose hostid6 was set
// explicitly by the operator (as opposed to the implicit default).
type normalizedDomains struct {
	ByFamily        map[ipnet.Family][]domain.Domain
	HostID6         map[domain.Domain]hostid6.Set
	ExplicitHostID6 map[domain.Domain]bool
}

// hostID6Opinion is a host-ID set together with a human-readable provenance
// string (e.g. "IP6_DOMAINS declaration 2 hostid6 assignment 1") so that a later
// conflicting declaration can be reported against the one that set it first.
type hostID6Opinion struct {
	set    hostid6.Set
	source string
}

// validateStaticIP6HostIDCompatibility reports whether the host-ID policies are
// compatible with the IPv6 prefixes already known at configuration time (those a
// static provider exposes without a network query). Incompatibilities are user
// errors; it returns false after describing every one of them.
func validateStaticIP6HostIDCompatibility(
	ppfmt pp.PP,
	providerName string,
	domains []domain.Domain,
	policies map[domain.Domain]hostid6.Set,
	rawEntries []ipnet.RawEntry,
) bool {
	_, problems := hostid6.DeriveDomains(domains, policies, rawEntries)
	for _, problem := range problems {
		derivations := problem.Derivations.StringOrScalar()
		domains := pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, problem.Domains, "(none)")
		observed := pp.EnglishJoinMapOrEmptyLabel(ipnet.RawEntry.String, problem.Observed, "(none)")

		switch problem.Kind {
		case hostid6.LiteralPrefixTooLong, hostid6.MACPrefixTooLong:
			ppfmt.Noticef(pp.EmojiUserError,
				"IP6_PROVIDER=%s is incompatible with hostid6=%s for %s: requires prefixes no longer than /%d, "+
					"but includes %s; change the listed hostid6 setting or IP6_PROVIDER",
				providerName, derivations, domains, problem.PrefixLenBound, observed)
		case hostid6.MACPrefixTooShort:
			ppfmt.Noticef(pp.EmojiUserError,
				"IP6_PROVIDER=%s is incompatible with hostid6=%s for %s: requires a /64 prefix, "+
					"but includes %s; change the listed hostid6 setting or IP6_PROVIDER",
				providerName, derivations, domains, observed)
			hostid6.EmitMACShortPrefixHint(ppfmt, problem.Derivations)
		default:
			panic(fmt.Sprintf("invalid host-ID incompatibility kind %d", problem.Kind))
		}
	}
	return len(problems) == 0
}

// mergeHostID6Opinions folds one setting's entries into opinions, keyed by
// domain. The first opinion for a domain wins; a later one is accepted only if it
// is identical, otherwise the conflict is a user error. This lets a domain appear
// in both DOMAINS and IP6_DOMAINS as long as every declaration agrees.
func mergeHostID6Opinions(
	ppfmt pp.PP,
	setting string,
	entries []domainentry.Entry,
	opinions map[domain.Domain]hostID6Opinion,
) bool {
	for declarationIndex, entry := range entries {
		for assignmentIndex, set := range entry.HostID6Opinions {
			source := fmt.Sprintf("%s declaration %d hostid6 assignment %d", setting, declarationIndex+1, assignmentIndex+1)
			if set.IsZero() {
				ppfmt.Noticef(pp.EmojiImpossible,
					"%s for %s contains an empty host-ID set; this should not happen. Please report it at %s",
					source, entry.Domain.Describe(), pp.IssueReportingURL)
				return false
			}
			previous, present := opinions[entry.Domain]
			if present && !hostid6.EqualSet(previous.set, set) {
				ppfmt.Noticef(pp.EmojiUserError,
					"Conflicting hostid6 settings for %s: %s configures %s, while %s configures %s; "+
						"configure exactly the same hostid6 set in every declaration or omit it from partial declarations",
					entry.Domain.Describe(), previous.source, previous.set.String(), source, set.String())
				return false
			}
			if !present {
				opinions[entry.Domain] = hostID6Opinion{set: set, source: source}
			}
		}
	}
	return true
}

// projectDomains collects the domains from one or more settings into a single
// sorted, deduplicated list.
func projectDomains(entries ...[]domainentry.Entry) []domain.Domain {
	var domains []domain.Domain
	for _, settingEntries := range entries {
		for _, entry := range settingEntries {
			domains = append(domains, entry.Domain)
		}
	}
	return sliceutil.SortAndCompact(domains, domain.CompareDomain)
}

// warnSuspiciousMACs emits advisory warnings (never errors) for MAC-based host
// IDs that are unlikely to identify a single host: the all-zero address, the
// Ethernet broadcast address, and group (multicast-bit) addresses. Domains are
// grouped by MAC so each suspicious address is reported once, listing every
// domain that uses it, in a deterministic order.
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
		description := hostid6.MAC(mac).String()
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

// normalizeDomains resolves the raw domain settings into a normalizedDomains:
// it projects the per-family domain lists, merges the hostid6 opinions from
// DOMAINS and IP6_DOMAINS (reporting conflicts), assigns the default set to every
// IPv6 domain without an explicit opinion, and warns about suspicious MACs. It
// returns false if any opinion conflict makes the configuration invalid.
func normalizeDomains(ppfmt pp.PP, raw *RawConfig) (normalizedDomains, bool) {
	result := normalizedDomains{
		ByFamily: map[ipnet.Family][]domain.Domain{
			ipnet.IP4: projectDomains(raw.IP4Domains, raw.Domains),
			ipnet.IP6: projectDomains(raw.IP6Domains, raw.Domains),
		},
		HostID6:         map[domain.Domain]hostid6.Set{},
		ExplicitHostID6: map[domain.Domain]bool{},
	}

	opinions := map[domain.Domain]hostID6Opinion{}
	if !mergeHostID6Opinions(ppfmt, "DOMAINS", raw.Domains, opinions) ||
		!mergeHostID6Opinions(ppfmt, "IP6_DOMAINS", raw.IP6Domains, opinions) {
		return normalizedDomains{ByFamily: nil, HostID6: nil, ExplicitHostID6: nil}, false
	}

	for _, dom := range result.ByFamily[ipnet.IP6] {
		if opinion, present := opinions[dom]; present {
			result.HostID6[dom] = opinion.set
			result.ExplicitHostID6[dom] = true
		} else {
			result.HostID6[dom] = hostid6.DefaultSet()
		}
	}
	warnSuspiciousMACs(ppfmt, result.HostID6)
	return result, true
}
