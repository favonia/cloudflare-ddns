package domain

import (
	"errors"
	"strings"
)

// Suffix is a fully-qualified, dot-delimited domain tail in ASCII form, such as
// example.com or org, with "" representing the root. Unlike Domain, a Suffix can
// be a single label or the root, but it is never a wildcard.
type Suffix string

// ErrWildcardSuffix means a suffix argument was a wildcard. A wildcard has no
// strict subdomains, so it cannot be a suffix.
var ErrWildcardSuffix error = errors.New("wildcard cannot be a suffix")

// NewSuffix parses a domain suffix. It is its own parser, parallel to New
// (not layered on it): it is looser — it accepts a single label (org) and the
// root (. or "") — and stricter — it rejects any wildcard (* or *.example.org).
// It applies the same IDNA normalization New uses for the ASCII form.
func NewSuffix(input string) (Suffix, Normalization, error) {
	ascii, err := profileKeepingLeadingDots.ToASCII(input)
	normalized, normalization := normalizeBoundary(ascii)

	if suffix, ok := strings.CutPrefix(normalized, "*."); ok {
		_, suffixNormalization, suffixErr := newWildcard(suffix, normalization, strings.HasPrefix(suffix, "."))
		if suffixErr != nil && !errors.Is(suffixErr, ErrTooFewLabels) {
			return "", Normalization{
				RemovedLeadingDots:       false,
				RemovedExtraTrailingDots: false,
			}, suffixErr
		}
		return "", normalization.combine(suffixNormalization), ErrWildcardSuffix
	}
	if normalized == "*" {
		return "", normalization, ErrWildcardSuffix
	}

	if err != nil {
		return Suffix(normalized), Normalization{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: false,
		}, err
	}
	if hasEmptyInteriorLabel(normalized) {
		return "", Normalization{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: false,
		}, newEmptyInteriorLabelError(false)
	}
	return Suffix(normalized), normalization, nil
}

// DNSNameASCII gives the ASCII name used for matching, the Cloudflare zone name,
// and cache keys. The root suffix yields "".
func (s Suffix) DNSNameASCII() string { return string(s) }

// String gives the canonical, round-trippable text form. The root suffix
// renders as ".".
func (s Suffix) String() string {
	if s == "" {
		return "."
	}
	return safelyToUnicode(string(s))
}

// HasStrictSuffix reports whether s is strictly under t — t is a proper suffix of
// s. b.c HasStrictSuffix c is true; c HasStrictSuffix c is false.
func (s Suffix) HasStrictSuffix(t Suffix) bool {
	return hasStrictSuffixASCII(s.DNSNameASCII(), t.DNSNameASCII())
}

// hasStrictSuffixASCII reports whether suffix is a proper (strict) dot-delimited
// suffix of s, both in ASCII form. The root suffix "" is a strict suffix of every
// non-root name; the dot-boundary arithmetic below cannot express that, so it is
// special-cased.
func hasStrictSuffixASCII(s, suffix string) bool {
	if suffix == "" {
		return s != ""
	}
	return strings.HasSuffix(s, suffix) && len(s) > len(suffix) && s[len(s)-len(suffix)-1] == '.'
}
