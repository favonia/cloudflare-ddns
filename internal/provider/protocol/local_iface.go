package protocol

import (
	"context"
	"net"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// LocalWithInterface detects the IP address by choosing the first "good" IP
// address assigned to a network interface.
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
		ppfmt.Noticef(pp.EmojiImpossible, "Unexpected data %q of type %T in interface %s", addr.String(), addr, iface)
		return netip.Addr{}, false
	}
}

func SelectInterfaceIP(ppfmt pp.PP, iface string, ipNet ipnet.Type, addrs []net.Addr) (netip.Addr, Method, bool) {
	ips := make([]netip.Addr, 0, len(addrs))
	for _, addr := range addrs {
		ip, ok := ExtractInterfaceAddr(ppfmt, iface, addr)
		if !ok {
			return ip, MethodUnspecified, false
		}
		ips = append(ips, ip)
	}

	i := slices.IndexFunc(ips, func(ip netip.Addr) bool {
		return ipNet.Matches(ip) && ip.IsGlobalUnicast()
	})
	if i >= 0 {
		return ips[i], MethodPrimary, true
	}

	// Choose an IP that is above the link-local scope
	i = slices.IndexFunc(ips, func(ip netip.Addr) bool {
		return ipNet.Matches(ip) &&
			!ip.IsUnspecified() &&
			!ip.IsLoopback() &&
			!ip.IsInterfaceLocalMulticast() &&
			!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
	})
	if i >= 0 {
		ppfmt.Noticef(pp.EmojiWarning,
			"Failed to find any global unicast %s address assigned to interface %s, "+
				"but found an address %s with a scope larger than the link-local scope",
			ipNet.Describe(), iface, ips[i].String())
		return ips[i], MethodPrimary, true
	}

	ppfmt.Noticef(pp.EmojiError,
		"Failed to find any global unicast %s address assigned to interface %s",
		ipNet.Describe(), iface)
	return netip.Addr{}, MethodUnspecified, false
}

// GetIP detects the IP address by pretending to send an UDP packet.
// (No actual UDP packets will be sent out.)
func (p LocalWithInterface) GetIP(_ context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, Method, bool) {
	iface, err := net.InterfaceByName(p.InterfaceName)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to find an interface named %q: %v", p.InterfaceName, err)
		return netip.Addr{}, MethodUnspecified, false
	}

	addrs, err := iface.Addrs()
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible, "Failed to list addresses of interface %s: %v", p.InterfaceName, err)
		return netip.Addr{}, MethodUnspecified, false
	}

	return SelectInterfaceIP(ppfmt, p.InterfaceName, ipNet, addrs)
}
