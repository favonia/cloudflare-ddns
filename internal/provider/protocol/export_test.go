package protocol

import (
	"context"
	"net"
	"net/http"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func ExtractUDPAddr(ppfmt pp.PP, addr net.Addr) (netip.Addr, bool) {
	return extractUDPAddr(ppfmt, addr)
}

func ExtractInterfaceAddr(ppfmt pp.PP, iface string, addr net.Addr) (netip.Addr, bool) {
	return extractInterfaceAddr(ppfmt, iface, addr)
}

func SelectAndNormalizeInterfaceIPs(
	ppfmt pp.PP, iface string, ipFamily ipnet.Family, defaultPrefixLen int, addrs []net.Addr,
) DetectionResult {
	return selectAndNormalizeInterfaceIPs(ppfmt, iface, ipFamily, defaultPrefixLen, addrs)
}

// GetRawDataWithHTTPClient lets external behavior tests use an in-memory HTTP server
// inside synctest while exercising the complete trace detection flow.
func (p CloudflareTrace) GetRawDataWithHTTPClient(
	ctx context.Context, ppfmt pp.PP, family ipnet.Family, prefixLen int, client *http.Client,
) DetectionResult {
	return p.getRawDataWithHTTPClient(ctx, ppfmt, family, prefixLen, client)
}
