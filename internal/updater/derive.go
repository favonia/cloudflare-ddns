package updater

import (
	"cmp"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

type dnsTargetsByDomain map[domain.Domain][]netip.Addr

type hostID6ProblemGroup struct {
	Kind         hostid6.IncompatibilityKind
	MaxPrefixLen int
	Domains      []domain.Domain
	Derivations  hostid6.Set
	Observed     []ipnet.RawEntry
}

type hostID6ProblemGroupKey struct {
	kind         hostid6.IncompatibilityKind
	maxPrefixLen int
}

type hostID6ProblemGroupBuilder struct {
	domains     map[domain.Domain]struct{}
	derivations []hostid6.Derivation
	observed    []ipnet.RawEntry
}

func deriveIP6DNSTargets(
	domains []domain.Domain,
	policies map[domain.Domain]hostid6.Set,
	rawData provider.DetectionResult,
) (dnsTargetsByDomain, []hostID6ProblemGroup) {
	if !rawData.Available {
		panic("deriveIP6DNSTargets received unavailable raw data; this should not happen; please report it")
	}
	for _, configuredDomain := range domains {
		if policies[configuredDomain].IsZero() {
			panic("deriveIP6DNSTargets received an empty host-ID policy; this should not happen; please report it")
		}
	}

	targets := make(dnsTargetsByDomain, len(domains))
	problemBuilders := map[hostID6ProblemGroupKey]*hostID6ProblemGroupBuilder{}
	for _, configuredDomain := range domains {
		domainTargets := make([]netip.Addr, 0, len(rawData.RawEntries)*policies[configuredDomain].Len())
		for raw := range slices.Values(rawData.RawEntries) {
			for derivation := range policies[configuredDomain].All() {
				target, problem := hostid6.Derive(raw, derivation)
				if problem == nil {
					domainTargets = append(domainTargets, target)
					continue
				}

				key := hostID6ProblemGroupKey{kind: problem.Kind, maxPrefixLen: problem.MaxPrefixLen}
				builder := problemBuilders[key]
				if builder == nil {
					builder = &hostID6ProblemGroupBuilder{
						domains:     map[domain.Domain]struct{}{},
						derivations: nil,
						observed:    nil,
					}
					problemBuilders[key] = builder
				}
				builder.domains[configuredDomain] = struct{}{}
				builder.derivations = append(builder.derivations, problem.Derivation)
				builder.observed = append(builder.observed, problem.ObservedPrefix)
			}
		}

		slices.SortFunc(domainTargets, netip.Addr.Compare)
		targets[configuredDomain] = slices.Compact(domainTargets)
	}

	if len(problemBuilders) == 0 {
		return targets, nil
	}

	problems := make([]hostID6ProblemGroup, 0, len(problemBuilders))
	for key, builder := range problemBuilders {
		groupDomains := make([]domain.Domain, 0, len(builder.domains))
		for configuredDomain := range builder.domains {
			groupDomains = append(groupDomains, configuredDomain)
		}
		domain.SortDomains(groupDomains)

		slices.SortFunc(builder.observed, ipnet.RawEntry.Compare)
		problems = append(problems, hostID6ProblemGroup{
			Kind:         key.kind,
			MaxPrefixLen: key.maxPrefixLen,
			Domains:      groupDomains,
			Derivations:  hostid6.NewSet(builder.derivations...),
			Observed:     slices.Compact(builder.observed),
		})
	}
	slices.SortFunc(problems, func(left, right hostID6ProblemGroup) int {
		if order := cmp.Compare(left.Kind, right.Kind); order != 0 {
			return order
		}
		return cmp.Compare(left.MaxPrefixLen, right.MaxPrefixLen)
	})

	return nil, problems
}
