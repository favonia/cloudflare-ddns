package domain

import (
	"cmp"
	"errors"
	"slices"
	"strings"

	"golang.org/x/net/idna"
)

// profileDroppingLeadingDots does C2 in UTS#46 with all checks on + removing leading dots.
// StringToASCII keeps it for best-effort diagnostic rendering.
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

// Normalization records compatibility cleanup applied while constructing a
// canonical domain value.
type Normalization struct {
	RemovedLeadingDots       bool
	RemovedExtraTrailingDots bool
}

// normalizeBoundary removes all leading and trailing dots, recording leading
// dots and runs of two or more trailing dots in Normalization. An all-dot input
// counts as trailing dots, not leading dots.
func normalizeBoundary(ascii string) (string, Normalization) {
	if strings.Trim(ascii, ".") == "" {
		return "", Normalization{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: len(ascii) >= 2,
		}
	}

	normalization := Normalization{
		RemovedLeadingDots:       false,
		RemovedExtraTrailingDots: false,
	}
	withoutLeadingDots := strings.TrimLeft(ascii, ".")
	if withoutLeadingDots != ascii {
		normalization.RemovedLeadingDots = true
	}
	trailingDots := len(withoutLeadingDots) - len(strings.TrimRight(withoutLeadingDots, "."))
	if trailingDots >= 2 {
		normalization.RemovedExtraTrailingDots = true
	}
	return strings.TrimRight(withoutLeadingDots, "."), normalization
}

func hasEmptyInteriorLabel(ascii string) bool {
	return strings.HasPrefix(ascii, ".") || strings.Contains(ascii, "..")
}

// wildcardSuffix recognizes both a bare wildcard and a wildcard with a suffix
// after whole-input normalization has exposed its canonical dot separators.
func wildcardSuffix(ascii string) (string, bool) {
	if ascii == "*" {
		return "", true
	}
	return strings.CutPrefix(ascii, "*.")
}

// ErrTooFewLabels means a domain name has fewer than two labels after
// normalization — a single label (com, localhost), the empty/root name (.),
// or a bare "*". Such a name cannot be a reasonable target domain name.
var (
	ErrTooFewLabels       error = errors.New("too few labels")
	ErrEmptyInteriorLabel error = errors.New("empty interior label")
)

// New normalizes a domain to its ASCII form and then stores
// the normalized domain in its Unicode form when the round trip
// gives back the same ASCII form without errors. Otherwise,
// the ASCII form (possibly using Punycode) is stored to avoid ambiguity.
func New(input string) (Domain, Normalization, error) {
	ascii, err := profileKeepingLeadingDots.ToASCII(input)
	normalized, normalization := normalizeBoundary(ascii)

	if suffix, ok := wildcardSuffix(normalized); ok {
		wildcard, wildcardErr := validateNormalizedWildcardSuffix(suffix)
		if wildcardErr != nil {
			if errors.Is(wildcardErr, ErrEmptyInteriorLabel) {
				return nil, Normalization{}, wildcardErr
			}
			return wildcard, Normalization{}, wildcardErr
		}
		if wildcard == "" {
			return wildcard, normalization, ErrTooFewLabels
		}
		return wildcard, normalization, nil
	}
	if strings.IndexByte(normalized, '.') == -1 {
		return FQDN(normalized), normalization, ErrTooFewLabels
	}

	if err != nil {
		return FQDN(normalized), Normalization{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: false,
		}, err
	}
	if hasEmptyInteriorLabel(normalized) {
		return nil, Normalization{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: false,
		}, ErrEmptyInteriorLabel
	}
	return FQDN(normalized), normalization, nil
}

// validateNormalizedWildcardSuffix expects a suffix cut from a whole input
// after boundary normalization. It re-runs IDNA without the wildcard marker so
// the marker's own error does not mask errors in the suffix. Target-specific
// wildcard policy belongs to the caller, so an empty suffix is valid here.
func validateNormalizedWildcardSuffix(suffix string) (Wildcard, error) {
	ascii, err := profileKeepingLeadingDots.ToASCII(suffix)
	if err != nil {
		normalized, _ := normalizeBoundary(ascii)
		return Wildcard(normalized), err
	}
	if hasEmptyInteriorLabel(suffix) {
		return "", ErrEmptyInteriorLabel
	}
	return Wildcard(ascii), nil
}

// CompareDomain compares two domains by their ASCII representations.
func CompareDomain(d1, d2 Domain) int {
	return cmp.Compare(d1.DNSNameASCII(), d2.DNSNameASCII())
}

// SortDomains sorts a list of domains according to their ASCII representations.
func SortDomains(s []Domain) {
	slices.SortFunc(s, CompareDomain)
}
