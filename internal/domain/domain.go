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

// DotTrimming records removal of leading or extra trailing dots.
// Removing a single final root dot leaves both flags false.
type DotTrimming struct {
	RemovedLeadingDots       bool
	RemovedExtraTrailingDots bool
}

// trimDots removes all leading and trailing dots but leaves interior
// consecutive dots unchanged for subsequent validation to reject. It records
// whether leading dots or extra trailing dots were removed. An all-dot input
// counts as trailing dots, not leading dots.
func trimDots(ascii string) (string, DotTrimming) {
	if strings.Trim(ascii, ".") == "" {
		return "", DotTrimming{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: len(ascii) >= 2,
		}
	}

	dotTrimming := DotTrimming{
		RemovedLeadingDots:       false,
		RemovedExtraTrailingDots: false,
	}
	withoutLeadingDots := strings.TrimLeft(ascii, ".")
	if withoutLeadingDots != ascii {
		dotTrimming.RemovedLeadingDots = true
	}
	trailingDots := len(withoutLeadingDots) - len(strings.TrimRight(withoutLeadingDots, "."))
	if trailingDots >= 2 {
		dotTrimming.RemovedExtraTrailingDots = true
	}
	return strings.TrimRight(withoutLeadingDots, "."), dotTrimming
}

// wildcardSuffix recognizes both a bare wildcard and a wildcard with a suffix
// after whole-input normalization has exposed its canonical dot separators.
func wildcardSuffix(ascii string) (string, bool) {
	if ascii == "*" {
		return "", true
	}
	return strings.CutPrefix(ascii, "*.")
}

var (
	// ErrTooFewLabels means the normalized target is a single label, the root,
	// or a bare wildcard. New returns its value and DotTrimming with this error.
	ErrTooFewLabels error = errors.New("too few labels")
	// ErrEmptyInteriorLabel means consecutive dots remain inside the full name
	// after trimming, including immediately after a wildcard marker.
	ErrEmptyInteriorLabel error = errors.New("empty interior label")
)

// New parses a target domain into an ASCII-backed FQDN or Wildcard and reports
// compatibility dot removal separately in DotTrimming.
//
// ErrTooFewLabels returns a non-nil value and its DotTrimming for callers
// that permit short targets. For non-wildcards, this check precedes IDNA errors;
// that value is not guaranteed to have passed IDNA validation.
// ErrEmptyInteriorLabel returns nil and zero DotTrimming. Other errors return
// a best-effort value for diagnostics only, with zero DotTrimming.
func New(input string) (Domain, DotTrimming, error) {
	ascii, err := profileKeepingLeadingDots.ToASCII(input)
	normalized, dotTrimming := trimDots(ascii)

	if suffix, ok := wildcardSuffix(normalized); ok {
		wildcard, wildcardErr := validateNormalizedWildcardSuffix(suffix)
		if wildcardErr != nil {
			if errors.Is(wildcardErr, ErrEmptyInteriorLabel) {
				return nil, DotTrimming{}, wildcardErr
			}
			return wildcard, DotTrimming{}, wildcardErr
		}
		if wildcard == "" {
			return wildcard, dotTrimming, ErrTooFewLabels
		}
		return wildcard, dotTrimming, nil
	}
	if strings.IndexByte(normalized, '.') == -1 {
		return FQDN(normalized), dotTrimming, ErrTooFewLabels
	}

	if err != nil {
		return FQDN(normalized), DotTrimming{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: false,
		}, err
	}
	if strings.Contains(normalized, "..") {
		return nil, DotTrimming{
			RemovedLeadingDots:       false,
			RemovedExtraTrailingDots: false,
		}, ErrEmptyInteriorLabel
	}
	return FQDN(normalized), dotTrimming, nil
}

// validateNormalizedWildcardSuffix expects a suffix cut from a whole input
// whose leading and trailing dots have been trimmed. It re-runs IDNA without the wildcard marker so
// the marker's own error does not mask errors in the suffix. Target-specific
// wildcard policy belongs to the caller, so an empty suffix is valid here.
// On an IDNA error it returns a best-effort value for diagnostics only.
// On an empty-label error it returns an empty value.
func validateNormalizedWildcardSuffix(suffix string) (Wildcard, error) {
	ascii, err := profileKeepingLeadingDots.ToASCII(suffix)
	if err != nil {
		normalized, _ := trimDots(ascii)
		return Wildcard(normalized), err
	}
	// Removing "*." from "*..example.org" exposes an interior empty label at
	// the start of the suffix. Do not trim that newly exposed leading dot.
	if strings.HasPrefix(suffix, ".") {
		return "", ErrEmptyInteriorLabel
	}
	if strings.Contains(suffix, "..") {
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
