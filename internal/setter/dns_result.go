package setter

import "github.com/favonia/cloudflare-ddns/internal/api"

// DNSSnapshot owns record facts copied from a successful managed-record listing.
// Cached records may be borrowed by the API; this copy survives cache mutation.
type DNSSnapshot struct {
	Cached  bool
	Records []api.RecordFact
}

// DNSResult owns its snapshot and effects. Before is nil only when the initial
// read failed; a non-nil empty snapshot is known-empty. Effects preserves reached
// operation order and includes only completed or unconfirmed mutations, never
// planned writes or prerequisite failures. Code retains reconciliation policy.
type DNSResult struct {
	Code    ResponseCode
	Before  *DNSSnapshot
	Effects []api.RecordMutation
}

func snapshotRecords(rs []api.Record, cached bool) *DNSSnapshot {
	facts := make([]api.RecordFact, len(rs))
	for i, r := range rs {
		facts[i] = api.RecordFact{ID: r.ID, IP: r.IP}
	}
	return &DNSSnapshot{Cached: cached, Records: facts}
}

func (r *DNSResult) admit(effect api.RecordMutation) bool {
	if effect.Status != api.MutationNoEffect {
		r.Effects = append(r.Effects, effect)
	}
	return effect.Status == api.MutationCompleted
}
