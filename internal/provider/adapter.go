package provider

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// singleIPProvider keeps the legacy provider surface during migration.
type singleIPProvider interface {
	Name() string
	GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, bool)
}

// singleIPProviderAdapter adds temporary GetIPs support by wrapping GetIP.
type singleIPProviderAdapter struct {
	singleIPProvider
}

// GetIPs implements the additive multi-IP capability via legacy single-IP detection.
func (p singleIPProviderAdapter) GetIPs(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) ([]netip.Addr, bool) {
	ip, ok := p.GetIP(ctx, ppfmt, ipNet)
	if !ok {
		return nil, false
	}
	return []netip.Addr{ip}, true
}

// withMultiIPSupport upgrades a legacy provider to the additive multi-IP interface.
func withMultiIPSupport(p singleIPProvider) Provider {
	if p == nil {
		return nil
	}
	return singleIPProviderAdapter{singleIPProvider: p}
}
