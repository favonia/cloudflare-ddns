package protocol

import (
	"context"
	"net"
	"net/netip"

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
func ExtractInterfaceAddr(ppfmt pp.PP, addr net.Addr, iface string) (netip.Addr, bool) {
	switch v := addr.(type) {
	case *net.IPAddr:
		ip, ok := netip.AddrFromSlice(v.IP)
		if !ok {
			ppfmt.Noticef(pp.EmojiImpossible, "Failed to parse address %q assigned to interface %q", addr.String(), iface)
			return netip.Addr{}, false
		}
		return ip.Unmap().WithZone(v.Zone), true
	case *net.IPNet:
		ip, ok := netip.AddrFromSlice(v.IP)
		if !ok {
			ppfmt.Noticef(pp.EmojiImpossible, "Failed to parse address %q assigned to interface %s", addr.String(), iface)
			return netip.Addr{}, false
		}
		return ip.Unmap(), true
	default:
		ppfmt.Noticef(pp.EmojiImpossible, "Unexpected data %q of type %T in interface %s", addr.String(), addr, iface)
		return netip.Addr{}, false
	}
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
		ppfmt.Noticef(pp.EmojiImpossible, "Failed to list addresses of interface %q: %v", p.InterfaceName, err)
		return netip.Addr{}, MethodUnspecified, false
	}

	var firstOkayIP netip.Addr
	for _, addr := range addrs {
		ip, ok := ExtractInterfaceAddr(ppfmt, addr, p.InterfaceName)
		if !ok {
			return ip, MethodUnspecified, false
		}

		// Skip all addresses in the wrong IP family or of a scope smaller or equal to the link-local scope.
		if !ipNet.Matches(ip) ||
			ip.IsUnspecified() ||
			ip.IsLoopback() ||
			ip.IsInterfaceLocalMulticast() ||
			ip.IsLinkLocalUnicast() || ip.Prev().IsLinkLocalMulticast() {
			continue
		}

		// Choose the first unicast address of the global scope.
		if ip.IsGlobalUnicast() {
			return ip, MethodPrimary, true
		}

		// Otherwise, remember the first okay choice.
		if !firstOkayIP.IsValid() {
			firstOkayIP = ip
		}
	}
	if firstOkayIP.IsValid() {
		ppfmt.Noticef(pp.EmojiWarning,
			"Failed to find any global unicast %s address assigned to interface %s, "+
				"but found an address %s with a scope larger than the link-local scope",
			ipNet.Describe(), p.InterfaceName, firstOkayIP.String())
		return firstOkayIP, MethodPrimary, true
	}

	ppfmt.Noticef(pp.EmojiError, "Failed to find any global unicast %s address assigned to interface %q",
		ipNet.Describe(), p.InterfaceName)
	return netip.Addr{}, MethodUnspecified, false
}
