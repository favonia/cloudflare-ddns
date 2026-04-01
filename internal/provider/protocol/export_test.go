package protocol

import (
	"net"
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
