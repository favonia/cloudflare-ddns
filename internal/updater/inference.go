package updater

import (
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// recordFactor chooses exactly one address, or absence when absent is true.
// An empty id is a symbolic create whose identity has not been established.
type recordFactor struct {
	id        api.ID
	addresses []netip.Addr
	absent    bool
}

// compatibleFactors reports whether independent factor choices can produce
// exactly the observed address set. Multiplicity of observed addresses is ignored.
func compatibleFactors(factors []recordFactor, observed []netip.Addr) bool {
	addresses := make([]netip.Addr, 0, len(observed))
	for _, ip := range observed {
		if !slices.Contains(addresses, ip) {
			addresses = append(addresses, ip)
		}
	}
	for _, f := range factors {
		if !f.absent {
			intersects := false
			for _, ip := range addresses {
				if slices.Contains(f.addresses, ip) {
					intersects = true
					break
				}
			}
			if !intersects {
				return false
			}
		}
	}
	// Match each distinct observed address to a distinct producing factor.
	matched := make([]int, len(factors))
	for i := range matched {
		matched[i] = -1
	}
	var augment func(int, []bool) bool
	augment = func(address int, seen []bool) bool {
		for i, f := range factors {
			if seen[i] || !slices.Contains(f.addresses, addresses[address]) {
				continue
			}
			seen[i] = true
			if matched[i] < 0 || augment(matched[i], seen) {
				matched[i] = address
				return true
			}
		}
		return false
	}
	for i := range addresses {
		if !augment(i, make([]bool, len(factors))) {
			return false
		}
	}
	return true
}

// dnsInference tracks one domain/family scope. The zero value has no baseline.
// Unclassifiable identity or budget overflow widens only this scope until its
// next coherent fresh read. No mutation history survives that read.
type dnsInference struct {
	known   bool
	factors []recordFactor
}

// These finite per-scope budgets are engineering policy, not measured performance.
const (
	inferenceFactorBudget = 256
	inferenceEdgeBudget   = 4096
)

// assess compares a fresh coherent Before against possible own outcomes, then
// replaces the baseline and applies reached effects in order. True means an
// unexplained address observation, never work failure. Cached/missing reads do
// not compare or refine. Call serially once per completed scope operation.
func (m *dnsInference) assess(result setter.DNSResult) bool {
	unexplained := false
	if before := result.Before; before != nil && !before.Cached {
		factors, observed, valid := inferenceSnapshot(before.Records)
		if valid {
			unexplained = m.known && !compatibleFactors(m.factors, observed)
			m.known = true
			m.factors = factors
			m.bound()
		} else {
			m.widen()
		}
	}
	for _, effect := range result.Effects {
		m.apply(effect)
	}
	return unexplained
}

// inferenceSnapshot normalizes repeated identical identities. Distinct IDs
// sharing an address remain separate factors. Missing/conflicting identities
// cannot establish an exact baseline under the API's list admission contract.
func inferenceSnapshot(records []api.RecordFact) ([]recordFactor, []netip.Addr, bool) {
	factors := make([]recordFactor, 0, min(len(records), inferenceFactorBudget))
	observed := make([]netip.Addr, 0, len(records))
	identities := make(map[api.ID]netip.Addr, len(records))
	for _, r := range records {
		if r.ID == "" || !r.IP.IsValid() {
			return nil, nil, false
		}
		if prior, exists := identities[r.ID]; exists {
			if prior != r.IP {
				return nil, nil, false
			}
			continue
		}
		identities[r.ID] = r.IP
		observed = append(observed, r.IP)
		factors = append(factors, recordFactor{id: r.ID, addresses: []netip.Addr{r.IP}, absent: false})
	}
	return factors, observed, true
}

func (m *dnsInference) widen() { m.known = false; m.factors = nil }

func (m *dnsInference) bound() {
	edges := 0
	for _, f := range m.factors {
		edges += len(f.addresses)
		if f.absent {
			edges++
		}
	}
	if len(m.factors) > inferenceFactorBudget || edges > inferenceEdgeBudget {
		m.widen()
	}
}

// apply uses only supported identities. An unknown association may refer to a
// symbolic create or contradict the baseline; widening avoids excluding an own
// outcome. Creates with no established ID get independent symbolic factors.
func (m *dnsInference) apply(effect api.RecordMutation) {
	if !m.known || effect.Status == api.MutationNoEffect {
		return
	}
	if effect.Status != api.MutationCompleted && effect.Status != api.MutationUnconfirmed {
		m.widen()
		return
	}
	index := -1
	if effect.Record.ID != "" {
		for i, f := range m.factors {
			if f.id == effect.Record.ID {
				index = i
				break
			}
		}
	}
	if effect.Kind == api.RecordCreate {
		// A returned identity colliding with an existing factor does not support an
		// independent new record; the producer does not promise collision rejection.
		if index >= 0 {
			m.widen()
			return
		}
	} else if effect.Record.ID == "" || index < 0 {
		m.widen()
		return
	}

	next := recordFactor{id: effect.Record.ID, addresses: nil, absent: false}
	switch effect.Kind {
	case api.RecordDelete:
		next.absent = true
	case api.RecordCreate, api.RecordUpdate:
		if !effect.Record.IP.IsValid() {
			m.widen()
			return
		}
		switch effect.Membership {
		case api.MembershipManaged:
			next.addresses = []netip.Addr{effect.Record.IP}
		case api.MembershipUnmanaged:
			next.absent = true
		case api.MembershipUnknown:
			next.addresses = []netip.Addr{effect.Record.IP}
			next.absent = true
		default:
			m.widen()
			return
		}
	default:
		m.widen()
		return
	}
	if effect.Status == api.MutationUnconfirmed {
		if index < 0 {
			next.absent = true
		} else {
			prior := m.factors[index]
			next.absent = next.absent || prior.absent
			addresses := slices.Clone(prior.addresses)
			for _, ip := range next.addresses {
				if !slices.Contains(addresses, ip) {
					addresses = append(addresses, ip)
				}
			}
			next.addresses = addresses
		}
	}
	if index < 0 {
		m.factors = append(m.factors, next)
	} else {
		m.factors[index] = next
	}
	m.bound()
}
