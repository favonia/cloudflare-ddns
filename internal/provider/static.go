package provider

import (
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewStatic creates a [protocol.Static] provider.
func NewStatic(ppfmt pp.PP, envKey string, ipFamily ipnet.Family, defaultPrefixLen int, raw string) (Provider, bool) {
	if strings.Trim(raw, ", \t\r\n") == "" {
		ppfmt.Noticef(
			pp.EmojiUserError,
			`%s=static: must be followed by at least one IP address`,
			envKey,
		)
		return nil, false
	}

	entries := make([]ipnet.RawEntry, 0)
	rawEntries := strings.Split(raw, ",")
	for i, rawEntry := range rawEntries {
		entryNum := i + 1
		rawEntry = strings.TrimSpace(rawEntry)

		if rawEntry == "" {
			if i == len(rawEntries)-1 && i > 0 {
				continue
			}
			ppfmt.Noticef(pp.EmojiUserError,
				`The %s entry in %s is empty (check for extra commas)`, pp.Ordinal(entryNum), envKey)
			return nil, false
		}

		entry, err := ipnet.ParseRawEntry(rawEntry, defaultPrefixLen)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError,
				`Failed to parse the %s entry (%q) in %s as an IP address or an IP address in CIDR notation`,
				pp.Ordinal(entryNum), rawEntry, envKey)
			return nil, false
		}

		// Per-entry validation with contextual error messages.
		normalized, problem, is4in6Hint, ok := ipnet.NormalizeRawEntryIP(ipFamily, entry)
		if !ok {
			ppfmt.Noticef(pp.EmojiUserError,
				`The %s entry (%q) in %s %s`,
				pp.Ordinal(entryNum), rawEntry, envKey, problem)
			ipnet.Emit4in6Hint(ppfmt, is4in6Hint)
			return nil, false
		}
		entries = append(entries, normalized)
	}

	// Make the explicit-input provider deterministic before it enters the pipeline.
	slices.SortFunc(entries, ipnet.RawEntry.Compare)
	entries = slices.Compact(entries)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Describe(defaultPrefixLen))
	}
	return protocol.NewStatic(
		"static:"+strings.Join(names, ","),
		entries,
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
