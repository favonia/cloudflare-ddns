package domain

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
