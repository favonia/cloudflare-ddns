package config

import (
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/sliceutil"
)

// readDomains reads an environment variable as a comma-separated list of domains.
func readDomains(ppfmt pp.PP, key string, field *[]domain.Domain) bool {
	if list, ok := domainexp.ParseList(ppfmt, key, getenv(key)); ok {
		*field = list
		return true
	}
	return false
}

// readDomainMap reads environment variables DOMAINS, IP4_DOMAINS, and IP6_DOMAINS
// and consolidate the domains into a map.
func readDomainMap(ppfmt pp.PP, field *map[ipnet.Family][]domain.Domain) bool {
	var domains, ip4Domains, ip6Domains []domain.Domain

	if !readDomains(ppfmt, "DOMAINS", &domains) ||
		!readDomains(ppfmt, "IP4_DOMAINS", &ip4Domains) ||
		!readDomains(ppfmt, "IP6_DOMAINS", &ip6Domains) {
		return false
	}

	// DOMAINS applies to both families; merge, then sort/dedup for stable processing.
	ip4Domains = sliceutil.SortAndCompact(append(ip4Domains, domains...), domain.CompareDomain)
	ip6Domains = sliceutil.SortAndCompact(append(ip6Domains, domains...), domain.CompareDomain)

	*field = map[ipnet.Family][]domain.Domain{
		ipnet.IP4: ip4Domains,
		ipnet.IP6: ip6Domains,
	}

	return true
}
