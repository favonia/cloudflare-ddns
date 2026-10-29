package setter

import (
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
)

// DNSSnapshot owns record facts copied from a successful managed-record listing.
// Cached records may be borrowed by the API; this copy survives cache mutation.
type DNSSnapshot struct {
	Cached  bool
	Records []api.RecordFact
}

// DNSOperationStatus distinguishes work not reached from a reached operation's outcome.
type DNSOperationStatus uint8

const (
	// DNSOperationNotAttempted means reconciliation returned before reaching this required operation.
	DNSOperationNotAttempted DNSOperationStatus = iota
	// DNSOperationBlocked means the operation was reached but no mutating request was sent.
	DNSOperationBlocked
	// DNSOperationUnconfirmed means the request may or may not have completed.
	DNSOperationUnconfirmed
	// DNSOperationCompleted means the mutation completed successfully.
	DNSOperationCompleted
)

// DNSOperation describes one required operation in reconciliation order.
// Create/update IP is the requested target; update ID identifies the reused record.
// Create ID is empty until a completed API result establishes it. Delete ID and IP
// identify the exact pre-operation record, including duplicate-address records.
// Membership comes only from reached mutation evidence; unattempted and blocked
// work have MembershipUnknown. Completed does not imply managed membership.
type DNSOperation struct {
	Kind       api.RecordMutationKind
	Record     api.RecordFact
	Status     DNSOperationStatus
	Membership api.ManagedMembership
}

// DNSProgress owns the initial read's already-covered targets and required work.
// Satisfied follows target input order and is historical evidence, with cache
// provenance in DNSResult.Before.Cached, rather than a fresh read-back.
// Operations includes the unvisited suffix in reconciliation order. Empty slices
// mean no satisfied targets or required operations; their nilness is immaterial.
type DNSProgress struct {
	Satisfied  []netip.Addr
	Operations []DNSOperation
}

// DNSResult owns its snapshot, progress and effects. Before and Progress are nil
// exactly when the initial read failed; non-nil empty values are known-empty.
// Effects preserves reached operation order and includes only completed or
// unconfirmed mutations, never planned writes or prerequisite failures. Progress
// separately records required work, including blocked and unvisited operations
// that Effects cannot describe. Code retains reconciliation policy.
type DNSResult struct {
	Code     ResponseCode
	Before   *DNSSnapshot
	Progress *DNSProgress
	Effects  []api.RecordMutation
}

func snapshotRecords(rs []api.Record, cached bool) *DNSSnapshot {
	facts := make([]api.RecordFact, len(rs))
	for i, r := range rs {
		facts[i] = api.RecordFact{ID: r.ID, IP: r.IP}
	}
	return &DNSSnapshot{Cached: cached, Records: facts}
}

// admit records reached evidence without changing the plan's requested target or
// deletion address. Effects retains the API shape, in particular addressless deletes.
func (r *DNSResult) admit(index int, effect api.RecordMutation) bool {
	operation := &r.Progress.Operations[index]
	switch effect.Status {
	case api.MutationNoEffect:
		operation.Status = DNSOperationBlocked
	case api.MutationUnconfirmed:
		operation.Status = DNSOperationUnconfirmed
	case api.MutationCompleted:
		operation.Status = DNSOperationCompleted
		if operation.Kind == api.RecordCreate {
			operation.Record.ID = effect.Record.ID
		}
	}
	operation.Membership = effect.Membership
	if effect.Status != api.MutationNoEffect {
		r.Effects = append(r.Effects, effect)
	}
	return effect.Status == api.MutationCompleted
}
