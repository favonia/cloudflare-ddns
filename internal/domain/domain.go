package domain

import (
	"cmp"
	"errors"
	"slices"
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

// ErrNotFQDN means a domain name is not fully qualified.
var ErrNotFQDN error = errors.New("not fully qualified")

// New normalizes a domain to its ASCII form and then stores
// the normalized domain in its Unicode form when the round trip
// gives back the same ASCII form without errors. Otherwise,
// the ASCII form (possibly using Punycode) is stored to avoid ambiguity.
func New(domain string) (Domain, error) {
	normalized, err := profileDroppingLeadingDots.ToASCII(domain)

	// Remove the final dot for consistency
	normalized = strings.TrimRight(normalized, ".")

	if strings.IndexByte(normalized, '.') == -1 {
		// Special case: "*"
		if normalized == "*" {
			return Wildcard(""), ErrNotFQDN
		} else {
			return FQDN(normalized), ErrNotFQDN
		}
	}

	// Special case: "*.something"
	if normalized, ok := strings.CutPrefix(normalized, "*."); ok {
		// redo the normalization after removing the offending "*" to get the true error (if any)
		normalized, err := profileKeepingLeadingDots.ToASCII(normalized)
		return Wildcard(normalized), err
	}

	// otherwise
	return FQDN(normalized), err
}

// SortDomains sorts a list of domains according to their ASCII representations.
func SortDomains(s []Domain) {
	slices.SortFunc(s, func(d1, d2 Domain) int { return cmp.Compare(d1.DNSNameASCII(), d2.DNSNameASCII()) })
}
