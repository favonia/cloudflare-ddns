package provider

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func newStatic(ppfmt pp.PP, raw string) (Provider, bool) {
	ips := make([]netip.Addr, 0)
	for rawIP := range strings.SplitSeq(raw, ",") {
		rawIP = strings.TrimSpace(rawIP)

		ip, err := netip.ParseAddr(rawIP)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError, `Failed to parse the IP address %q for "static:"`, rawIP)
			return nil, false
		}
		if ip.Zone() != "" {
			ppfmt.Noticef(
				pp.EmojiUserError,
				`Failed to parse the IP address %q for "static:": zoned IP addresses are not allowed`,
				rawIP,
			)
			return nil, false
		}
		ips = append(ips, ip)
	}

	// Make the explicit-input provider deterministic before it enters the pipeline.
	slices.SortFunc(ips, netip.Addr.Compare)
	ips = slices.Compact(ips)

	rawIPs := make([]string, 0, len(ips))
	for _, ip := range ips {
		rawIPs = append(rawIPs, ip.String())
	}
	return protocol.NewStatic("static:"+strings.Join(rawIPs, ","), ips), true
}

// NewStatic creates a [protocol.Static] provider.
func NewStatic(ppfmt pp.PP, raw string) (Provider, bool) {
	return newStatic(ppfmt, raw)
}

// NewStaticEmpty creates an explicit-empty [protocol.Static] provider.
func NewStaticEmpty() Provider {
	return protocol.NewStatic("static.empty", nil)
}

// MustNewStatic creates a [protocol.Static] provider and panics if it fails.
func MustNewStatic(raw string) Provider {
	var buf strings.Builder
	p, ok := NewStatic(pp.NewDefault(&buf), raw)
	if !ok {
		panic(buf.String())
	}
	return p
}

// IsStaticEmpty reports whether the provider is the explicit-empty static mode.
func IsStaticEmpty(p Provider) bool {
	return Name(p) == "static.empty"
}

// StaticTargets returns the configured explicit targets of a static provider.
func StaticTargets(p Provider) ([]netip.Addr, bool) {
	static, ok := p.(protocol.Static)
	if !ok {
		return nil, false
	}
	return slices.Clone(static.IPs), true
}

// StaticMatchesFamily reports whether all explicit static targets match ipFamily.
func StaticMatchesFamily(p Provider, ipFamily ipnet.Family) bool {
	ips, ok := StaticTargets(p)
	if !ok {
		return false
	}
	for _, ip := range ips {
		if !ipFamily.Matches(ip) {
			return false
		}
	}
	return true
}
