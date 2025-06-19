package provider

import (
	"context"
	"net/netip"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// composite combines a dynamic IP provider with static IP addresses.
type composite struct {
	// Name of the detection protocol.
	providerName string

	// Primary provider for dynamic IP detection.
	primary Provider

	// Static IP addresses.
	staticIPs []netip.Addr
}

// Name of the detection protocol.
func (p composite) Name() string {
	return p.providerName
}

// GetIP returns the primary IP. The static IPs are handled separately.
func (p composite) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, bool) {
	return p.primary.GetIP(ctx, ppfmt, ipNet)
}

// GetAllIPs returns both dynamic and static IPs.
func (p composite) GetAllIPs(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) ([]netip.Addr, bool) {
	// Get dynamic IP
	dynamicIP, ok := p.primary.GetIP(ctx, ppfmt, ipNet)
	if !ok {
		return nil, false
	}

	// Combine with static IPs, filtering by IP version
	var allIPs []netip.Addr
	allIPs = append(allIPs, dynamicIP)

	for _, staticIP := range p.staticIPs {
		// Normalize the static IP for the current IP version
		if normalizedIP, ok := ipNet.NormalizeDetectedIP(ppfmt, staticIP); ok {
			// Check if this IP is already in the list to avoid duplicates
			if !slices.Contains(allIPs, normalizedIP) {
				allIPs = append(allIPs, normalizedIP)
			}
		}
	}

	return allIPs, true
}

// NewComposite creates a composite provider that combines dynamic IP detection with static IPs.
func NewComposite(ppfmt pp.PP, primary Provider, staticIPs []string) (CompositeProvider, bool) {
	var parsedIPs []netip.Addr
	for _, ipStr := range staticIPs {
		ipStr = strings.TrimSpace(ipStr)
		if ipStr == "" {
			continue
		}
		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError, `Failed to parse static IP address %q`, ipStr)
			return nil, false
		}
		parsedIPs = append(parsedIPs, ip)
	}

	staticIPsStr := make([]string, len(parsedIPs))
	for i, ip := range parsedIPs {
		staticIPsStr[i] = ip.String()
	}

	return composite{
		providerName: primary.Name() + "+static[" + strings.Join(staticIPsStr, ",") + "]",
		primary:      primary,
		staticIPs:    parsedIPs,
	}, true
}
