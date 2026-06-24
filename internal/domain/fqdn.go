package domain

import "strings"

// FQDN is a fully qualified domain in its ASCII form.
type FQDN string

// DNSNameASCII retruns the ASCII form of the FQDN.
func (f FQDN) DNSNameASCII() string { return string(f) }

// String gives the canonical, round-trippable form of the FQDN. The empty
// FQDN is the root domain, rendered as ".".
func (f FQDN) String() string {
	if f == "" {
		return "."
	}
	return safelyToUnicode(string(f))
}

// Describe gives the human-readable form: String with an annotation where one
// helps, so an empty value never renders blank.
func (f FQDN) Describe() string {
	if f == "" {
		return ". (root)"
	}
	return f.String()
}

// Zones starts from a.b.c for the domain a.b.c.
func (f FQDN) Zones(yield func(ZoneNameASCII string) bool) {
	domain := string(f)
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
