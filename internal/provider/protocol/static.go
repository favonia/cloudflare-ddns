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

	// The IPs.
	IPs []netip.Addr
}

// Name of the detection protocol.
func (p Static) Name() string {
	return p.ProviderName
}

// GetIPs returns the IPs as a deterministic set.
func (p Static) GetIPs(_ context.Context, ppfmt pp.PP, ipNet ipnet.Type) ([]netip.Addr, bool) {
	return ipNet.NormalizeDetectedIPs(ppfmt, p.IPs)
}
