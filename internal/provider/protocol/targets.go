package protocol

import "net/netip"

// Targets bundles one managed family's provider output for a run.
//
// This runtime carrier is still address-only:
// IPv4 entries carry singleton /32 raw data, and IPv6 entries carry singleton
// /64 raw data.
//
// Runtime maps use presence to mean "managed/in scope" and absence to mean
// "out of scope". That keeps out-of-scope distinct from temporary target
// unavailability without reusing nil pointers or empty IP slices.
type Targets struct {
	// Available reports whether the provider output is known for this run.
	// When it is false, callers must preserve existing managed content of that
	// family because the provider output is unavailable.
	//
	// When it is true, IPs stores the current deterministic address-only carrier.
	// An empty IP list is the explicit-empty intent.
	Available bool
	IPs       []netip.Addr
}

// NewAvailableTargets builds the managed deterministic provider-output state.
func NewAvailableTargets(ips []netip.Addr) Targets {
	return Targets{Available: true, IPs: ips}
}

// NewUnavailableTargets builds the managed temporary-unavailability state.
func NewUnavailableTargets() Targets {
	return Targets{Available: false, IPs: nil}
}

// HasUsableTargets reports whether downstream derivation and reconciliation may proceed.
func (t Targets) HasUsableTargets() bool {
	return t.Available
}
