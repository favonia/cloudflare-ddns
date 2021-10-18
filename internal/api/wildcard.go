package api

import "strings"

// Wildcard is a fully qualified zone name in its ASCII or Unicode (when unambiguous) form,
// represnting the wildcard domain name under the zone.
type Wildcard string

func (w Wildcard) String() string {
	if string(w) == "" {
		return "*"
	}

	return "*." + string(w)
}

func (w Wildcard) Describe() string {
	best, ok := safelyToUnicode(string(w))
	if !ok {
		// use the unconverted string if the conversation failed
		return "*." + string(w)
	}

	return "*." + best
}

type WildcardSplitter struct {
	domain       string
	prefixCursor int
	suffixCursor int
	exhausted    bool
}

func (w Wildcard) Split() DomainSplitter {
	return &WildcardSplitter{
		domain:       string(w),
		suffixCursor: 0,
		prefixCursor: 0,
		exhausted:    false,
	}
}

func (s *WildcardSplitter) IsValid() bool         { return !s.exhausted }
func (s *WildcardSplitter) ZoneNameASCII() string { return s.domain[s.suffixCursor:] }
func (s *WildcardSplitter) DNSNameASCII() string {
	if s.suffixCursor <= 0 {
		return "*"
	}
	return "*." + s.domain[:s.prefixCursor]
}

func (s *WildcardSplitter) Next() {
	if s.suffixCursor == len(s.domain) {
		s.exhausted = true
	} else {
		shift := strings.IndexRune(s.ZoneNameASCII(), '.')
		if shift == -1 {
			s.prefixCursor = len(s.domain)
			s.suffixCursor = len(s.domain)
		} else {
			s.suffixCursor += shift + 1
			s.prefixCursor = s.suffixCursor - 1
		}
	}
}
