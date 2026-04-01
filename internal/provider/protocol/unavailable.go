package protocol

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Unavailable is a synthetic provider that always reports detection as unavailable.
type Unavailable struct {
	// Name of the detection protocol.
	ProviderName string
}

// Name of the detection protocol.
func (p Unavailable) Name() string {
	return p.ProviderName
}

// IsExplicitEmpty reports whether the provider intentionally clears the family.
// Unavailable is in-scope but produces no usable data; it is not explicit-empty.
func (p Unavailable) IsExplicitEmpty() bool {
	return false
}

// GetRawData always reports unavailable raw data for the requested family.
func (p Unavailable) GetRawData(
	_ context.Context, ppfmt pp.PP, _ ipnet.Family, _ int,
) DetectionResult {
	ppfmt.Infof(pp.EmojiError,
		"The provider %s simulates a detection failure; no real detection is attempted",
		p.ProviderName)
	return NewUnavailableDetectionResult()
}
