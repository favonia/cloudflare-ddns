package protocol

import (
	"context"
	"net/http"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func getRawEntriesFromHTTP(
	ctx context.Context, ppfmt pp.PP,
	transportIPFamily ipnet.Family, url string,
	ipFamily ipnet.Family, defaultPrefixLen int,
) ([]ipnet.RawEntry, bool) {
	c := httpCore{
		ipFamily:          transportIPFamily,
		url:               url,
		method:            http.MethodGet,
		additionalHeaders: nil,
		requestBody:       nil,
		maxReadLength:     0, // use default limit
	}

	body, ok := c.getBody(ctx, ppfmt)
	if !ok {
		return nil, false
	}

	entries := make([]ipnet.RawEntry, 0)
	for lineNum, raw := range file.ProcessLines(string(body)) {
		entry, err := ipnet.ParseRawEntry(raw, defaultPrefixLen)
		if err != nil {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to parse line %d in the response from %q (%q) as an IP address or an IP address in CIDR notation",
				lineNum, url, raw)
			return nil, false
		}

		normalized, problem, is4in6Hint, ok := ipnet.NormalizeRawEntryIP(ipFamily, entry)
		if !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Line %d in the response from %q (%q) %s", lineNum, url, raw, problem)
			ipnet.Emit4in6Hint(ppfmt, is4in6Hint)
			return nil, false
		}
		entries = append(entries, normalized)
	}

	slices.SortFunc(entries, ipnet.RawEntry.Compare)
	entries = slices.Compact(entries)

	return entries, true
}

// HTTP represents a generic detection protocol to use an HTTP response directly.
type HTTP struct {
	ProviderName            string                  // name of the protocol
	URL                     map[ipnet.Family]string // URL of the page for detection
	ForcedTransportIPFamily *ipnet.Family
	// ForcedTransportIPFamily optionally overrides the network family used for
	// the HTTP connection. When absent, GetIPs uses the requested family itself.
}

// Name of the detection protocol.
func (p HTTP) Name() string {
	return p.ProviderName
}

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (HTTP) IsExplicitEmpty() bool {
	return false
}

// GetRawData detects the IP address by using the HTTP response directly.
func (p HTTP) GetRawData(
	ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	url, found := p.URL[ipFamily]
	if !found {
		ppfmt.Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", ipFamily.Describe())
		return NewUnavailableDetectionResult()
	}

	transportIP := ipFamily
	if p.ForcedTransportIPFamily != nil {
		transportIP = *p.ForcedTransportIPFamily
	}

	rawEntries, ok := getRawEntriesFromHTTP(ctx, ppfmt, transportIP, url, ipFamily, defaultPrefixLen)
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(rawEntries)
}
