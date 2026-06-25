package domain

import "strings"

// Wildcard is a fully qualified zone name in its ASCII form, represnting the wildcard domain name
// under the zone. For example, Wildcard("example.org") represents *.example.org.
type Wildcard string

// DNSNameASCII retruns the ASCII form of the wildcard domain.
func (w Wildcard) DNSNameASCII() string {
	if string(w) == "" {
		return "*"
	}

	return "*." + string(w)
}

// String gives the canonical, round-trippable form of the wildcard domain.
func (w Wildcard) String() string {
	if string(w) == "" {
		return "*"
	}

	return "*." + safelyToUnicode(string(w))
}

// Describe gives the human-readable form. The wildcard carries no annotation,
// so it equals String.
func (w Wildcard) Describe() string {
	return w.String()
}

// HasStrictSuffix reports whether the wildcard domain is strictly under the
// suffix s. Wildcard("example.org") (i.e. *.example.org) is strictly under
// example.org.
func (w Wildcard) HasStrictSuffix(s Suffix) bool {
	return hasStrictSuffixASCII(w.DNSNameASCII(), s.DNSNameASCII())
}

// Zones starts from a.b.c for the wildcard domain *.a.b.c.
func (w Wildcard) Zones(yield func(ZoneNameASCII string) bool) {
	domain := string(w)
	for {
		if !yield(domain) {
			return
		}
		if i := strings.IndexRune(domain, '.'); i == -1 {
			return
		} else {
			domain = domain[i+1:]
		}
	}
}
