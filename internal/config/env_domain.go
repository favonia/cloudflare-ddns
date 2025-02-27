package config

import (
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ReadDomains reads an environment variable as a comma-separated list of domains.
func ReadDomains(ppfmt pp.PP, key string, field *[]domain.Domain) bool {
	if list, ok := domainexp.ParseDomainList(ppfmt, key, Getenv(key)); ok {
		*field = list
		return true
	}
	return false
}

// ReadDomainHostIDs reads an environment variable as a comma-separated list of domains with optional IPv6 host IDs.
func ReadDomainHostIDs(ppfmt pp.PP, key string, field *[]domainexp.DomainHostID) bool {
	if list, ok := domainexp.ParseDomainHostIDList(ppfmt, key, Getenv(key)); ok {
		*field = list
		return true
	}
	return false
}

// deduplicate always sorts and deduplicates the input list,
// returning true if elements are already distinct.
func deduplicate(list []domain.Domain) []domain.Domain {
	domain.SortDomains(list)
	return slices.Compact(list)
}

func processDomainHostIDMap(ppfmt pp.PP,
	hostID map[domain.Domain]ipnet.HostID,
	domainHostIDs []domainexp.DomainHostID,
) ([]domain.Domain, bool) {
	domains := make([]domain.Domain, 0, len(domainHostIDs))
	for _, dh := range domainHostIDs {
		domains = append(domains, dh.Domain)

		if dh.HostID == nil {
			continue
		}

		if val, ok := hostID[dh.Domain]; ok && val != dh.HostID {
			ppfmt.Noticef(pp.EmojiUserError,
				"Domain %q is associated with different host IDs %s and %s",
				dh.Domain.Describe(), val.Describe(), dh.HostID.Describe(),
			)
			return nil, false
		}

		hostID[dh.Domain] = dh.HostID
	}
	return domains, true
}

// ReadDomainMap reads environment variables DOMAINS, IP4_DOMAINS, and IP6_DOMAINS
// and consolidate the domains into a map.
func ReadDomainMap(ppfmt pp.PP,
	fieldDomains *map[ipnet.Type][]domain.Domain,
	fieldHostID *map[domain.Domain]ipnet.HostID,
) bool {
	var (
		domains          []domain.Domain
		ip4Domains       []domain.Domain
		ip6DomainHostIDs []domainexp.DomainHostID
	)
	if !ReadDomains(ppfmt, "DOMAINS", &domains) ||
		!ReadDomains(ppfmt, "IP4_DOMAINS", &ip4Domains) ||
		!ReadDomainHostIDs(ppfmt, "IP6_DOMAINS", &ip6DomainHostIDs) {
		return false
	}

	hostID := map[domain.Domain]ipnet.HostID{}
	ip6Domains, ok := processDomainHostIDMap(ppfmt, hostID, ip6DomainHostIDs)
	if !ok {
		return false
	}

	ip4Domains = deduplicate(append(ip4Domains, domains...))
	ip6Domains = deduplicate(append(ip6Domains, domains...))

	*fieldDomains = map[ipnet.Type][]domain.Domain{
		ipnet.IP4: ip4Domains,
		ipnet.IP6: ip6Domains,
	}
	*fieldHostID = hostID

	return true
}
