// Package provider implements protocols to detect public IP addresses.
package provider

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -destination=../mocks/mock_provider.go -package=mocks . Provider

// Provider is the abstraction of a protocol to detect public IP addresses.
type Provider interface {
	Name() string
	// Name gives the name of the protocol

	ShouldWeCheck1111() bool
	// ShouldWeCheck1111() says whether the provider will connect to 1.1.1.1.
	// Only when the IPv4 provider returns yes on this operation
	// the hijacking detection of 1.1.1.1 will be performed.

	GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, use1001 bool) (netip.Addr, bool)
	// Actually get the IP
}

// Name gets the protocol name. It returns "none" for nil.
func Name(p Provider) string {
	if p == nil {
		return "none"
	}

	return p.Name()
}
