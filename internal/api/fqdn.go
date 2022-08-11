package api

import "strings"

// FQDN is a fully qualified domain in its ASCII or Unicode (when unambiguous) form.
type FQDN string

func (f FQDN) DNSName() string { return string(f) }

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

func (f FQDN) Split() DomainSplitter {
	return &FQDNSplitter{
		domain:    string(f),
		cursor:    0,
		exhausted: false,
	}
}

func (s *FQDNSplitter) IsValid() bool    { return !s.exhausted }
func (s *FQDNSplitter) ZoneName() string { return s.domain[s.cursor:] }
func (s *FQDNSplitter) Next() {
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
