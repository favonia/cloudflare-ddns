package domain

// A Splitter enumerates all possible zones from a domain.
type Splitter interface {
	// IsValid checks whether the current splitting point is still valid
	IsValid() bool
	// ZoneNameASCII gives the suffix (the zone), when it is still valid
	ZoneNameASCII() string
	// Next moves to the next possible splitting point, which might end up being invalid
	Next()
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
