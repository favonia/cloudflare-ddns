package domain

import "strings"

// FQDN is a fully qualified domain in its ASCII form.
type FQDN string

// DNSNameASCII retruns the ASCII form of the FQDN.
func (f FQDN) DNSNameASCII() string { return string(f) }

// Describe gives a human-readible representation of the FQDN.
func (f FQDN) Describe() string {
	return safelyToUnicode(string(f))
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
