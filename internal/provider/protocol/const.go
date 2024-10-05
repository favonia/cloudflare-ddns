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

// GetIP returns the IP.
func (p Const) GetIP(_ context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, Method, bool) {
	normalizedIP, ok := ipNet.NormalizeDetectedIP(ppfmt, p.IP)
	return normalizedIP, MethodPrimary, ok
}
