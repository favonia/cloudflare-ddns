// Package setter implements the logic to update DNS records using [api.Handle].
//
// The idea is to reuse existing DNS records as much as possible, and only when
// that fails, create new DNS records and remove stall ones. The complexity of
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

// Setter uses [api.Handle] to update DNS records.
type Setter interface {
	// SetIPs sets a particular domain to the given IP addresses.
	//
	// Invariant: IPs must already be canonical and represent a deterministic set:
	// - each IP is valid, unzoned, and matches IPNetwork
	// - IPs are sorted by [netip.Addr.Compare] and deduplicated
	SetIPs(
		ctx context.Context,
		ppfmt pp.PP,
		IPNetwork ipnet.Type,
		Domain domain.Domain,
		IPs []netip.Addr,
		expectedParams api.RecordParams,
	) ResponseCode

	// FinalDelete removes DNS records of a particular domain.
	FinalDelete(
		ctx context.Context,
		ppfmt pp.PP,
		IPNetwork ipnet.Type,
		Domain domain.Domain,
		expectedParams api.RecordParams,
	) ResponseCode

	// SetWAFList keeps only IP ranges overlapping with detected target sets
	// and ensures each detected target is covered by at least one range.
	//
	// Contract for detected:
	// - if an entry exists with a non-empty slice, that family is managed and
	//   the slice is a deterministic target set (sorted, deduplicated)
	// - if an entry exists with an empty slice, detection was attempted but failed
	//   and existing matching ranges are preserved
	// - if an entry is missing, that family is unmanaged and matching ranges are removed
	SetWAFList(
		ctx context.Context,
		ppfmt pp.PP,
		list api.WAFList,
		listDescription string,
		detected map[ipnet.Type][]netip.Addr,
		itemComment string,
	) ResponseCode

	// FinalClearWAFList deletes or empties a list.
	FinalClearWAFList(
		ctx context.Context,
		ppfmt pp.PP,
		list api.WAFList,
		listDescription string,
	) ResponseCode
}
