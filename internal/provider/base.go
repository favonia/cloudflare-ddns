// Package provider implements protocols to detect public IP addresses.
package provider

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

//go:generate go tool mockgen -typed -destination=../mocks/mock_provider.go -package=mocks . Provider

// Provider is the abstraction of a protocol to detect public IP addresses.
type Provider interface {
	Name() string
	// Name gives the name of the protocol.

	GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, bool)
	// GetIP gets one detected IP for the requested network family.
	// Existing provider/updater flows treat this as a singleton target.

	GetIPs(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) ([]netip.Addr, bool)
	// GetIPs gets all detected IPs for the requested network family.
	// Providers that only support one detected IP are temporarily adapted
	// to return a singleton slice until migration is complete.
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
