package updater

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func inferenceAddresses() [3]netip.Addr {
	return [3]netip.Addr{netip.AddrFrom4([4]byte{192, 0, 2, 1}), netip.AddrFrom4([4]byte{192, 0, 2, 2}), netip.AddrFrom4([4]byte{192, 0, 2, 3})}
}

// Enumerate full assignments independently of the matching algorithm. All
// nonempty subsets of A/B/C/absence occur, with zero through four factors.
func TestCompatibleFactorsEnumeration(t *testing.T) {
	t.Parallel()
	for count := range 5 {
		models := 1
		for range count {
			models *= 15
		}
		for model := range models {
			factors := make([]recordFactor, count)
			masks := make([]int, count)
			n := model
			for i := range factors {
				masks[i] = n%15 + 1
				n /= 15
				factors[i].absent = masks[i]&8 != 0
				for j, ip := range inferenceAddresses() {
					if masks[i]&(1<<j) != 0 {
						factors[i].addresses = append(factors[i].addresses, ip)
					}
				}
			}
			possible := [8]bool{}
			var enumerate func(int, int)
			enumerate = func(i, set int) {
				if i == count {
					possible[set] = true
					return
				}
				for choice := range 4 {
					if masks[i]&(1<<choice) != 0 {
						next := set
						if choice < 3 {
							next |= 1 << choice
						}
						enumerate(i+1, next)
					}
				}
			}
			enumerate(0, 0)
			for set, want := range possible {
				var observed []netip.Addr
				for i, ip := range inferenceAddresses() {
					if set&(1<<i) != 0 {
						observed = append(observed, ip)
					}
				}
				if got := compatibleFactors(factors, observed); got != want {
					t.Fatalf("masks=%v observed=%v got %v want %v", masks, observed, got, want)
				}
			}
		}
	}
}

func inferenceRead(ids string, addresses ...int) setter.DNSResult {
	records := make([]api.RecordFact, len(addresses))
	for i, a := range addresses {
		records[i] = api.RecordFact{ID: api.ID(string(ids[i])), IP: inferenceAddresses()[a]}
	}
	return setter.DNSResult{Code: setter.ResponseNoop, Effects: nil, Before: &setter.DNSSnapshot{Cached: false, Records: records}}
}

func inferenceEffect(status api.MutationStatus, kind api.RecordMutationKind, id string, address int, membership api.ManagedMembership) setter.DNSResult {
	return setter.DNSResult{Code: setter.ResponseNoop, Before: nil, Effects: []api.RecordMutation{{Status: status, Kind: kind, Record: api.RecordFact{ID: api.ID(id), IP: inferenceAddresses()[address]}, Membership: membership}}}
}

func TestDNSInferenceTransitions(t *testing.T) {
	t.Parallel()
	uncertain := inferenceEffect(api.MutationUnconfirmed, api.RecordUpdate, "r", 1, api.MembershipManaged)
	completed := inferenceEffect(api.MutationCompleted, api.RecordUpdate, "r", 2, api.MembershipManaged)
	tests := []struct {
		name   string
		rounds []setter.DNSResult
		want   []bool
	}{
		{"first read", []setter.DNSResult{inferenceRead("r", 0)}, []bool{false}},
		{"exclusive replacement rejects empty", []setter.DNSResult{inferenceRead("r", 0), uncertain, inferenceRead("")}, []bool{false, false, true}},
		{"exclusive replacement rejects union", []setter.DNSResult{inferenceRead("r", 0), uncertain, inferenceRead("rs", 0, 1)}, []bool{false, false, true}},
		{"resolved alternative cannot return", []setter.DNSResult{inferenceRead("r", 0), uncertain, inferenceRead("r", 0), completed, inferenceRead("r", 1)}, []bool{false, false, false, false, true}},
		{"discrepancy reanchors", []setter.DNSResult{inferenceRead("r", 0), inferenceRead("r", 2), inferenceRead("r", 2), inferenceRead("r", 0)}, []bool{false, true, false, true}},
		{"successful create requires presence", []setter.DNSResult{inferenceRead(""), inferenceEffect(api.MutationCompleted, api.RecordCreate, "r", 1, api.MembershipManaged), inferenceRead("")}, []bool{false, false, true}},
		{"optional create permits absence", []setter.DNSResult{inferenceRead(""), inferenceEffect(api.MutationUnconfirmed, api.RecordCreate, "", 1, api.MembershipManaged), inferenceRead("")}, []bool{false, false, false}},
		{"optional create permits presence", []setter.DNSResult{inferenceRead(""), inferenceEffect(api.MutationUnconfirmed, api.RecordCreate, "", 1, api.MembershipManaged), inferenceRead("s", 1)}, []bool{false, false, false}},
		{"duplicate addresses preserve other identity", []setter.DNSResult{inferenceRead("rs", 0, 0), inferenceEffect(api.MutationCompleted, api.RecordDelete, "r", 0, api.MembershipUnknown), inferenceRead("")}, []bool{false, false, true}},
		{"identical duplicate identity normalizes", []setter.DNSResult{inferenceRead("rr", 0, 0), inferenceEffect(api.MutationCompleted, api.RecordDelete, "r", 0, api.MembershipUnknown), inferenceRead("")}, []bool{false, false, false}},
		{"contradictory identities cannot anchor", []setter.DNSResult{inferenceRead("rr", 0, 1), inferenceRead("s", 2), inferenceRead("")}, []bool{false, false, true}},
		{"ownership exit", []setter.DNSResult{inferenceRead("r", 0), inferenceEffect(api.MutationCompleted, api.RecordUpdate, "r", 1, api.MembershipUnmanaged), inferenceRead("")}, []bool{false, false, false}},
		{"unknown ownership permits absence", []setter.DNSResult{inferenceRead("r", 0), inferenceEffect(api.MutationCompleted, api.RecordUpdate, "r", 1, api.MembershipUnknown), inferenceRead("")}, []bool{false, false, false}},
		{"unconfirmed ownership exit retains old", []setter.DNSResult{inferenceRead("r", 0), inferenceEffect(api.MutationUnconfirmed, api.RecordUpdate, "r", 1, api.MembershipUnknown), inferenceRead("r", 0)}, []bool{false, false, false}},
		{"unconfirmed delete retains old", []setter.DNSResult{inferenceRead("r", 0), inferenceEffect(api.MutationUnconfirmed, api.RecordDelete, "r", 0, api.MembershipUnknown), inferenceRead("r", 0)}, []bool{false, false, false}},
		{"unknown effect association is conservative", []setter.DNSResult{inferenceRead("r", 0), inferenceEffect(api.MutationCompleted, api.RecordUpdate, "s", 1, api.MembershipManaged), inferenceRead("s", 1)}, []bool{false, false, false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var m dnsInference
			for i, r := range tt.rounds {
				if got := m.assess(r); got != tt.want[i] {
					t.Fatalf("round %d got %v want %v", i, got, tt.want[i])
				}
			}
		})
	}
}

