package protocol

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Const returns the same IP.
type Const struct {
	// Name of the detection protocol.
	ProviderName string

	// The IP.
	IP netip.Addr
}

// Name of the detection protocol.
func (p Const) Name() string {
	return p.ProviderName
}

// GetIPs returns the IP in a singleton set.
func (p Const) GetIPs(_ context.Context, ppfmt pp.PP, ipNet ipnet.Type) ([]netip.Addr, bool) {
	normalizedIP, ok := ipNet.NormalizeDetectedIP(ppfmt, p.IP)
	if !ok {
		return nil, false
	}

	return []netip.Addr{normalizedIP}, true
}
