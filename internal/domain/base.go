// Package domain parses DNS domain names.
package domain

// A Domain represents a domain name to update.
type Domain interface {
	// DNSNameASCII gives a name suitable for accessing the Cloudflare API
	DNSNameASCII() string

	// String gives the canonical, round-trippable text form of the domain.
	String() string

	// Describe gives the most human-readable domain name that is still unambiguous
	Describe() string

	// HasStrictSuffix reports whether the domain is strictly under the suffix s.
	HasStrictSuffix(s Suffix) bool

	// Zones iterates from the smallest possible zone to largest ones (the root).
	Zones(yield func(Suffix) bool)
}
