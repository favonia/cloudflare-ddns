package protocol

import (
	"context"
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

// Name of the detection protocol.
func (p File) Name() string {
	return p.ProviderName
}

// IsExplicitEmpty reports whether the provider intentionally clears the family.
// File providers are dynamic; the content may change between cycles.
func (p File) IsExplicitEmpty() bool {
	return false
}

// GetRawData reads the file, parses IP addresses or IP addresses in CIDR notation, validates them
// for the requested family, and returns deterministic raw data.
func (p File) GetRawData(
	_ context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	lines, ok := file.ReadLines(ppfmt, p.Path)
	if !ok {
		return NewUnavailableDetectionResult()
	}

	entries := make([]ipnet.RawEntry, 0)
	displayPath := pp.QuoteIfUnsafeInSentence(p.Path)
	for lineNum, raw := range lines {
		entry, err := ipnet.ParseRawEntry(raw, defaultPrefixLen)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError,
				"Failed to parse line %d (%q) in %s as an IP address or an IP address in CIDR notation",
				lineNum, raw, displayPath)
			return NewUnavailableDetectionResult()
		}

		// Per-line validation with contextual error messages.
		normalized, problem, is4in6Hint, ok := ipnet.NormalizeRawEntryIP(ipFamily, entry)
		if !ok {
			ppfmt.Noticef(pp.EmojiUserError,
				"Line %d (%q) in %s %s", lineNum, raw, displayPath, problem)
			ipnet.Emit4in6Hint(ppfmt, is4in6Hint)
			return NewUnavailableDetectionResult()
		}
		entries = append(entries, normalized)
	}

	slices.SortFunc(entries, ipnet.RawEntry.Compare)
	entries = slices.Compact(entries)
	if len(entries) == 0 {
		ppfmt.Noticef(pp.EmojiUserError,
			"No IP addresses were found in %s", displayPath)
		return NewUnavailableDetectionResult()
	}

	if len(entries) > 1 {
		ppfmt.InfoOncef(pp.MessageExperimentalMultipleAddressesFile, pp.EmojiExperimental,
			"The file contains multiple addresses; this multi-address support is experimental (available since version 1.16.0)")
	}

	return NewKnownDetectionResult(entries)
}
