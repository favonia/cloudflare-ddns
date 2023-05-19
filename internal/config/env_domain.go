package config

import (
	"golang.org/x/exp/slices"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// deduplicate always sorts and deduplicates the input list,
// returning true if elements are already distinct.
func deduplicate(list []domain.Domain) []domain.Domain {
	domain.SortDomains(list)
	return slices.Compact(list)
}

// ReadDomainMap reads environment variables DOMAINS, IP4_DOMAINS, and IP6_DOMAINS
// and consolidate the domains into a map.
func ReadDomainMap(ppfmt pp.PP, field *map[ipnet.Type][]domain.Domain) bool {
	var domains, ip4Domains, ip6Domains []domain.Domain

	if !ReadDomains(ppfmt, "DOMAINS", &domains) ||
		!ReadDomains(ppfmt, "IP4_DOMAINS", &ip4Domains) ||
		!ReadDomains(ppfmt, "IP6_DOMAINS", &ip6Domains) {
		return false
	}

	ip4Domains = deduplicate(append(ip4Domains, domains...))
	ip6Domains = deduplicate(append(ip6Domains, domains...))

	*field = map[ipnet.Type][]domain.Domain{
		ipnet.IP4: ip4Domains,
		ipnet.IP6: ip6Domains,
	}

	return true
}
