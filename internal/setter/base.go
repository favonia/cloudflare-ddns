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

//go:generate mockgen -typed -destination=../mocks/mock_setter.go -package=mocks . Setter

// Setter uses [api.Handle] to update DNS records.
type Setter interface {
	// Set sets a particular domain to the given IP address.
	Set(
		ctx context.Context,
		ppfmt pp.PP,
		IPNetwork ipnet.Type,
		Domain domain.Domain,
		IP netip.Addr,
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

	// SetWAFList keeps only IP ranges overlapping with detected IPs
	// and makes sure there will be ranges overlapping with detected ones.
	SetWAFList(
		ctx context.Context,
		ppfmt pp.PP,
		list api.WAFList,
		listDescription string,
		detectedRange map[ipnet.Type]netip.Prefix,
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
