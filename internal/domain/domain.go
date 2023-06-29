package domain

import (
	"strings"

	"golang.org/x/exp/slices"
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
func safelyToUnicode(ascii string) string {
	unicode, errToA := profileKeepingLeadingDots.ToUnicode(ascii)
	roundTrip, errToU := profileKeepingLeadingDots.ToASCII(unicode)
	if errToA != nil || errToU != nil || roundTrip != ascii {
		return ascii
	}

	return unicode
}

// StringToASCII normalizes a domain with best efforts, ignoring errors.
func StringToASCII(domain string) string {
	normalized, _ := profileDroppingLeadingDots.ToASCII(domain)

	// Remove the final dot for consistency
	normalized = strings.TrimRight(normalized, ".")

	return normalized
}

// NewDomain normalizes a domain to its ASCII form and then stores
// the normalized domain in its Unicode form when the round trip
// gives back the same ASCII form without errors. Otherwise,
// the ASCII form (possibly using Punycode) is stored to avoid ambiguity.
func New(domain string) (Domain, error) {
	normalized, err := profileDroppingLeadingDots.ToASCII(domain)

	// Remove the final dot for consistency
	normalized = strings.TrimRight(normalized, ".")

	// Special case 1: "*"
	if normalized == "*" {
		return Wildcard(""), nil
	}

	// Special case 2: "*.something"
	if normalized, ok := strings.CutPrefix(normalized, "*."); ok {
		// redo the normalization after removing the offending "*" to get the true error (if any)
		normalized, err := profileKeepingLeadingDots.ToASCII(strings.TrimPrefix(normalized, "*."))
		return Wildcard(normalized), err
	}

	// otherwise
	return FQDN(normalized), err
}

// SortDomains sorts a list of domains according to their ASCII representations.
func SortDomains(s []Domain) {
	slices.SortFunc(s, func(d1, d2 Domain) bool { return d1.DNSNameASCII() < d2.DNSNameASCII() })
}
