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

// FQDNSplitter implements [Splitter] for a [FQDN].
type FQDNSplitter struct {
	domain    string
	cursor    int
	exhausted bool
}

// Split constructs a [FQDNSplitter] for the FQDN.
func (f FQDN) Split() Splitter {
	return FQDNSplitter{
		domain:    string(f),
		cursor:    0,
		exhausted: false,
	}
}

// IsValid checks whether the cursor is still valid.
func (s FQDNSplitter) IsValid() bool { return !s.exhausted }

// ZoneNameASCII gives the ASCII form of the current zone suffix.
func (s FQDNSplitter) ZoneNameASCII() string { return s.domain[s.cursor:] }

// Next moves the cursor to the next spltting point.
// Call [IsValid] to check whether the resulting cursor is valid.
func (s FQDNSplitter) Next() Splitter {
	next := s
	if s.cursor == len(s.domain) {
		next.exhausted = true
	} else {
		shift := strings.IndexRune(s.ZoneNameASCII(), '.')
		if shift == -1 {
			next.cursor = len(s.domain)
		} else {
			next.cursor += shift + 1
		}
	}
	return next
}
