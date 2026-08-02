package protocol

import (
	"context"
	"fmt"
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

type traceAttemptStatus uint8

const (
	traceAttemptUnstarted traceAttemptStatus = iota
	traceAttemptSucceeded
	traceAttemptFailed
	traceAttemptCanceled
)

type traceWarningKind uint8

const (
	traceWarningMissingH traceWarningKind = iota
	traceWarningMissingWarp
)

type traceFailureKind uint8

const (
	traceFailureRequest traceFailureKind = iota
	traceFailureInvalidEndpoint
	traceFailureMismatchedH
	traceFailureWarpOn
	traceFailureMissingIP
	traceFailureUnparseableIP
	traceFailureCloudflareIP
	traceFailureInvalidDetectedIP
)

type traceFailure struct {
	kind             traceFailureKind
	cause            error
	observed         string
	expected         string
	problem          string
	wantsMapped4Hint bool
}

type traceAttemptResult struct {
	status   traceAttemptStatus
	rawData  DetectionResult
	warnings []traceWarningKind
	failure  traceFailure
}

func attemptCloudflareTrace(
	ctx context.Context,
	traceURL string,
	ipFamily ipnet.Family,
	defaultPrefixLen int,
) traceAttemptResult {
	parsedURL, err := url.Parse(traceURL)
	if err != nil {
		return traceAttemptResult{
			status:   traceAttemptFailed,
			rawData:  NewUnavailableDetectionResult(),
			warnings: nil,
			failure: traceFailure{ //nolint:exhaustruct // This failure carries only its parse error.
				kind:  traceFailureInvalidEndpoint,
				cause: err,
			},
		}
	}

	c := httpCore{ //nolint:exhaustruct // GET request; no additional headers or body needed.
		ipFamily:      ipFamily,
		url:           traceURL,
		method:        http.MethodGet,
		maxReadLength: traceMaxReadLength,
	}
	body, err := c.getBodyOnce(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return traceAttemptResult{ //nolint:exhaustruct // Cancellation is not a definite failure.
				status:  traceAttemptCanceled,
				rawData: NewUnavailableDetectionResult(),
			}
		}
		return traceAttemptResult{
			status:   traceAttemptFailed,
			rawData:  NewUnavailableDetectionResult(),
			warnings: nil,
			failure: traceFailure{ //nolint:exhaustruct // This failure carries only its request error.
				kind:  traceFailureRequest,
				cause: err,
			},
		}
	}

	fields := parseTraceBody(body)
	var warnings []traceWarningKind

	// Validate h: integrity check on the response source.
	// A missing h is unexpected but tolerated; a mismatched h is a hard failure.
	switch {
	case fields.h == "":
		warnings = append(warnings, traceWarningMissingH)
	case fields.h != parsedURL.Host:
		return traceAttemptResult{
			status:   traceAttemptFailed,
			rawData:  NewUnavailableDetectionResult(),
			warnings: warnings,
			failure: traceFailure{ //nolint:exhaustruct // This failure compares the observed and expected hosts.
				kind:     traceFailureMismatchedH,
				observed: fields.h,
				expected: parsedURL.Host,
			},
		}
	}

	// Validate warp: reject warp=on because the reported ip would be a
	// Cloudflare-routed egress identity, not the client's real public IP.
	// A missing warp is unexpected but tolerated.
	switch fields.warp {
	case "":
		warnings = append(warnings, traceWarningMissingWarp)
	case "on":
		return traceAttemptResult{
			status:   traceAttemptFailed,
			rawData:  NewUnavailableDetectionResult(),
			warnings: warnings,
			failure: traceFailure{ //nolint:exhaustruct // The observed WARP state is sufficient.
				kind:     traceFailureWarpOn,
				observed: fields.warp,
			},
		}
	}

	// Validate ip: must be present, parseable, and not a Cloudflare egress/proxy IP.
	if fields.ip == "" {
		return traceAttemptResult{
			status:   traceAttemptFailed,
			rawData:  NewUnavailableDetectionResult(),
			warnings: warnings,
			failure:  traceFailure{kind: traceFailureMissingIP}, //nolint:exhaustruct // No supporting fields needed.
		}
	}
	ip, err := netip.ParseAddr(fields.ip)
	if err != nil {
		return traceAttemptResult{
			status:   traceAttemptFailed,
			rawData:  NewUnavailableDetectionResult(),
			warnings: warnings,
			failure: traceFailure{ //nolint:exhaustruct // The unparseable value is sufficient.
				kind:     traceFailureUnparseableIP,
				observed: fields.ip,
			},
		}
	}
	if ipnet.IsCloudflareIP(ip) {
		return traceAttemptResult{
			status:   traceAttemptFailed,
			rawData:  NewUnavailableDetectionResult(),
			warnings: warnings,
			failure: traceFailure{ //nolint:exhaustruct // The detected Cloudflare address is sufficient.
				kind:     traceFailureCloudflareIP,
				observed: ip.String(),
			},
		}
	}

	normalized, _, problem, wantsMapped4Hint, ok := ipnet.ValidateAndNormalizeIP(ipFamily, ip)
	if !ok {
		return traceAttemptResult{
			status:   traceAttemptFailed,
			rawData:  NewUnavailableDetectionResult(),
			warnings: warnings,
			failure: traceFailure{ //nolint:exhaustruct // Validation supplies the problem and optional hint flag.
				kind:             traceFailureInvalidDetectedIP,
				observed:         ip.String(),
				problem:          problem,
				wantsMapped4Hint: wantsMapped4Hint,
			},
		}
	}

	return traceAttemptResult{
		status: traceAttemptSucceeded,
		rawData: NewKnownDetectionResult(
			ipnet.LiftValidatedIPsToRawEntries([]netip.Addr{normalized}, defaultPrefixLen),
		),
		warnings: warnings,
		failure:  traceFailure{}, //nolint:exhaustruct // The zero value means no failure.
	}
}

