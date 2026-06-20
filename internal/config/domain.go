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

// hostID6Provenance remembers where a host-ID set came from, but only at the
// level needed for operator-facing conflict messages: same entry, same setting,
// or different settings.
type hostID6Provenance struct {
	// set is the normalized value used for conflict equality.
	set hostid6.Set
	// sourceSnippet is the original hostid6=... assignment text used in diagnostics.
	sourceSnippet string
	// setting is the setting containing the domain entry, such as DOMAINS or IP6_DOMAINS.
	setting string
	// entryIndex is the zero-based index of the domainentry.Entry within setting.
	entryIndex int
}

func hostID6Snippet(set hostid6.Set) string {
	return fmt.Sprintf("hostid6=%s", set.ConfigString())
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
		derivations := problem.Derivations.ConfigString()
		domains := pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, problem.Domains, "(none)")
		observed := pp.EnglishJoinMapOrEmptyLabel(ipnet.RawEntry.String, problem.Observed, "(none)")

		switch problem.Kind {
		case hostid6.LiteralPrefixTooLong, hostid6.MACPrefixTooLong:
			ppfmt.Noticef(pp.EmojiUserError,
				"IP6_PROVIDER=%s cannot be used for %s with hostid6=%s: it requires prefixes no longer than /%d, "+
					"but the provider includes %s; change IP6_PROVIDER or that hostid6 setting",
				providerName, domains, derivations, problem.PrefixLenBound, observed)
		case hostid6.MACPrefixTooShort:
			ppfmt.Noticef(pp.EmojiUserError,
				"IP6_PROVIDER=%s cannot be used for %s with hostid6=%s: it requires a /64 prefix, "+
					"but the provider includes %s; change IP6_PROVIDER or that hostid6 setting",
				providerName, domains, derivations, observed)
			if len(problem.Observed) > 0 {
				hostid6.EmitMACShortPrefixHint(ppfmt, problem.Derivations, problem.Observed[0].Prefix())
			}
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
	opinions map[domain.Domain]hostID6Provenance,
) bool {
	for entryIndex, entry := range entries {
		for _, opinion := range entry.HostID6Opinions {
			set := opinion.Set
			if set.IsZero() {
				ppfmt.Noticef(pp.EmojiImpossible,
					"An internal error produced an empty hostid6 set for %s in %s; this should not happen. Please report it at %s",
					entry.Domain.Describe(), setting, pp.IssueReportingURL)
				return false
			}
			snippet := hostID6Snippet(set)
			if opinion.SourceSnippet != "" {
				snippet = opinion.SourceSnippet
			}
			previous, present := opinions[entry.Domain]
			if present && !hostid6.EqualSet(previous.set, set) {
				switch {
				case previous.setting == setting && previous.entryIndex == entryIndex:
					ppfmt.Noticef(pp.EmojiUserError,
						`Conflicting hostid6 settings for %s: `+
							`the same %s entry has "%s" and "%s"; `+
							`use only one hostid6 assignment, or make the assignments identical`,
						entry.Domain.Describe(), setting, previous.sourceSnippet, snippet)
				case previous.setting == setting:
					ppfmt.Noticef(pp.EmojiUserError,
						`Conflicting hostid6 settings for %s: %s has "%s" and also "%s"; `+
							`use the same hostid6 set everywhere %s configures hostid6, `+
							`or remove the extra hostid6 assignment`,
						entry.Domain.Describe(), setting, previous.sourceSnippet, snippet, entry.Domain.Describe())
				default:
					ppfmt.Noticef(pp.EmojiUserError,
						`Conflicting hostid6 settings for %s: %s has "%s", but %s has "%s"; `+
							`use the same hostid6 set everywhere %s configures hostid6, `+
							`or remove the extra hostid6 assignment`,
						entry.Domain.Describe(), previous.setting, previous.sourceSnippet, setting, snippet, entry.Domain.Describe())
				}
				return false
			}
			if !present {
				opinions[entry.Domain] = hostID6Provenance{
					set:           set,
					sourceSnippet: snippet,
					setting:       setting,
					entryIndex:    entryIndex,
				}
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
				"hostid6=%s for %s uses the all-zero MAC address; check whether this is the MAC address you intended to use",
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

// warnShadowedFamilyIntents warns when a family-specific configuration intent is
// rendered inert by that family's provider being disabled, even if the domain
// survives via the other family. The intents are: a domain's membership declared
// through IP4_DOMAINS or IP6_DOMAINS, and an explicit hostid6 setting (IPv6 only).
// A bare DOMAINS membership is family-agnostic and never warns: running
// single-stack is the normal narrowing of a broad selector, not an inert intent.
//
// Warnings are grouped (like warnSuspiciousMACs): at most one membership warning
// per disabled family listing every shadowed domain, and one hostid6 warning per
// distinct host-ID set value listing every domain that carries it. This family
// intent set is small and operator-authored, so grouping is the right shape (see
// docs/designs/guides/operator-messages.markdown).
//
// These per-intent warnings replace the older whole-domain "only for X but X is
// disabled" warning: a domain listed only under a disabled family is covered by
// its membership warning, so there is no double-warn. To avoid two warnings for a
// domain listed in IP6_DOMAINS that also carries hostid6, the hostid6 warning is
// emitted only for domains whose hostid6 came through DOMAINS (i.e. not also
// listed in IP6_DOMAINS, whose membership warning already subsumes it).
func warnShadowedFamilyIntents(
	ppfmt pp.PP,
	ip4Managed, ip6Managed bool,
	normalized normalizedDomains,
	raw *RawConfig,
) {
	managed := map[ipnet.Family]bool{ipnet.IP4: ip4Managed, ipnet.IP6: ip6Managed}
	specific := map[ipnet.Family][]domainentry.Entry{
		ipnet.IP4: raw.IP4Domains,
		ipnet.IP6: raw.IP6Domains,
	}
	settingName := map[ipnet.Family]string{ipnet.IP4: "IP4_DOMAINS", ipnet.IP6: "IP6_DOMAINS"}

	// Membership intents: one grouped warning per disabled family.
	for _, family := range []ipnet.Family{ipnet.IP4, ipnet.IP6} {
		if managed[family] {
			continue
		}
		domains := projectDomains(specific[family])
		if len(domains) == 0 {
			continue
		}
		ppfmt.Noticef(pp.EmojiUserWarning,
			"The %s listing of %s is ignored because %s is disabled",
			settingName[family],
			pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, domains, "(none)"),
			family.Describe())
	}

	// Explicit hostid6 intents (IPv6 only), grouped by set value. Domains already
	// covered by the IP6_DOMAINS membership warning above are excluded.
	if !ip6Managed {
		ip6Listed := map[domain.Domain]bool{}
		for _, dom := range projectDomains(specific[ipnet.IP6]) {
			ip6Listed[dom] = true
		}

		// ConfigString() is the canonical hostid6 value rendering, so it is a safe
		// grouping key: distinct sets never collide, and sorting the keys gives a
		// deterministic order (mirroring how warnSuspiciousMACs sorts MACs).
		domainsBySet := map[string][]domain.Domain{}
		for dom := range normalized.ExplicitHostID6 {
			if ip6Listed[dom] {
				continue
			}
			key := normalized.HostID6[dom].ConfigString()
			domainsBySet[key] = append(domainsBySet[key], dom)
		}

		keys := make([]string, 0, len(domainsBySet))
		for key := range domainsBySet {
			keys = append(keys, key)
		}
		slices.Sort(keys)

		for _, key := range keys {
			domains := sliceutil.SortAndCompact(domainsBySet[key], domain.CompareDomain)
			ppfmt.Noticef(pp.EmojiUserWarning,
				"hostid6=%s for %s is ignored because IPv6 is disabled",
				key,
				pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, domains, "(none)"))
		}
	}
}

// normalizeDomains resolves the raw domain settings into a normalizedDomains:
// it projects the per-family domain lists, merges the hostid6 opinions from
// DOMAINS and IP6_DOMAINS (reporting conflicts), assigns the default set to every
// IPv6 domain without an explicit opinion. It returns false if any opinion
// conflict makes the configuration invalid.
func normalizeDomains(ppfmt pp.PP, raw *RawConfig) (normalizedDomains, bool) {
	result := normalizedDomains{
		ByFamily: map[ipnet.Family][]domain.Domain{
			ipnet.IP4: projectDomains(raw.IP4Domains, raw.Domains),
			ipnet.IP6: projectDomains(raw.IP6Domains, raw.Domains),
		},
		HostID6:         map[domain.Domain]hostid6.Set{},
		ExplicitHostID6: map[domain.Domain]bool{},
	}

	opinions := map[domain.Domain]hostID6Provenance{}
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
	return result, true
}
