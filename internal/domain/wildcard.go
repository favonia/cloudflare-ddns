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

// WildcardSplitter implements [Splitter] for a wildcard domain.
type WildcardSplitter struct {
	domain    string
	cursor    int
	exhausted bool
}

// Split constructs a [Splitter] for a wildcard domain.
func (w Wildcard) Split() Splitter {
	return &WildcardSplitter{
		domain:    string(w),
		cursor:    0,
		exhausted: false,
	}
}

// IsValid checks whether the cursor is still valid.
func (s *WildcardSplitter) IsValid() bool { return !s.exhausted }

// ZoneNameASCII gives the ASCII form of the current zone suffix.
func (s *WildcardSplitter) ZoneNameASCII() string { return s.domain[s.cursor:] }

// Next moves the cursor to the next spltting point.
// Call [WildcardSplitter.IsValid] to check whether the resulting cursor is valid.
func (s *WildcardSplitter) Next() {
	if s.cursor == len(s.domain) {
		s.exhausted = true
	} else {
		shift := strings.IndexRune(s.ZoneNameASCII(), '.')
		if shift == -1 {
			s.cursor = len(s.domain)
		} else {
			s.cursor += shift + 1
		}
	}
}
