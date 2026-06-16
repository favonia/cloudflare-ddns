// Package provider implements config-facing provider constructors and the
// runtime Provider interface.
//
// Creation functions accept an envKey parameter (the environment variable
// name) so that validation messages identify the configuration source.
// Pure protocol implementations live in provider/protocol.
package provider

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

//go:generate go tool mockgen -typed -destination=../mocks/mock_provider.go -package=mocks . Provider

// DetectionResult is the runtime family-state contract exported from the provider layer.
//
// This is the in-memory landing of the provider raw-data contract and
// ownership-model design notes:
//   - one map entry means the family is managed/in scope for this run
//   - map absence means the family is out of scope for this run
//   - Available=false means the family is in scope but its raw data is
//     unavailable for this run
//   - Available=true means the family is in scope and its raw data is
//     known for this run
//
// In the lifecycle model, this carrier is detection-phase raw data, not
// resource-specific derived targets.
type DetectionResult = protocol.DetectionResult

// NewKnownDetectionResult builds the managed deterministic raw-data state.
func NewKnownDetectionResult(rawEntries []ipnet.RawEntry) DetectionResult {
	return protocol.NewKnownDetectionResult(rawEntries)
}

// NewUnavailableDetectionResult builds the managed temporary-unavailability state.
func NewUnavailableDetectionResult() DetectionResult {
	return protocol.NewUnavailableDetectionResult()
}

// Provider is the abstraction of a protocol to detect public IP addresses.
type Provider interface {
	Name() string
	// Name gives the name of the protocol.

	IsExplicitEmpty() bool
	// IsExplicitEmpty reports whether the provider intentionally manages the
	// requested family to an empty result set when detection succeeds.

	GetRawData(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int) DetectionResult
	// GetRawData gets the detection-phase raw data for the requested managed network family.
	// defaultPrefixLen is the shared product default used by providers that need
	// to lift bare detected addresses into raw data.
	//
	// Contract:
	// - when Available is true:
	//   - each returned raw entry is valid and matches ipFamily
	//   - providers that lift bare addresses preserve the observed address bits
	//     while using defaultPrefixLen as their lifted prefix length
	//   - the slice is sorted by [ipnet.RawEntry.Compare] and deduplicated so
	//     callers can treat it as a deterministic set
	// - dynamic providers use Available=false when they cannot produce a usable
	//   raw-data set for the requested family
	// - explicit-empty modes use Available=true with an empty list
}

// StaticProvider is a Provider whose raw data is fixed at configuration time
// and never changes across runs. This is distinct from the per-run "known"
// state of GetRawData: every provider, including dynamic ones, reports a
// known-or-unavailable result each run, whereas only a StaticProvider exposes
// raw data settled before any detection round.
//
// Implementations return entries already normalized, sorted, and deduplicated
// for their family (see NewStatic), so callers may trust the result without
// revalidation.
type StaticProvider interface {
	Provider

	// StaticRawData returns the configuration-time raw data, fixed across runs.
	// It is a pure accessor: no context, no IO, no detection round (contrast
	// GetRawData). The returned slice is a defensive clone the caller may freely
	// mutate; it is already normalized, sorted, and deduplicated for the
	// provider's family.
	StaticRawData() []ipnet.RawEntry
}

// Name gets the protocol name. It returns "none" for nil.
func Name(p Provider) string {
	if p == nil {
		return "none"
	}

	return p.Name()
}

// CloseIdleConnections closes all idle (keep-alive) connections after the detection.
// This is to prevent some lingering TCP connections from disturbing the IP detection.
func CloseIdleConnections() {
	protocol.CloseIdleConnections()
}
