package config

import (
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ReadDomains reads an environment variable as a comma-separated list of domains.
func ReadDomains(ppfmt pp.PP, key string, field *[]domain.Domain) bool {
	if list, ok := domainexp.ParseList(ppfmt, key, Getenv(key)); ok {
		*field = list
		return true
	}
	return false
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

	ip4Domains = sortAndCompact(append(ip4Domains, domains...), domain.CompareDomain)
	ip6Domains = sortAndCompact(append(ip6Domains, domains...), domain.CompareDomain)

	*field = map[ipnet.Type][]domain.Domain{
		ipnet.IP4: ip4Domains,
		ipnet.IP6: ip6Domains,
	}

	return true
}
