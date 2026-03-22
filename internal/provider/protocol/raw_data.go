package protocol

import (
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// DetectionResult carries one managed family's detection-phase raw data for a run.
//
// Runtime maps use presence to mean "managed/in scope" and absence to mean
// "out of scope". That keeps out-of-scope distinct from temporary raw-data
// unavailability without reusing nil pointers or empty slices.
type DetectionResult struct {
	// Available reports whether the raw data is known for this run.
	// When it is false, callers must preserve existing managed content of that
	// family because the raw data is unavailable.
	//
	// When it is true, CIDRs stores the current deterministic raw-data carrier.
	// An empty CIDR list is the explicit-empty intent.
	Available bool
	CIDRs     []netip.Prefix
}

// NewKnownDetectionResult builds the managed deterministic raw-data state.
func NewKnownDetectionResult(cidrs []netip.Prefix) DetectionResult {
	return DetectionResult{Available: true, CIDRs: cidrs}
}

// NewUnavailableDetectionResult builds the managed temporary-unavailability state.
func NewUnavailableDetectionResult() DetectionResult {
	return DetectionResult{Available: false, CIDRs: nil}
}

// HasUsableRawData reports whether downstream derivation and reconciliation may proceed.
func (r DetectionResult) HasUsableRawData() bool {
	return r.Available
}

// DefaultRawDataPrefixLen returns the shared product default used when lifting
// a bare detected address into detection-phase raw data for one family.
func DefaultRawDataPrefixLen(ipFamily ipnet.Family) int {
	switch ipFamily {
	case ipnet.IP4:
		return 32
	case ipnet.IP6:
		return 64
	default:
		return 0
	}
}

// NormalizeDetectedRawData validates detected addresses for one family and lifts
// them into deterministic raw-data CIDRs using the given default prefix length.
func NormalizeDetectedRawData(
	ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int, ips []netip.Addr,
) ([]netip.Prefix, bool) {
	ips, ok := ipFamily.NormalizeDetectedIPs(ppfmt, ips)
	if !ok {
		return nil, false
	}
	return ipnet.LiftValidatedIPsToPrefixes(ips, defaultPrefixLen), true
}
