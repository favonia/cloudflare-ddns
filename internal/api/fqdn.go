package api

import (
	"sort"
	"strings"

	"golang.org/x/net/idna"
)

type FQDN string

func (f FQDN) String() string {
	return string(f)
}

func NewFQDN(domain string) FQDN {
	// Remove the final period for consistency
	domain = strings.TrimSuffix(domain, ".")

	// Process Punycode
	normalized, err := idna.ToUnicode(domain)
	if err != nil {
		// Something appears to be wrong, but we can live with it
		return FQDN(domain)
	}

	return FQDN(normalized)
}

type FQDNSlice []FQDN

func (x FQDNSlice) Len() int           { return len(x) }
func (x FQDNSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x FQDNSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func SortFQDNs(s []FQDN) {
	sort.Sort(FQDNSlice(s))
}

type FQDNSplitter struct {
	domain    string
	cursor    int
	exhausted bool
}

func NewFQDNSplitter(domain FQDN) *FQDNSplitter {
	return &FQDNSplitter{
		domain:    domain.String(),
		cursor:    0,
		exhausted: false,
	}
}

func (s *FQDNSplitter) IsValid() bool {
	return !s.exhausted
}

func (s *FQDNSplitter) Next() {
	if s.cursor == len(s.domain) {
		s.exhausted = true
	} else {
		shift := strings.IndexRune(s.AfterPeriodString(), '.')
		if shift == -1 {
			s.cursor = len(s.domain)
		} else {
			s.cursor += shift + 1
		}
	}
}

func (s *FQDNSplitter) AfterPeriodString() string {
	return s.domain[s.cursor:]
}
