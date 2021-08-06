package api

import (
	"sort"
	"strings"

	"golang.org/x/net/idna"
)

//nolint: gochecknoglobals
// profile does C2 in UTS#46 with all checks on + removing leading dots.
// This is the main conversion profile in use.
var profile = idna.New(
	idna.MapForLookup(),
	idna.BidiRule(),
	// idna.Transitional(false), // https://go-review.googlesource.com/c/text/+/317729/
	idna.RemoveLeadingDots(true),
)

// FQDN is a fully qualified domain in its ASCII or Unicode (when unambiguous) form.
type FQDN string

// safelyToUnicode takes an ASCII form and returns the Unicode form
// when the round trip gives the same ASCII form back without errors.
// Otherwise, the input ASCII form is returned.
func safelyToUnicode(ascii string) (string, bool) {
	unicode, errToA := profile.ToUnicode(ascii)
	roundTrip, errToU := profile.ToASCII(unicode)
	if errToA != nil || errToU != nil || roundTrip != ascii {
		return ascii, false
	}

	return unicode, true
}

func (f FQDN) ToASCII() string { return string(f) }

func (f FQDN) Describe() string {
	best, ok := safelyToUnicode(string(f))
	if !ok {
		return string(f)
	}

	return best
}

// NewFQDN normalizes a domain to its ASCII form and then stores
// the normalized domain in its Unicode form when the round trip
// gives back the same ASCII form without errors. Otherwise,
// the ASCII form (possibly using Punycode) is stored to avoid ambiguity.
func NewFQDN(domain string) (FQDN, error) {
	normalized, err := profile.ToASCII(domain)

	// Remove the final dot for consistency
	normalized = strings.TrimSuffix(normalized, ".")

	return FQDN(normalized), err
}

func SortFQDNs(s []FQDN) { sort.Slice(s, func(i, j int) bool { return s[i] < s[j] }) }

type FQDNSplitter struct {
	domain    string
	cursor    int
	exhausted bool
}

func NewFQDNSplitter(domain FQDN) *FQDNSplitter {
	return &FQDNSplitter{
		domain:    domain.ToASCII(),
		cursor:    0,
		exhausted: false,
	}
}

func (s *FQDNSplitter) IsValid() bool  { return !s.exhausted }
func (s *FQDNSplitter) Suffix() string { return s.domain[s.cursor:] }
func (s *FQDNSplitter) Next() {
	if s.cursor == len(s.domain) {
		s.exhausted = true
	} else {
		shift := strings.IndexRune(s.Suffix(), '.')
		if shift == -1 {
			s.cursor = len(s.domain)
		} else {
			s.cursor += shift + 1
		}
	}
}
