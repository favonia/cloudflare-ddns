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
func NewStatic(ppfmt pp.PP, envKey string, ipFamily ipnet.Family, defaultPrefixLen int, raw string) (Provider, bool) {
	ips := make([]netip.Addr, 0)
	entryNum := 0
	for rawIP := range strings.SplitSeq(raw, ",") {
		entryNum++
		rawIP = strings.TrimSpace(rawIP)

		if rawIP == "" {
			ppfmt.Noticef(pp.EmojiUserError,
				`The %s entry of %s is empty (check for extra commas)`, pp.Ordinal(entryNum), envKey)
			return nil, false
		}

		ip, err := netip.ParseAddr(rawIP)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError,
				`Failed to parse the %s entry (%q) of %s as an IP address`, pp.Ordinal(entryNum), rawIP, envKey)
			return nil, false
		}
		normalized, issue, is4in6Hint, ok := ipnet.ValidateAndNormalizeIP(ipFamily, ip)
		if !ok {
			ppfmt.Noticef(pp.EmojiUserError,
				`The %s entry (%q) of %s is %s`,
				pp.Ordinal(entryNum), rawIP, envKey, issue)
			if is4in6Hint {
				ppfmt.InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
					"An IPv4-mapped IPv6 address is an IPv4 address in disguise. "+
						"It cannot be used for routing IPv6 traffic. "+
						"If you need to use it for DNS, please open an issue at %s",
					pp.IssueReportingURL)
			}
			return nil, false
		}
		ips = append(ips, normalized)
	}

	// Make the explicit-input provider deterministic before it enters the pipeline.
	slices.SortFunc(ips, netip.Addr.Compare)
	ips = slices.Compact(ips)

	rawIPs := make([]string, 0, len(ips))
	for _, ip := range ips {
		rawIPs = append(rawIPs, ip.String())
	}
	return protocol.NewStatic(
		"static:"+strings.Join(rawIPs, ","),
		ipnet.LiftValidatedIPsToRawEntries(ips, defaultPrefixLen),
	), true
}

// NewStaticEmpty creates an explicit-empty [protocol.Static] provider.
func NewStaticEmpty() Provider {
	return protocol.NewStatic("static.empty", nil)
}

// MustNewStatic creates a [protocol.Static] provider and panics if it fails.
func MustNewStatic(ipFamily ipnet.Family, defaultPrefixLen int, raw string) Provider {
	var buf strings.Builder
	p, ok := NewStatic(pp.NewDefault(&buf), "IP_PROVIDER", ipFamily, defaultPrefixLen, raw)
	if !ok {
		panic(buf.String())
	}
	return p
}
