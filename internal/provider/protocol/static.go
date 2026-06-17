package protocol

import (
	"context"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Static returns the same set of raw IP addresses with prefix lengths.
type Static struct {
	// Name of the detection protocol.
	ProviderName string

	// The raw IP addresses with prefix lengths. Config-side constructors
	// canonicalize these for stable naming. Runtime normalization still runs
	// in GetRawData because the provider contract is enforced per requested
	// family at the point the raw data is consumed.
	RawEntries []ipnet.RawEntry
}

// Name of the detection protocol.
func (p Static) Name() string {
	return p.ProviderName
}

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (p Static) IsExplicitEmpty() bool {
	return len(p.RawEntries) == 0
}

// StaticRawData returns a defensive copy of the configuration-time raw data.
// The entries are fixed across runs and already canonical for this provider's
// family (see provider.NewStatic); this is a pure accessor, not a detection
// round like GetRawData. See provider.StaticProvider for the contract.
func (p Static) StaticRawData() []ipnet.RawEntry {
	return slices.Clone(p.RawEntries)
}

// GetRawData returns the static raw entries as deterministic raw data.
func (p Static) GetRawData(
	_ context.Context, ppfmt pp.PP, ipFamily ipnet.Family, _ int,
) DetectionResult {
	rawEntries, ok := ipFamily.NormalizeDetectedRawEntries(ppfmt, p.RawEntries)
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(rawEntries)
}
