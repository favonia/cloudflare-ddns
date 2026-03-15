// Package setter implements reconciliation logic for DNS records and WAF lists
// using [api.Handle].
//
// The idea is to reuse existing DNS records as much as possible, and only when
// that fails, create new DNS records and remove stale ones. The complexity of
// this package is due to the error handling of each API call.
package setter

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

//go:generate go tool mockgen -typed -destination=../mocks/mock_setter.go -package=mocks . Setter

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
		configuredParams api.RecordParams,
	) ResponseCode

	// FinalDelete removes DNS records of a particular domain.
	FinalDelete(
		ctx context.Context,
		ppfmt pp.PP,
		ipFamily ipnet.Family,
		Domain domain.Domain,
		configuredParams api.RecordParams,
	) ResponseCode

	// SetWAFList reconciles one WAF list against family target states.
	//
	// Contract for targetsByFamily:
	// - map presence means the family is managed/in scope for this run
	// - map absence means the family is out of scope and existing managed content
	//   of that family must be preserved
	// - present + Available=false preserves existing managed content because the
	//   desired targets are unavailable for this run
	// - present + Available=true with an empty IP list is the explicit-empty
	//   intent for that family
	// - present + Available=true with a non-empty IP list carries a deterministic
	//   target set (sorted, deduplicated)
	SetWAFList(
		ctx context.Context,
		ppfmt pp.PP,
		list api.WAFList,
		listDescription string,
		targetsByFamily map[ipnet.Family]provider.Targets,
		configuredItemComment string,
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
