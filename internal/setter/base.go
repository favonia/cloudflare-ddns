// Package setter implements reconciliation logic for DNS records and WAF lists
// using [api.Handle].
//
// The idea is to reuse existing DNS records as much as possible, and only when
// that fails, create new DNS records and remove outdated ones. The complexity of
// this package is due to the error handling of each API call.
package setter

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate go tool mockgen -typed -destination=../mocks/mock_setter.go -package=mocks . Setter

// WAFTargets carries one managed family's derived WAF target prefixes for a run.
//
// Map presence means the family is managed/in scope for this run. Available
// distinguishes explicit-empty intent from temporary unavailability.
type WAFTargets struct {
	Available bool
	Prefixes  []netip.Prefix
}

// NewAvailableWAFTargets builds the managed deterministic WAF-target state.
func NewAvailableWAFTargets(prefixes []netip.Prefix) WAFTargets {
	return WAFTargets{Available: true, Prefixes: prefixes}
}

// NewUnavailableWAFTargets builds the managed temporary-unavailability state.
func NewUnavailableWAFTargets() WAFTargets {
	return WAFTargets{Available: false, Prefixes: nil}
}

// HasUsableTargets reports whether reconciliation may proceed for this family.
func (t WAFTargets) HasUsableTargets() bool {
	return t.Available
}

// Setter uses [api.Handle] to reconcile DNS records and WAF lists.
type Setter interface {
	// SetIPs sets a particular domain to the given IP addresses.
	//
	// Invariant: IPs must already be canonical and represent a deterministic set:
	// - each IP is valid, unzoned, and matches IPNetwork
	// - IPs are sorted by [netip.Addr.Compare] and deduplicated
	SetIPs(
		ctx context.Context,
		ppfmt pp.PP,
		ipFamily ipnet.Family,
		Domain domain.Domain,
		IPs []netip.Addr,
		fallbackParams api.RecordParams,
	) ResponseCode

	// FinalDelete removes DNS records of a particular domain.
	FinalDelete(
		ctx context.Context,
		ppfmt pp.PP,
		ipFamily ipnet.Family,
		Domain domain.Domain,
		fallbackParams api.RecordParams,
	) ResponseCode

	// SetWAFList reconciles one WAF list against family target states.
	//
	// Contract for targetsByFamily:
	// - map presence means the family is managed/in scope for this run
	// - map absence means the family is out of scope and existing managed content
	//   of that family must be preserved
	// - present + Available=false preserves existing managed content because the
	//   desired targets are unavailable for this run
	// - present + Available=true with an empty target list is the explicit-empty
	//   intent for that family
	// - present + Available=true with a non-empty target list carries a deterministic
	//   target set (sorted, deduplicated)
	// - when the list is missing, preserve/unavailable/explicit-empty families do
	//   not force structural creation by themselves
	SetWAFList(
		ctx context.Context,
		ppfmt pp.PP,
		list api.WAFList,
		listDescription string,
		targetsByFamily map[ipnet.Family]WAFTargets,
		fallbackItemComment string,
	) ResponseCode

	// FinalClearWAFList removes managed WAF content during shutdown in managed family scope.
	FinalClearWAFList(
		ctx context.Context,
		ppfmt pp.PP,
		list api.WAFList,
		listDescription string,
		managedFamilies map[ipnet.Family]bool,
	) ResponseCode
}
