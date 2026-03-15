// Package provider implements protocols to detect public IP addresses.
package provider

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

//go:generate go tool mockgen -typed -destination=../mocks/mock_provider.go -package=mocks . Provider

// Targets is the runtime family-state contract exported from the provider layer.
//
// This is the in-memory landing of the provider target-validation and
// ownership-model design notes:
//   - one map entry means the family is managed/in scope for this run
//   - map absence means the family is out of scope for this run
//   - Available=false means the family is in scope but its desired target set is
//     unavailable for this run
//   - Available=true means the family is in scope and its desired target set is
//     known for this run
type Targets = protocol.Targets

// NewAvailableTargets builds the managed deterministic target-set state.
var NewAvailableTargets = protocol.NewAvailableTargets

// NewUnavailableTargets builds the managed temporary-unavailability state.
var NewUnavailableTargets = protocol.NewUnavailableTargets

// Provider is the abstraction of a protocol to detect public IP addresses.
type Provider interface {
	Name() string
	// Name gives the name of the protocol.

	GetIPs(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family) Targets
	// GetIPs gets the desired targets for the requested managed network family.
	//
	// Contract:
	// - when Available is true:
	//   - each returned IP is valid and matches ipFamily
	//   - each returned IP is canonical (e.g., IPv4-mapped IPv6 is unmapped)
	//   - each returned IP has no zone identifier and is suitable as DNS content
	//   - the slice is sorted by netip.Addr.Compare and deduplicated so callers
	//     can treat it as a deterministic set
	// - dynamic providers use Available=false when they cannot produce a usable
	//   target set for the requested family
	// - explicit static-empty modes use Available=true with an empty IP list
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