func TestDNSInferenceCachedAndMissingReads(t *testing.T) {
	t.Parallel()
	var m dnsInference
	m.assess(inferenceRead("r", 0))
	m.assess(inferenceEffect(api.MutationUnconfirmed, api.RecordUpdate, "r", 1, api.MembershipManaged))
	cached := inferenceRead("r", 0)
	cached.Before.Cached = true
	if m.assess(cached) || m.assess(setter.DNSResult{Code: setter.ResponseNoop, Before: nil, Effects: nil}) || m.assess(inferenceRead("r", 1)) {
		t.Fatal("cached or missing input refined alternatives")
	}
	var empty dnsInference
	if empty.assess(cached) || empty.assess(inferenceRead("r", 2)) {
		t.Fatal("cache created a baseline")
	}
}

func TestDNSInferenceCompareBeforeEffects(t *testing.T) {
	t.Parallel()
	var m dnsInference
	m.assess(inferenceRead("r", 0))
	next := inferenceRead("r", 1)
	next.Effects = inferenceEffect(api.MutationCompleted, api.RecordUpdate, "r", 0, api.MembershipManaged).Effects
	if !m.assess(next) || m.assess(inferenceRead("r", 0)) {
		t.Fatal("must compare, reanchor, then apply effects")
	}
}

func TestDNSInferenceWidenAndRecover(t *testing.T) {
	t.Parallel()
	var m dnsInference
	large := setter.DNSResult{Code: setter.ResponseNoop, Effects: nil, Before: &setter.DNSSnapshot{Cached: false, Records: nil}}
	for i := range 257 {
		large.Before.Records = append(large.Before.Records, api.RecordFact{ID: api.ID(strconv.Itoa(i)), IP: inferenceAddresses()[0]})
	}
	if m.assess(large) || m.assess(inferenceRead("r", 1)) || !m.assess(inferenceRead("r", 2)) {
		t.Fatal("factor overflow must widen, then fresh read restores precision")
	}
	var edges dnsInference
	edges.assess(inferenceRead("r", 0))
	for i := range 4097 {
		effect := inferenceEffect(api.MutationUnconfirmed, api.RecordUpdate, "r", 0, api.MembershipManaged)
		effect.Effects[0].Record.IP = netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
		edges.assess(effect)
	}
	if edges.assess(inferenceRead("r", 1)) || !edges.assess(inferenceRead("r", 2)) {
		t.Fatal("edge overflow must widen, then recover")
	}
}

