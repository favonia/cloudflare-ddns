package protocol

import (
	"context"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// traceMaxReadLength is the maximum response size for Cloudflare trace endpoints.
// A real trace response is ~300 bytes; this limit guards against unexpected payloads.
const traceMaxReadLength int64 = 4096

// traceFields holds the trace response fields that the detector validates.
type traceFields struct {
	h    string // host identifier echoed by the trace endpoint
	ip   string // detected IP address
	warp string // WARP routing status
}

// parseTraceBody parses a Cloudflare trace response (key=value lines)
// and extracts only the fields we validate.
func parseTraceBody(body []byte) traceFields {
	var fields traceFields
	for line := range strings.SplitSeq(string(body), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "h":
			fields.h = value
		case "ip":
			fields.ip = value
		case "warp":
			fields.warp = value
		}
	}
	return fields
}

// CloudflareTrace implements detection via Cloudflare's /cdn-cgi/trace endpoint
// with hard validation of h, warp, and ip fields.
//
// Validation rationale:
//   - This detector returns a publishable client/public IP for DDNS use.
//   - The h field is a conservative integrity check on the response source,
//     based on observed endpoint behavior rather than a strong public field
//     specification.
//   - warp=on indicates WARP is routing the connection, so the reported ip
//     is a Cloudflare egress IP, not the client's real IP.
//   - An ip inside Cloudflare's published ranges indicates a proxy scenario
//     where the reported ip is not the client's real public IP.
type CloudflareTrace struct {
	ProviderName string                  // name of the detection protocol
	URL          map[ipnet.Family]string // trace endpoint URL per family
}

// Name of the detection protocol.
func (p CloudflareTrace) Name() string { return p.ProviderName }

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (CloudflareTrace) IsExplicitEmpty() bool { return false }

// GetRawData detects the IP address by parsing and validating a Cloudflare
// trace response.
func (p CloudflareTrace) GetRawData(
	ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	traceURL, found := p.URL[ipFamily]
	if !found {
		ppfmt.Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", ipFamily.Describe())
		return NewUnavailableDetectionResult()
	}

	c := httpCore{ //nolint:exhaustruct // GET request; no additional headers or body needed.
		ipFamily:      ipFamily,
		url:           traceURL,
		method:        http.MethodGet,
		maxReadLength: traceMaxReadLength,
	}

	body, ok := c.getBody(ctx, ppfmt)
	if !ok {
		return NewUnavailableDetectionResult()
	}

	fields := parseTraceBody(body)

	parsedURL, err := url.Parse(traceURL)
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible, "Failed to parse the provider URL %q: %v", traceURL, err)
		return NewUnavailableDetectionResult()
	}

	// Validate h: integrity check on the response source.
	// A missing h is unexpected but tolerated; a mismatched h is a hard failure.
	switch {
	case fields.h == "":
		ppfmt.Noticef(pp.EmojiImpossible,
			"The response of %q does not contain an h (host) field; please report this at %s",
			traceURL, pp.IssueReportingURL)
	case fields.h != parsedURL.Host:
		ppfmt.Noticef(pp.EmojiImpossible,
			"The h field %q in the response of %q does not match the expected host %q; please report this at %s",
			fields.h, traceURL, parsedURL.Host, pp.IssueReportingURL)
		return NewUnavailableDetectionResult()
	}

	// Validate warp: reject warp=on because the reported ip would be a
	// Cloudflare-routed egress identity, not the client's real public IP.
	// A missing warp is unexpected but tolerated.
	switch fields.warp {
	case "":
		ppfmt.Noticef(pp.EmojiImpossible,
			"The response of %q does not contain a warp field; please report this at %s",
			traceURL, pp.IssueReportingURL)
	case "on":
		ppfmt.Noticef(pp.EmojiError,
			"The response of %q has warp=on; the detected IP is a Cloudflare WARP egress IP, not your real public IP",
			traceURL)
		return NewUnavailableDetectionResult()
	}

	// Validate ip: must be present, parseable, and not a Cloudflare egress/proxy IP.
	if fields.ip == "" {
		ppfmt.Noticef(pp.EmojiError, "The response of %q does not contain an ip field", traceURL)
		return NewUnavailableDetectionResult()
	}
	ip, err := netip.ParseAddr(fields.ip)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to parse the IP address in the response of %q (%q)", traceURL, fields.ip)
		return NewUnavailableDetectionResult()
	}
	if ipnet.IsCloudflareIP(ip) {
		ppfmt.Noticef(pp.EmojiError,
			"The detected IP address %s is inside Cloudflare's own IP range and is not your real public IP",
			ip.String())
		return NewUnavailableDetectionResult()
	}

	rawEntries, ok := NormalizeDetectedRawIPs(ppfmt, ipFamily, defaultPrefixLen, []netip.Addr{ip})
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(rawEntries)
}
