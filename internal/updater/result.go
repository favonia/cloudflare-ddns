package updater

import (
	"maps"
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
	filtered bool
	scope    dnsScope
	cleanup  bool
	target   []netip.Addr
	result   setter.DNSResult
}

// wafScope records managed families by key presence and operation selection by
// value. A false value retains owned inventory while excluding that family from
// this operation's delta. Selection does not promise successful completion.
// A missing key is unmanaged, including when its inventory would be empty.
type wafScope map[ipnet.Family]bool

func (s wafScope) manages(family ipnet.Family) bool {
	_, present := s[family]
	return present
}

func (s wafScope) selects(family ipnet.Family) bool { return s[family] }

// wafOutcome owns admitted evidence and scope. cleanup records invocation intent,
// while result.Cleanup carries admitted cleanup evidence; neither is inferred
// from the work code. Production cleanup selects every managed family; consumers
// also preserve scope distinctions when supplied a partial cleanup selection.
type wafOutcome struct {
	result   setter.WAFResult
	filtered bool
	scope    wafScope
	list     api.WAFList
	cleanup  bool
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
	if o.result.Progress != nil {
		o.result.Progress = &setter.DNSProgress{
			Satisfied:  slices.Clone(o.result.Progress.Satisfied),
			Operations: slices.Clone(o.result.Progress.Operations),
		}
	}
	o.result.Effects = slices.Clone(o.result.Effects)
	return o
}

// clone detaches all nested producer evidence at each retention boundary.
func (o wafOutcome) clone() wafOutcome {
	o.scope = maps.Clone(o.scope)
	o.result.Before = cloneWAFSnapshot(o.result.Before)
	o.result.Batches = slices.Clone(o.result.Batches)
	for i := range o.result.Batches {
		b := &o.result.Batches[i]
		b.Create = slices.Clone(b.Create)
		b.Delete = slices.Clone(b.Delete)
		if b.Result != nil {
			result := cloneWAFBatch(*b.Result)
			b.Result = &result
		}
	}
	if o.result.Cleanup != nil {
		cleanup := *o.result.Cleanup
		cleanup.Before = cloneWAFSnapshot(cleanup.Before)
		cleanup.Stages = slices.Clone(cleanup.Stages)
		for i := range cleanup.Stages {
			stage := &cleanup.Stages[i]
			stage.Items = slices.Clone(stage.Items)
			stage.Result = cloneWAFBatch(stage.Result)
		}
		o.result.Cleanup = &cleanup
	}
	return o
}

func cloneWAFSnapshot(snapshot *api.WAFSnapshot) *api.WAFSnapshot {
	if snapshot == nil {
		return nil
	}
	result := *snapshot
	result.Items = slices.Clone(snapshot.Items)
	return &result
}

func cloneWAFBatch(result api.WAFBatchResult) api.WAFBatchResult {
	result.Returned = cloneWAFSnapshot(result.Returned)
	result.ReturnedItems = slices.Clone(result.ReturnedItems)
	return result
}
