package ipnet

import (
	_ "embed"
	"fmt"
	"net/netip"
	"strings"
	"sync"
)

// Cloudflare publishes its IP ranges at:
//   - https://www.cloudflare.com/ips-v4/
//   - https://www.cloudflare.com/ips-v6/
//
// These embedded text files are the local copy of those ranges, used to reject
// detection results that report a Cloudflare egress IP rather than the client's
// real public IP. Cloudflare documents that proxied hostnames use shared
// Cloudflare IP ranges, and that trace-based proxy scenarios can report a
// Cloudflare egress IP instead of the client's real IP.
//
// This check is based on observed Cloudflare documentation rather than a
// guaranteed API contract. See the doc-watch configs for drift monitoring.

//go:embed cloudflare-ips-v4.txt
var cloudflareIPv4Text string

//go:embed cloudflare-ips-v6.txt
var cloudflareIPv6Text string

var (
	cloudflareRanges     []netip.Prefix //nolint:gochecknoglobals // lazy-init cache shared across all callers
	cloudflareRangesOnce sync.Once      //nolint:gochecknoglobals // guards cloudflareRanges init
)

func parseCloudflareRanges() []netip.Prefix {
	var ranges []netip.Prefix
	for line := range strings.SplitSeq(cloudflareIPv4Text+"\n"+cloudflareIPv6Text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ranges = append(ranges, netip.MustParsePrefix(line))
	}
	return ranges
}

func loadCloudflareRanges() {
	cloudflareRangesOnce.Do(func() {
		cloudflareRanges = parseCloudflareRanges()
	})
}

// IsCloudflareIP reports whether ip falls inside any of Cloudflare's
// published IP ranges. A detected IP inside these ranges is a Cloudflare
// egress IP, not a publishable client/public IP for DDNS.
func IsCloudflareIP(ip netip.Addr) bool {
	loadCloudflareRanges()
	ip = ip.Unmap()
	for _, prefix := range cloudflareRanges {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// RawIPRejecter validates a detected raw IP address (before normalization).
// If the IP should be rejected, RejectRawIP returns (false, reason).
// Otherwise it returns (true, "").
type RawIPRejecter interface {
	RejectRawIP(ip netip.Addr) (bool, string)
}

// CloudflareIPRejecter rejects IPs that fall inside Cloudflare's published
// IP ranges. Such IPs are Cloudflare egress/proxy IPs, not publishable
// client IPs for DDNS.
type CloudflareIPRejecter struct{}

// RejectRawIP rejects IPs inside Cloudflare's published ranges.
func (CloudflareIPRejecter) RejectRawIP(ip netip.Addr) (bool, string) {
	if IsCloudflareIP(ip) {
		return false, fmt.Sprintf(
			"The detected IP address %s is inside Cloudflare's own IP range and is not your real public IP",
			ip.String())
	}
	return true, ""
}
