// Package domain parses DNS domain names.
package domain

// A Splitter enumerates all possible zones of a domain by moving a cursor
// from the start of the domain's name to its end.
type Splitter interface {
	// IsValid checks whether the cursor is still valid.
	IsValid() bool

	// ZoneNameASCII gives the suffix after the cursor as a possible zone, if the cursor is still valid.
	ZoneNameASCII() string

	// Next moves the cursor to the next possible splitting point, which might end up being invalid.
	Next() Splitter
}

// A Domain represents a domain name to update.
type Domain interface {
	// DNSNameASCII gives a name suitable for accessing the Cloudflare API
	DNSNameASCII() string

	// Describe gives the most human-readable domain name that is still unambiguous
	Describe() string

	// Split gives a Splitter that can be used to find zones
	Split() Splitter
}
