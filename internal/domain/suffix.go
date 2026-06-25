package domain

import "strings"

// Suffix is a fully-qualified, dot-delimited domain tail in ASCII form, such as
// example.com or org, with "" representing the root. Unlike Domain, a Suffix can
// be a single label or the root, but it is never a wildcard.
type Suffix string

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

// HasStrictSuffix reports whether s is strictly under t — t is a proper suffix of
// s. b.c HasStrictSuffix c is true; c HasStrictSuffix c is false.
func (s Suffix) HasStrictSuffix(t Suffix) bool {
	return hasStrictSuffixASCII(s.DNSNameASCII(), t.DNSNameASCII())
}