func describeCloudflareTraceFailure(failure traceFailure) string {
	switch failure.kind {
	case traceFailureRequest:
		return failure.cause.Error()
	case traceFailureInvalidEndpoint:
		return fmt.Sprintf("failed to parse the provider URL: %v", failure.cause)
	case traceFailureMismatchedH:
		return fmt.Sprintf(
			"the h field %q does not match the expected host %q; please report this at %s",
			failure.observed, failure.expected, pp.IssueReportingURL,
		)
	case traceFailureWarpOn:
		return "the response has warp=on; the detected IP is a Cloudflare WARP egress IP, not your real public IP"
	case traceFailureMissingIP:
		return "the response does not contain an ip field"
	case traceFailureUnparseableIP:
		return fmt.Sprintf("failed to parse the IP address %q", failure.observed)
	case traceFailureCloudflareIP:
		return fmt.Sprintf(
			"the detected IP address %s is inside Cloudflare's own IP range and is not your real public IP",
			failure.observed,
		)
	case traceFailureInvalidDetectedIP:
		return fmt.Sprintf("the detected IP address %s %s", failure.observed, failure.problem)
	default:
		return "unknown Cloudflare trace failure"
	}
}

func reportCloudflareTraceWarnings(ppfmt pp.PP, traceURL string, warnings []traceWarningKind) {
	displayTraceURL := pp.QuoteIfUnsafeInSentence(traceURL)
	for _, warning := range warnings {
		switch warning {
		case traceWarningMissingH:
			ppfmt.Noticef(pp.EmojiImpossible,
				"The response from %s does not contain an h (host) field; please report this at %s",
				displayTraceURL, pp.IssueReportingURL)
		case traceWarningMissingWarp:
			ppfmt.Noticef(pp.EmojiImpossible,
				"The response from %s does not contain a warp field; please report this at %s",
				displayTraceURL, pp.IssueReportingURL)
		}
	}
}

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

	result := attemptCloudflareTrace(ctx, traceURL, ipFamily, defaultPrefixLen)
	reportCloudflareTraceWarnings(ppfmt, traceURL, result.warnings)
	if result.status == traceAttemptSucceeded {
		return result.rawData
	}
	if result.status == traceAttemptCanceled {
		return NewUnavailableDetectionResult()
	}

	displayTraceURL := pp.QuoteIfUnsafeInSentence(traceURL)
	emoji := pp.EmojiError
	if result.failure.kind == traceFailureInvalidEndpoint || result.failure.kind == traceFailureMismatchedH {
		emoji = pp.EmojiImpossible
	}
	ppfmt.Noticef(emoji, "Cloudflare trace detection from %s failed: %s",
		displayTraceURL, describeCloudflareTraceFailure(result.failure))
	ipnet.Emit4in6Hint(ppfmt, result.failure.wantsMapped4Hint)
	return result.rawData
}
