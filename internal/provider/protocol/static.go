package protocol

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Static returns the same set of IPs.
type Static struct {
	// Name of the detection protocol.
	ProviderName string

	// The IPs. Config-side constructors canonicalize these for stable naming.
	// Runtime normalization still runs in GetIPs because the provider contract is
	// enforced per requested family at the point the targets are consumed.
	IPs []netip.Addr
}

// NewStatic creates a static provider with a defensive copy of ips.
func NewStatic(providerName string, ips []netip.Addr) Static {
	return Static{
		ProviderName: providerName,
		IPs:          append([]netip.Addr(nil), ips...),
	}
}

// Name of the detection protocol.
func (p Static) Name() string {
	return p.ProviderName
}

// GetIPs returns the IPs as a deterministic set.
func (p Static) GetIPs(_ context.Context, ppfmt pp.PP, ipFamily ipnet.Family) Targets {
	ips, ok := ipFamily.NormalizeDetectedIPs(ppfmt, p.IPs)
	if !ok {
		return NewUnavailableTargets()
	}
	return NewAvailableTargets(ips)
}
