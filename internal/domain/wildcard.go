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

// Describe gives a human-readible representation of the wildcard domain.
func (w Wildcard) Describe() string {
	if string(w) == "" {
		return "*"
	}

	return "*." + safelyToUnicode(string(w))
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
