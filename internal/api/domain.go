package api

import (
	"sort"
	"strings"

	"golang.org/x/net/idna"
)

// profileDroppingLeadingDots does C2 in UTS#46 with all checks on + removing leading dots.
// This is the main conversion profile in use.
//
//nolint:gochecknoglobals
var (
	profileDroppingLeadingDots = idna.New(
		idna.MapForLookup(),
		idna.BidiRule(),
		idna.Transitional(false),
		idna.RemoveLeadingDots(true),
	)
	profileKeepingLeadingDots = idna.New(
		idna.MapForLookup(),
		idna.BidiRule(),
		idna.Transitional(false),
		idna.RemoveLeadingDots(false),
	)
)

// safelyToUnicode takes an ASCII form and returns the Unicode form
// when the round trip gives the same ASCII form back without errors.
// Otherwise, the input ASCII form is returned.
func safelyToUnicode(ascii string) (string, bool) {
	unicode, errToA := profileKeepingLeadingDots.ToUnicode(ascii)
	roundTrip, errToU := profileKeepingLeadingDots.ToASCII(unicode)
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
	normalized, err := profileDroppingLeadingDots.ToASCII(domain)

	// Remove the final dot for consistency
	normalized = strings.TrimRight(normalized, ".")

	switch {
	case normalized == "*":
		return Wildcard(""), nil
	case strings.HasPrefix(normalized, "*."):
		// redo the normalization after removing the offending "*"
		normalized, err := profileKeepingLeadingDots.ToASCII(strings.TrimPrefix(normalized, "*."))
		return Wildcard(normalized), err
	default:
		return FQDN(normalized), err
	}
}

func SortDomains(s []Domain) {
	sort.Slice(s, func(i, j int) bool { return s[i].DNSNameASCII() < s[j].DNSNameASCII() })
}
