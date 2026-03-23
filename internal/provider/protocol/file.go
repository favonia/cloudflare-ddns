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
		normalized, issue, is4in6Hint, ok := ipnet.ValidateAndNormalizeIP(ipFamily, ip)
		if !ok {
			ppfmt.Noticef(pp.EmojiUserError,
				"Line %d (%q) of %s is %s", lineNum, rawIP, p.Path, issue)
			if is4in6Hint {
				ppfmt.InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
					"An IPv4-mapped IPv6 address is an IPv4 address in disguise. "+
						"It cannot be used for routing IPv6 traffic. "+
						"If you need to use it for DNS, please open an issue at %s",
					pp.IssueReportingURL)
			}
			return NewUnavailableDetectionResult()
		}
		ips = append(ips, normalized)
	}

	slices.SortFunc(ips, netip.Addr.Compare)
	ips = slices.Compact(ips)

	return NewKnownDetectionResult(ipnet.LiftValidatedIPsToRawEntries(ips, defaultPrefixLen))
}