func TestCompatibleFactorsExamples(t *testing.T) {
	t.Parallel()
	a, b, c := inferenceAddresses()[0], inferenceAddresses()[1], inferenceAddresses()[2]
	tests := []struct {
		name     string
		factors  []recordFactor
		observed []netip.Addr
		want     bool
	}{
		{"required choice cannot disappear", []recordFactor{{id: "", absent: false, addresses: []netip.Addr{a, b}}}, nil, false},
		{"required choice cannot split", []recordFactor{{id: "", absent: false, addresses: []netip.Addr{a, b}}}, []netip.Addr{a, b}, false},
		{"matching bottleneck", []recordFactor{{id: "", absent: false, addresses: []netip.Addr{a, b}}, {id: "", absent: false, addresses: []netip.Addr{c}}, {id: "", absent: false, addresses: []netip.Addr{c}}}, []netip.Addr{a, b, c}, false},
		{"augmenting path", []recordFactor{{id: "", absent: false, addresses: []netip.Addr{a, b}}, {id: "", absent: false, addresses: []netip.Addr{a}}}, []netip.Addr{a, b}, true},
		{"duplicate observation is a set", []recordFactor{{id: "", absent: false, addresses: []netip.Addr{a}}}, []netip.Addr{a, a}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := compatibleFactors(tt.factors, tt.observed); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestDNSInferenceIdentityAndPresence(t *testing.T) {
	t.Parallel()
	t.Run("symbolic create then identified update", func(t *testing.T) {
		t.Parallel()
		var m dnsInference
		m.assess(inferenceRead(""))
		m.assess(inferenceEffect(api.MutationUnconfirmed, api.RecordCreate, "", 0, api.MembershipManaged))
		m.assess(inferenceEffect(api.MutationCompleted, api.RecordUpdate, "r", 1, api.MembershipManaged))
		if m.assess(inferenceRead("r", 1)) || !m.assess(inferenceRead("r", 2)) {
			t.Fatal("unknown association must widen, then recover")
		}
	})
	t.Run("successful symbolic create requires presence", func(t *testing.T) {
		t.Parallel()
		var m dnsInference
		m.assess(inferenceRead(""))
		m.assess(inferenceEffect(api.MutationCompleted, api.RecordCreate, "", 0, api.MembershipManaged))
		if !m.assess(inferenceRead("")) {
			t.Fatal("successful create was made optional")
		}
	})
	t.Run("fresh presence replaces optional factors", func(t *testing.T) {
		t.Parallel()
		var m dnsInference
		m.assess(inferenceRead(""))
		for range 2 {
			m.assess(inferenceEffect(api.MutationUnconfirmed, api.RecordCreate, "", 0, api.MembershipManaged))
		}
		if m.assess(inferenceRead("r", 0)) || !m.assess(inferenceRead("")) {
			t.Fatal("fresh presence must replace, not intersect, alternatives")
		}
	})
	t.Run("malformed observation cannot classify or anchor", func(t *testing.T) {
		t.Parallel()
		for _, fact := range []api.RecordFact{{ID: "", IP: inferenceAddresses()[1]}, {ID: "r", IP: netip.Addr{}}} {
			var m dnsInference
			m.assess(inferenceRead("r", 0))
			if m.assess(setter.DNSResult{Code: setter.ResponseNoop, Effects: nil, Before: &setter.DNSSnapshot{Cached: false, Records: []api.RecordFact{fact}}}) || m.assess(inferenceRead("r", 2)) || !m.assess(inferenceRead("")) {
				t.Fatal("invalid snapshot created false precision")
			}
		}
	})
	t.Run("cached effects still apply", func(t *testing.T) {
		t.Parallel()
		var m dnsInference
		m.assess(inferenceRead("r", 0))
		cached := inferenceRead("r", 0)
		cached.Before.Cached = true
		cached.Effects = inferenceEffect(api.MutationCompleted, api.RecordUpdate, "r", 1, api.MembershipManaged).Effects
		if m.assess(cached) || !m.assess(inferenceRead("r", 0)) {
			t.Fatal("cached read suppressed a reached effect")
		}
	})
}

// Independent concrete outcomes check effect construction, not merely matching.
// One update and one delete may independently complete or leave their old fact.
func TestDNSInferencePartialEffectsSoundness(t *testing.T) {
	t.Parallel()
	for _, updateStatus := range []api.MutationStatus{api.MutationNoEffect, api.MutationUnconfirmed, api.MutationCompleted} {
		for _, deleteStatus := range []api.MutationStatus{api.MutationNoEffect, api.MutationUnconfirmed, api.MutationCompleted} {
			for _, updated := range []bool{false, true} {
				if updateStatus == api.MutationNoEffect && updated || updateStatus == api.MutationCompleted && !updated {
					continue
				}
				for _, deleted := range []bool{false, true} {
					if deleteStatus == api.MutationNoEffect && deleted || deleteStatus == api.MutationCompleted && !deleted {
						continue
					}
					var m dnsInference
					m.assess(inferenceRead("rs", 0, 2))
					m.assess(inferenceEffect(updateStatus, api.RecordUpdate, "r", 1, api.MembershipManaged))
					m.assess(inferenceEffect(deleteStatus, api.RecordDelete, "s", 0, api.MembershipUnknown))
					address := 0
					if updated {
						address = 1
					}
					observed := inferenceRead("r", address)
					if !deleted {
						observed = inferenceRead("rs", address, 2)
					}
					if m.assess(observed) {
						t.Fatalf("excluded own outcome: update=%v/%v delete=%v/%v", updateStatus, updated, deleteStatus, deleted)
					}
				}
			}
		}
	}
}
