package provider

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func newLiteral(ppfmt pp.PP, raw string) (Provider, bool) {
	rawIPs := strings.Split(raw, ",")
	ips := make([]netip.Addr, 0, len(rawIPs))
	for _, rawIP := range rawIPs {
		rawIP = strings.TrimSpace(rawIP)

		ip, err := netip.ParseAddr(rawIP)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError, `Failed to parse the IP address %q for "literal:"`, rawIP)
			return nil, false
		}
		if ip.Zone() != "" {
			ppfmt.Noticef(
				pp.EmojiUserError,
				`Failed to parse the IP address %q for "literal:": zoned IP addresses are not allowed`,
				rawIP,
			)
			return nil, false
		}
		ips = append(ips, ip)
	}

	// Make the explicit-input provider deterministic before it enters the pipeline.
	slices.SortFunc(ips, netip.Addr.Compare)
	ips = slices.Compact(ips)

	rawIPs = make([]string, 0, len(ips))
	for _, ip := range ips {
		rawIPs = append(rawIPs, ip.String())
	}
	return protocol.Static{
		ProviderName: "literal:" + strings.Join(rawIPs, ","),
		IPs:          ips,
	}, true
}

// NewLiteral creates a [protocol.Static] provider.
func NewLiteral(ppfmt pp.PP, raw string) (Provider, bool) {
	return newLiteral(ppfmt, raw)
}

// MustNewLiteral creates a [protocol.Static] provider and panics if it fails.
func MustNewLiteral(raw string) Provider {
	var buf strings.Builder
	p, ok := NewLiteral(pp.NewDefault(&buf), raw)
	if !ok {
		panic(buf.String())
	}
	return p
}
