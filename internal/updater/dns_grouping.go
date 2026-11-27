package updater

import (
	"cmp"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// dnsDisplayOperation is an identity-free display qualification computed within
// one original scope. Equivalent IDs may group; different pairing/count promises
// may not. Visibility-dependent Other remains the renderer's per-scope decision.
type dnsDisplayOperation struct {
	kind       api.RecordMutationKind
	status     setter.DNSOperationStatus
	membership api.ManagedMembership
	address    netip.Addr
	old        netip.Addr
	paired     bool
	count      int
	countKnown bool
	omitOne    bool
}

func (e dnsEvidence) displayOperations() []dnsDisplayOperation {
	var display []dnsDisplayOperation
	for _, item := range e.operations {
		op := item.operation
		fact := dnsDisplayOperation{
			kind: op.Kind, status: op.Status, membership: op.Membership, address: op.Record.IP,
			old: item.old.IP, paired: item.paired, count: 0, countKnown: false, omitOne: false,
		}
		if !item.paired {
			fact.old = netip.Addr{}
		}
		if op.Kind == api.RecordDelete {
			var indices []int
			for j, other := range e.operations {
				if other.operation.Kind == op.Kind && other.operation.Status == op.Status &&
					other.operation.Record.IP == op.Record.IP {
					indices = append(indices, j)
				}
			}
			count := e.removalFacts(indices, nil)
			fact.count = count.count
			fact.countKnown = count.countKnown
			fact.omitOne = count.omitOne
		}
		// Repeated identical evidence does not create another visible operation.
		if !slices.Contains(display, fact) {
			display = append(display, fact)
		}
	}
	slices.SortFunc(display, func(a, b dnsDisplayOperation) int {
		if result := cmp.Compare(a.status, b.status); result != 0 {
			return result
		}
		if result := cmp.Compare(a.kind, b.kind); result != 0 {
			return result
		}
		if result := a.address.Compare(b.address); result != 0 {
			return result
		}
		return a.old.Compare(b.old)
	})
	return display
}

func (r dnsRow) sameFacts(other dnsRow) bool {
	if r.family != other.family || r.cleanup != other.cleanup || r.code != other.code ||
		r.beforeKnown != other.beforeKnown || r.beforeCached != other.beforeCached ||
		r.filtered != other.filtered || r.unexplained != other.unexplained ||
		r.evidence.deltaComplete != other.evidence.deltaComplete ||
		!slices.Equal(r.before, other.before) || !slices.Equal(r.target, other.target) ||
		!slices.Equal(r.satisfied, other.satisfied) {
		return false
	}
	if r.code == setter.ResponseNoop || r.code == setter.ResponseUpdated && r.evidence.deltaComplete {
		return true
	}
	return slices.Equal(r.evidence.displayOperations(), other.evidence.displayOperations())
}
