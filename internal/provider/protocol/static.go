package protocol

import (
	"context"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Static returns the same set of raw CIDRs.
type Static struct {
	// Name of the detection protocol.
	ProviderName string

	// The raw CIDRs. Config-side constructors canonicalize these for stable
	// naming. Runtime normalization still runs in GetRawData because the
	// provider contract is enforced per requested family at the point the raw
	// data is consumed.
	CIDRs []netip.Prefix
}

// NewStatic creates a static provider with a defensive copy of cidrs.
func NewStatic(providerName string, cidrs []netip.Prefix) Static {
	return Static{
		ProviderName: providerName,
		CIDRs:        slices.Clone(cidrs),
	}
}

// Name of the detection protocol.
func (p Static) Name() string {
	return p.ProviderName
}

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (p Static) IsExplicitEmpty() bool {
	return len(p.CIDRs) == 0
}

// GetRawData returns the static raw CIDRs as deterministic raw data.
func (p Static) GetRawData(
	_ context.Context, ppfmt pp.PP, ipFamily ipnet.Family, _ int,
) DetectionResult {
	cidrs, ok := ipFamily.NormalizeDetectedPrefixes(ppfmt, p.CIDRs)
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(cidrs)
}
