package protocol

import (
	"context"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// File reads IP addresses from a file on every detection cycle.
type File struct {
	// ProviderName is the name of the detection protocol.
	ProviderName string

	// Path is the absolute path to the file containing IP addresses.
	Path string
}

// NewFile creates a file-backed provider.
func NewFile(providerName string, path string) File {
	return File{
		ProviderName: providerName,
		Path:         path,
	}
}

// Name of the detection protocol.
func (p File) Name() string {
	return p.ProviderName
}

// IsExplicitEmpty reports whether the provider intentionally clears the family.
// File providers are dynamic; the content may change between cycles.
func (p File) IsExplicitEmpty() bool {
	return false
}

// GetRawData reads the file, parses IP addresses, validates them for the
// requested family, and returns deterministic raw data.
func (p File) GetRawData(
	_ context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	lines, ok := file.ReadLines(ppfmt, p.Path)
	if !ok {
		return NewUnavailableDetectionResult()
	}

	ips := make([]netip.Addr, 0)
	for lineNum, rawIP := range lines {
		ip, err := netip.ParseAddr(rawIP)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError,
				"Failed to parse line %d (%q) of %s as an IP address", lineNum, rawIP, p.Path)
			return NewUnavailableDetectionResult()
		}
		if ip.Zone() != "" {
			ppfmt.Noticef(pp.EmojiUserError,
				"Line %d (%q) of %s has a zone identifier, which is not allowed",
				lineNum, rawIP, p.Path)
			return NewUnavailableDetectionResult()
		}
		if ipFamily == ipnet.IP6 && ip.Is4In6() {
			ppfmt.Noticef(pp.EmojiUserError,
				"Line %d (%q) of %s is an IPv4-mapped IPv6 address",
				lineNum, rawIP, p.Path)
			return NewUnavailableDetectionResult()
		}
		ip = ip.Unmap()
		if !ipFamily.Matches(ip) {
			ppfmt.Noticef(pp.EmojiUserError,
				"Line %d (%q) of %s is not a valid %s address",
				lineNum, rawIP, p.Path, ipFamily.Describe())
			return NewUnavailableDetectionResult()
		}
		if desc, bad := ipnet.DescribeAddressIssue(ip); bad {
			ppfmt.Noticef(pp.EmojiUserError,
				"Line %d (%q) of %s is %s",
				lineNum, rawIP, p.Path, desc)
			return NewUnavailableDetectionResult()
		}
		ips = append(ips, ip)
	}

	slices.SortFunc(ips, netip.Addr.Compare)
	ips = slices.Compact(ips)

	rawEntries := ipnet.LiftValidatedIPsToRawEntries(ips, defaultPrefixLen)

	normalized, ok := ipFamily.NormalizeDetectedRawEntries(ppfmt, rawEntries)
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(normalized)
}
