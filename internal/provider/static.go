package provider

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewStatic creates a [protocol.Static] provider.
func NewStatic(ppfmt pp.PP, envKey string, ipFamily ipnet.Family, raw string) (Provider, bool) {
	ips := make([]netip.Addr, 0)
	for rawIP := range strings.SplitSeq(raw, ",") {
		rawIP = strings.TrimSpace(rawIP)

		if rawIP == "" {
			ppfmt.Noticef(pp.EmojiUserError,
				`%s has an empty entry (check for extra commas)`, envKey)
			return nil, false
		}

		ip, err := netip.ParseAddr(rawIP)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError, `Failed to parse the IP address %q in %s`, rawIP, envKey)
			return nil, false
		}
		if ip.Zone() != "" {
			ppfmt.Noticef(
				pp.EmojiUserError,
				`The IP address %q in %s has a zone identifier, which is not allowed`,
				rawIP,
				envKey,
			)
			return nil, false
		}
		if ipFamily == ipnet.IP6 && ip.Is4In6() {
			ppfmt.Noticef(
				pp.EmojiUserError,
				`The IP address %q in %s is an IPv4-mapped IPv6 address`,
				rawIP,
				envKey,
			)
			return nil, false
		}
		ip = ip.Unmap()
		if !ipFamily.Matches(ip) {
			ppfmt.Noticef(
				pp.EmojiUserError,
				`The IP address %q in %s is not a valid %s address`,
				rawIP,
				envKey,
				ipFamily.Describe(),
			)
			return nil, false
		}
		if desc, bad := ipnet.DescribeAddressIssue(ip); bad {
			ppfmt.Noticef(pp.EmojiUserError,
				`The IP address %q in %s is %s`,
				rawIP, envKey, desc,
			)
			return nil, false
		}
		if ipnet.IsNonGlobalUnicast(ip) {
			ppfmt.Noticef(pp.EmojiUserWarning,
				`The IP address %q in %s does not look like a global unicast address`,
				rawIP, envKey,
			)
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

// NewStaticEmpty creates an explicit-empty [protocol.Static] provider.
func NewStaticEmpty() Provider {
	return protocol.NewStatic("static.empty", nil)
}

// MustNewStatic creates a [protocol.Static] provider and panics if it fails.
func MustNewStatic(ipFamily ipnet.Family, raw string) Provider {
	var buf strings.Builder
	p, ok := NewStatic(pp.NewDefault(&buf), "IP_PROVIDER", ipFamily, raw)
	if !ok {
		panic(buf.String())
	}
	return p
}
