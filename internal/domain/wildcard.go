package domain

import "strings"

// Wildcard is a fully qualified zone name in its ASCII form, represnting the wildcard domain name
// under the zone. For example, Wildcard("example.org") represents "*.example.org".
type Wildcard string

func (w Wildcard) DNSNameASCII() string {
	if string(w) == "" {
		return "*"
	}

	return "*." + string(w)
}

func (w Wildcard) Describe() string {
	if string(w) == "" {
		return "*"
	}

	return "*." + safelyToUnicode(string(w))
}

type WildcardSplitter struct {
	domain    string
	cursor    int
	exhausted bool
}

func (w Wildcard) Split() Splitter {
	return &WildcardSplitter{
		domain:    string(w),
		cursor:    0,
		exhausted: false,
	}
}

func (s *WildcardSplitter) IsValid() bool         { return !s.exhausted }
func (s *WildcardSplitter) ZoneNameASCII() string { return s.domain[s.cursor:] }
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
