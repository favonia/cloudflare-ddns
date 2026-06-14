package updater

import (
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

type (
	dnsTargetsByDomain  = hostid6.TargetsByDomain
	hostID6ProblemGroup = hostid6.ProblemGroup
)

func deriveIP6DNSTargets(
	domains []domain.Domain,
	policies map[domain.Domain]hostid6.Set,
	rawData provider.DetectionResult,
) (dnsTargetsByDomain, []hostID6ProblemGroup) {
	if !rawData.Available {
		panic("deriveIP6DNSTargets received unavailable raw data; this should not happen; please report it")
	}
	return hostid6.DeriveDomains(domains, policies, rawData.RawEntries)
}
