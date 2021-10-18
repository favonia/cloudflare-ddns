package api

import (
	"sort"
	"strings"

	"golang.org/x/net/idna"
)

//nolint:gochecknoglobals
// profile does C2 in UTS#46 with all checks on + removing leading dots.
// This is the main conversion profile in use.
var profile = idna.New(
	idna.MapForLookup(),
	idna.BidiRule(),
	// idna.Transitional(false), // https://go-review.googlesource.com/c/text/+/317729/
	idna.RemoveLeadingDots(true),
)

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

// NewDomain normalizes a domain to its ASCII form and then stores
// the normalized domain in its Unicode form when the round trip
// gives back the same ASCII form without errors. Otherwise,
// the ASCII form (possibly using Punycode) is stored to avoid ambiguity.
func NewDomain(domain string) (Domain, error) {
	normalized, err := profile.ToASCII(domain)

	// Remove the final dot for consistency
	normalized = strings.TrimRight(normalized, ".")

	switch {
	case strings.HasPrefix(normalized, "*."):
		// redo the normalization after removing the offending "*"
		normalized, err := profile.ToASCII(strings.TrimPrefix(normalized, "*."))
		return Wildcard(normalized), err
	default:
		return FQDN(normalized), err
	}
}

func SortDomains(s []Domain) {
	sort.Slice(s, func(i, j int) bool { return s[i].String() < s[j].String() })
}
