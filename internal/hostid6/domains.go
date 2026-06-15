package hostid6

import (
	"cmp"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// TargetsByDomain maps each managed IPv6 DNS domain to its derived target set.
type TargetsByDomain map[domain.Domain][]netip.Addr

// ProblemGroup groups incompatible derivations with the same corrective bound.
type ProblemGroup struct {
	Kind         IncompatibilityKind
	MaxPrefixLen int
	Domains      []domain.Domain
	Derivations  Set
	Observed     []ipnet.RawEntry
}

type problemGroupKey struct {
	kind         IncompatibilityKind
	maxPrefixLen int
}

type problemGroupBuilder struct {
	domains     map[domain.Domain]struct{}
	derivations []Derivation
	observed    []ipnet.RawEntry
}

// DeriveDomains derives per-domain targets and groups all incompatibilities.
// Raw entries must be valid IPv6 entries, and each domain must have a non-empty policy.
func DeriveDomains(
	domains []domain.Domain,
	policies map[domain.Domain]Set,
	rawEntries []ipnet.RawEntry,
) (TargetsByDomain, []ProblemGroup) {
	for _, configuredDomain := range domains {
		if policies[configuredDomain].IsZero() {
			panic("hostid6.DeriveDomains received an empty host-ID policy; this should not happen; please report it")
		}
	}

	targets := make(TargetsByDomain, len(domains))
	problemBuilders := map[problemGroupKey]*problemGroupBuilder{}
	for _, configuredDomain := range domains {
		domainTargets := make([]netip.Addr, 0, len(rawEntries)*policies[configuredDomain].Len())
		for raw := range slices.Values(rawEntries) {
			for derivation := range policies[configuredDomain].All() {
				target, problem := Derive(raw, derivation)
				if problem == nil {
					domainTargets = append(domainTargets, target)
					continue
				}

				key := problemGroupKey{kind: problem.Kind, maxPrefixLen: problem.MaxPrefixLen}
				builder := problemBuilders[key]
				if builder == nil {
					builder = &problemGroupBuilder{
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

	problems := make([]ProblemGroup, 0, len(problemBuilders))
	for key, builder := range problemBuilders {
		groupDomains := make([]domain.Domain, 0, len(builder.domains))
		for configuredDomain := range builder.domains {
			groupDomains = append(groupDomains, configuredDomain)
		}
		domain.SortDomains(groupDomains)

		slices.SortFunc(builder.observed, ipnet.RawEntry.Compare)
		problems = append(problems, ProblemGroup{
			Kind:         key.kind,
			MaxPrefixLen: key.maxPrefixLen,
			Domains:      groupDomains,
			Derivations:  NewSet(builder.derivations...),
			Observed:     slices.Compact(builder.observed),
		})
	}
	slices.SortFunc(problems, func(left, right ProblemGroup) int {
		if order := cmp.Compare(left.Kind, right.Kind); order != 0 {
			return order
		}
		return cmp.Compare(left.MaxPrefixLen, right.MaxPrefixLen)
	})

	// Deliberately discard all derived targets when any derivation is incompatible:
	// callers preserve every existing IPv6 record and WAF item for the whole update
	// rather than apply a partial set, so returning the survivors would be misleading.
	return nil, problems
}
