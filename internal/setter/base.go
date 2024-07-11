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
	// SanityCheck determines whether one should continue trying
	SanityCheck(
		ctx context.Context,
		ppfmt pp.PP,
	) bool

	// Set sets a particular domain to the given IP address.
	Set(
		ctx context.Context,
		ppfmt pp.PP,
		Domain domain.Domain,
		IPNetwork ipnet.Type,
		IP netip.Addr,
		ttl api.TTL,
		proxied bool,
		recordComment string,
	) ResponseCode

	// Delete removes DNS records of a particular domain.
	Delete(
		ctx context.Context,
		ppfmt pp.PP,
		Domain domain.Domain,
		IPNetwork ipnet.Type,
	) ResponseCode
}
