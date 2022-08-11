package api

import "strings"

// Wildcard is a fully qualified zone name in its ASCII form, represnting the wildcard domain name
// under the zone. For example, Wildcard("example.org") represents "*.example.org".
type Wildcard string

func (w Wildcard) DNSName() string {
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
	domain    string
	cursor    int
	exhausted bool
}

func (w Wildcard) Split() DomainSplitter {
	return &WildcardSplitter{
		domain:    string(w),
		cursor:    0,
		exhausted: false,
	}
}

func (s *WildcardSplitter) IsValid() bool    { return !s.exhausted }
func (s *WildcardSplitter) ZoneName() string { return s.domain[s.cursor:] }
func (s *WildcardSplitter) Next() {
	if s.cursor == len(s.domain) {
		s.exhausted = true
	} else {
		shift := strings.IndexRune(s.ZoneName(), '.')
		if shift == -1 {
			s.cursor = len(s.domain)
		} else {
			s.cursor += shift + 1
		}
	}
}
