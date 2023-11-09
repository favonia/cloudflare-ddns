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

// ResponseCode encodes the minimum information to generate messages for monitors and notifiers.
type ResponseCode int

const (
	// No updates were needed. The records were already okay.
	ResponseNoUpdatesNeeded = iota
	// Updates were needed and they are done.
	ResponseUpdatesApplied
	// Updates were needed and they did not fully complete. The records may be inconsistent.
	ResponseUpdatesFailed
)

// Setter uses [api.Handle] to update DNS records.
type Setter interface {
	// Set sets a particular domain to the given IP address.
	Set(
		ctx context.Context,
		ppfmt pp.PP,
		Domain domain.Domain,
		IPNetwork ipnet.Type,
		IP netip.Addr,
		ttl api.TTL,
		proxied bool,
	) ResponseCode

	// Clear removes DNS records of a particular domain.
	Delete(
		ctx context.Context,
		ppfmt pp.PP,
		Domain domain.Domain,
		IPNetwork ipnet.Type,
	) ResponseCode
}
