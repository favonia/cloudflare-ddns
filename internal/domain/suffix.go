package domain

import (
	"errors"
	"strings"
)

// Suffix is a fully-qualified, dot-delimited domain tail in ASCII form, such as
// example.com or org, with "" representing the root. Unlike Domain, a Suffix can
// be a single label or the root, but it is never a wildcard.
type Suffix string

// ErrWildcardSuffix means the input is a validated wildcard and therefore not a usable suffix.
var ErrWildcardSuffix error = errors.New("wildcard cannot be a suffix")

// NewSuffix parses an ASCII-backed suffix using the same IDNA and boundary-dot
// normalization as New. It accepts single labels and the root ("." or ""), but
// rejects wildcards.
//
// ErrWildcardSuffix returns an empty suffix and the input's normalization;
// the wildcard's suffix has passed validation, so callers may handle it as a
// wildcard-specific advisory. Invalid wildcard suffixes return their validation
// error instead. All other errors return zero normalization, and any returned
// suffix is for diagnostics only, not evaluation.
func NewSuffix(input string) (Suffix, Normalization, error) {
	ascii, err := profileKeepingLeadingDots.ToASCII(input)
	normalized, normalization := normalizeBoundaryDots(ascii)

	if suffix, ok := wildcardSuffix(normalized); ok {
		_, wildcardErr := validateNormalizedWildcardSuffix(suffix)
		if wildcardErr != nil {
			return "", Normalization{}, wildcardErr
		}
		return "", normalization, ErrWildcardSuffix
	}

	if err != nil {
		return Suffix(normalized), Normalization{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: false,
		}, err
	}
	if strings.Contains(normalized, "..") {
		return "", Normalization{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: false,
		}, ErrEmptyInteriorLabel
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

// walkZonesASCII visits a canonical ASCII name and then its parents, ending at
// the single-label suffix without adding the root. An empty name visits the root
// once. A false yield result stops traversal immediately.
func walkZonesASCII(name string, yield func(Suffix) bool) {
	for {
		if !yield(Suffix(name)) {
			return
		}
		if i := strings.IndexRune(name, '.'); i == -1 {
			return
		} else {
			name = name[i+1:]
		}
	}
}
