package updater

import (
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type dnsScope struct {
	domain domain.Domain
	family ipnet.Family
}

// dnsOutcome retains one invoked scope, including no-ops. A steady-state target
// is known even when empty; cleanup has no target. Before retains the setter's
// distinction between an unknown snapshot and a known empty set.
type dnsOutcome struct {
	scope   dnsScope
	cleanup bool
	target  []netip.Addr
	result  setter.DNSResult
}

type wafOutcome struct {
	list    api.WAFList
	code    setter.ResponseCode
	cleanup bool
}

type operationProblemKind uint8

const (
	problemDetection operationProblemKind = iota
	problemFiltering
	problemDerivation
	problemMissingTarget
)

// operationProblem describes work aborted before setter invocation. Family-wide
// detection/filter failures omit domains; atomic derivation aborts name every
// affected configured domain, and missing targets name only absent mappings.
type operationProblem struct {
	kind    operationProblemKind
	family  ipnet.Family
	domains []domain.Domain
}

// clone owns every mutable value retained across message composition.
func (o dnsOutcome) clone() dnsOutcome {
	o.target = slices.Clone(o.target)
	if o.result.Before != nil {
		o.result.Before = &setter.DNSSnapshot{
			Cached:  o.result.Before.Cached,
			Records: slices.Clone(o.result.Before.Records),
		}
	}
	o.result.Effects = slices.Clone(o.result.Effects)
	return o
}
