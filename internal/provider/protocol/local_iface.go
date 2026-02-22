package protocol

import (
	"context"
	"net"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// LocalWithInterface detects the IP address by choosing the first "good"
// unicast IP address assigned to a network interface.
type LocalWithInterface struct {
	// Name of the detection protocol.
	ProviderName string

	// The name of the network interface
	InterfaceName string
}

// Name of the detection protocol.
func (p LocalWithInterface) Name() string {
	return p.ProviderName
}

// ExtractInterfaceAddr converts an address from [net.Interface.Addrs] to [netip.Addr].
// The address will be unmapped.
func ExtractInterfaceAddr(ppfmt pp.PP, iface string, addr net.Addr) (netip.Addr, bool) {
	switch v := addr.(type) {
	case *net.IPAddr:
		ip, ok := netip.AddrFromSlice(v.IP)
		if !ok {
			ppfmt.Noticef(pp.EmojiImpossible, "Failed to parse address %q assigned to interface %s", v.IP.String(), iface)
			return netip.Addr{}, false
		}
		return ip.Unmap().WithZone(v.Zone), true
	case *net.IPNet:
		ip, ok := netip.AddrFromSlice(v.IP)
		if !ok {
			ppfmt.Noticef(pp.EmojiImpossible, "Failed to parse address %q assigned to interface %s", v.IP.String(), iface)
			return netip.Addr{}, false
		}
		return ip.Unmap(), true
	default:
		ppfmt.Noticef(pp.EmojiImpossible,
			"Unexpected address data %q of type %T found in interface %s",
			addr.String(), addr, iface)
		return netip.Addr{}, false
	}
}

// SelectInterfaceIP takes a list of unicast [net.Addr] and chooses the first reasonable IP (if any).
func SelectInterfaceIP(ppfmt pp.PP, iface string, ipNet ipnet.Type, addrs []net.Addr) (netip.Addr, bool) {
	ips := make([]netip.Addr, 0, len(addrs))
	for _, addr := range addrs {
		ip, ok := ExtractInterfaceAddr(ppfmt, iface, addr)
		if !ok {
			return ip, false
		}
		// net.Interface.Addrs documents that it returns only unicast interface addresses.
		// A multicast address here means this assumption is broken and should be reported.
		if ip.IsMulticast() {
			ppfmt.Noticef(pp.EmojiImpossible,
				"Found multicast address %s in net.Interface.Addrs for interface %s "+
					"(expected unicast addresses only); please report this at %s",
				ip.String(), iface, pp.IssueReportingURL,
			)
			return netip.Addr{}, false
		}
		ips = append(ips, ip)
	}

	i := slices.IndexFunc(ips, func(ip netip.Addr) bool {
		return ipNet.Matches(ip) && ip.IsGlobalUnicast()
	})
	if i >= 0 {
		return ips[i], true
	}

	// Fallback for deployments that intentionally publish internal addresses.
	// In practice, IsGlobalUnicast already covers almost all useful candidates for
	// DDNS, even for private/internal deployments.
	//
	// Current exceptional case after the filters above: IPv4 limited broadcast
	// 255.255.255.255 (including ::ffff:255.255.255.255 before Unmap in
	// ExtractInterfaceAddr). This fallback is kept as a future-proof guard for
	// unusual address classes that are not link-local/loopback/unspecified yet
	// also not classified as global unicast. Multicast addresses have already been
	// rejected above.
	i = slices.IndexFunc(ips, func(ip netip.Addr) bool {
		return ipNet.Matches(ip) &&
			!ip.IsUnspecified() &&
			!ip.IsLoopback() &&
			!ip.IsLinkLocalUnicast()
	})
	if i >= 0 {
		ppfmt.Noticef(pp.EmojiWarning,
			"Failed to find any global unicast %s address among unicast addresses assigned to interface %s, "+
				"but found a unicast address %s with a scope larger than the link-local scope",
			ipNet.Describe(), iface, ips[i].String())
		return ips[i], true
	}

	ppfmt.Noticef(pp.EmojiError,
		"Failed to find any global unicast %s address among unicast addresses assigned to interface %s",
		ipNet.Describe(), iface)
	return netip.Addr{}, false
}

// GetIP detects the IP address from unicast addresses assigned to a network
// interface.
func (p LocalWithInterface) GetIP(_ context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, bool) {
	iface, err := net.InterfaceByName(p.InterfaceName)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to find an interface named %q: %v", p.InterfaceName, err)
		return netip.Addr{}, false
	}

	addrs, err := iface.Addrs()
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible, "Failed to list unicast addresses of interface %s: %v", p.InterfaceName, err)
		return netip.Addr{}, false
	}

	return SelectInterfaceIP(ppfmt, p.InterfaceName, ipNet, addrs)
}
