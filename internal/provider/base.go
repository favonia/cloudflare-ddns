// Package provider implements protocols to detect public IP addresses.
package provider

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

//go:generate mockgen -typed -destination=../mocks/mock_provider.go -package=mocks . Provider,SplitProvider

// Method reexports [protocol.Method].
type Method = protocol.Method

// Re-exporting constants from protocol.
const (
	MethodPrimary     = protocol.MethodPrimary
	MethodAlternative = protocol.MethodAlternative
	MethodUnspecified = protocol.MethodUnspecified
)

// Provider is the abstraction of a protocol to detect public IP addresses.
type Provider interface {
	Name() string
	// Name gives the name of the protocol.

	GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, Method, bool)
	// GetIP gets the IP.
}

// SplitProvider is the abstraction of a protocol to detect public IP addresses
// in potentially two ways. GetIP is taking an extra argument to change the method.
// So far, all alternative methods are to connect to 1.0.0.1 instead of 1.1.1.1
// to work around bad ISPs or bad routers.
type SplitProvider interface {
	Name() string
	// Name gives the name of the protocol.

	HasAlternative(ipNet ipnet.Type) bool
	// HasAlternative checks whether there is a different alternative method.

	GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, method Method) (netip.Addr, bool)
	// GetIP gets the IP.
}

// Name gets the protocol name. It returns "none" for nil.
func Name(p Provider) string {
	if p == nil {
		return "none"
	}

	return p.Name()
}
