package domain

import "strings"

// FQDN is a fully qualified domain in its ASCII form.
type FQDN string

func (f FQDN) DNSNameASCII() string { return string(f) }

func (f FQDN) Describe() string {
	best, ok := safelyToUnicode(string(f))
	if !ok {
		// use the unconverted string if the conversation failed
		return string(f)
	}

	return best
}

type FQDNSplitter struct {
	domain    string
	cursor    int
	exhausted bool
}

func (f FQDN) Split() Splitter {
	return &FQDNSplitter{
		domain:    string(f),
		cursor:    0,
		exhausted: false,
	}
}

func (s *FQDNSplitter) IsValid() bool         { return !s.exhausted }
func (s *FQDNSplitter) ZoneNameASCII() string { return s.domain[s.cursor:] }
func (s *FQDNSplitter) Next() {
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
