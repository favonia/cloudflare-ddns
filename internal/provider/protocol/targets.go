package protocol

import "net/netip"

// Targets bundles one managed family's desired targets for a run.
//
// Runtime maps use presence to mean "managed/in scope" and absence to mean
// "out of scope". That keeps out-of-scope distinct from temporary target
// unavailability without reusing nil pointers or empty IP slices.
type Targets struct {
	// Available reports whether the desired target set is known for this run.
	// When it is false, callers must preserve existing managed content of that
	// family because the desired targets are unavailable.
	//
	// When it is true, IPs is the deterministic desired target set for the run.
	// An empty IP list is the explicit-empty intent.
	Available bool
	IPs       []netip.Addr
}

// NewAvailableTargets builds the managed deterministic target-set state.
func NewAvailableTargets(ips []netip.Addr) Targets {
	return Targets{Available: true, IPs: ips}
}

// NewUnavailableTargets builds the managed temporary-unavailability state.
func NewUnavailableTargets() Targets {
	return Targets{Available: false}
}

// HasUsableTargets reports whether reconciliation may mutate toward a known target set.
func (t Targets) HasUsableTargets() bool {
	return t.Available
}
