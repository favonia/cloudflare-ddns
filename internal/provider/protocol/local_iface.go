package protocol

import (
	"context"
	"net"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// LocalWithInterface detects IP addresses assigned to a network interface.
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

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (LocalWithInterface) IsExplicitEmpty() bool {
	return false
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

// SelectAndNormalizeInterfaceIPs takes a list of unicast [net.Addr], keeps all
// matching global-unicast addresses, and lifts them to [DetectionResult].
func SelectAndNormalizeInterfaceIPs(
	ppfmt pp.PP, iface string, ipFamily ipnet.Family, defaultPrefixLen int, addrs []net.Addr,
) DetectionResult {
	ips := make([]netip.Addr, 0, len(addrs))
	for _, addr := range addrs {
		ip, ok := ExtractInterfaceAddr(ppfmt, iface, addr)
		// Fail fast on malformed interface data instead of proceeding with a partial snapshot.
		if !ok {
			return NewUnavailableDetectionResult()
		}
		// net.Interface.Addrs documents that it returns only unicast interface addresses.
		// A multicast address here means this assumption is broken and should be reported.
		if ip.IsMulticast() {
			ppfmt.Noticef(pp.EmojiImpossible,
				"Found multicast address %s in net.Interface.Addrs for interface %s "+
					"(expected unicast addresses only); please report this at %s",
				ip.String(), iface, pp.IssueReportingURL,
			)
			return NewUnavailableDetectionResult()
		}
		// Keep only addresses in the requested family that are usable as
		// unicast targets. Note that IsGlobalUnicast still includes private
		// and internal ranges.
		if !ipFamily.Matches(ip) || !ip.IsGlobalUnicast() {
			continue
		}
		// By this point the address matches the requested family and is global
		// unicast. A zone on a global address is unusual and often indicates a
		// misconfigured setup. Independently, Cloudflare DNS record content is
		// validated as an IPv4/IPv6 address, so zone-qualified values must be
		// rejected.
		if ip.Zone() != "" {
			ppfmt.Noticef(pp.EmojiWarning, "Ignoring zoned address %s assigned to interface %s", ip.String(), iface)
			continue
		}
		ips = append(ips, ip)
	}

	if len(ips) == 0 {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to find any global unicast %s address among unicast addresses assigned to interface %s",
			ipFamily.Describe(), iface)
		return NewUnavailableDetectionResult()
	}

	rawEntries, ok := NormalizeDetectedRawIPs(ppfmt, ipFamily, defaultPrefixLen, ips)
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(rawEntries)
}

// GetRawData detects raw data from unicast addresses assigned to a network
// interface.
func (p LocalWithInterface) GetRawData(
	_ context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	iface, err := net.InterfaceByName(p.InterfaceName)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to find an interface named %q: %v", p.InterfaceName, err)
		return NewUnavailableDetectionResult()
	}

	addrs, err := iface.Addrs()
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible, "Failed to list unicast addresses of interface %s: %v", p.InterfaceName, err)
		return NewUnavailableDetectionResult()
	}

	return SelectAndNormalizeInterfaceIPs(ppfmt, p.InterfaceName, ipFamily, defaultPrefixLen, addrs)
}
