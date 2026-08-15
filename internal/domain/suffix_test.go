package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
)

func expectedNormalization(leading, extraTrailing bool) domain.Normalization {
	return domain.Normalization{
		RemovedLeadingDots:       leading,
		RemovedExtraTrailingDots: extraTrailing,
	}
}

func TestSuffixDNSNameASCII(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input    domain.Suffix
		expected string
	}{
		{"example.com", "example.com"},
		{"org", "org"},
		{"", ""}, // root
	} {
		t.Run(string(tc.input), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.DNSNameASCII())
		})
	}
}

func TestSuffixString(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input    domain.Suffix
		expected string
	}{
		{"example.com", "example.com"},
		{"org", "org"},
		{"", "."}, // root renders as "."
	} {
		t.Run(string(tc.input), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.String())
		})
	}
}

func TestSuffixHasStrictSuffix(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		s        domain.Suffix
		t        domain.Suffix
		expected bool
	}{
		{"b.c", "c", true},     // b.c is strictly under c
		{"c", "c", false},      // strict: not under itself
		{"a.b.c", "c", true},   // multi-label
		{"a.b.c", "b.c", true}, // multi-label deeper suffix
		{"a.b.c", "", true},    // every non-root name is under the root
		{"", "", false},        // root is not strictly under the root
		{"c", "", true},        // single label is under the root
		{"example.com", "org", false},
		{"xc", "c", false}, // "c" is a substring suffix but not on a dot boundary
	} {
		t.Run(string(tc.s)+"/"+string(tc.t), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.s.HasStrictSuffix(tc.t))
		})
	}
}

func TestNewSuffix(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input         string
		expected      domain.Suffix
		normalization domain.Normalization
		err           error
	}{
		{"", "", expectedNormalization(false, false), nil},
		{".", "", expectedNormalization(false, false), nil},
		{"..", "", expectedNormalization(false, true), nil},
		{"...", "", expectedNormalization(false, true), nil},
		{"example.org", "example.org", expectedNormalization(false, false), nil},
		{"org", "org", expectedNormalization(false, false), nil},
		{"example.org.", "example.org", expectedNormalization(false, false), nil},
		{".example.org", "example.org", expectedNormalization(true, false), nil},
		{"example.org..", "example.org", expectedNormalization(false, true), nil},
		{"..example.org...", "example.org", expectedNormalization(true, true), nil},
		{"\u3002", "", expectedNormalization(false, false), nil},
		{"\uff0e\uff0e", "", expectedNormalization(false, true), nil},
		{"\uff61\uff61\uff61", "", expectedNormalization(false, true), nil},
		{"\u3002example.org", "example.org", expectedNormalization(true, false), nil},
		{"\uff0eexample.org", "example.org", expectedNormalization(true, false), nil},
		{"\uff61example.org", "example.org", expectedNormalization(true, false), nil},
		{"example.org\u3002\u3002", "example.org", expectedNormalization(false, true), nil},
		{"example.org\uff0e\uff0e", "example.org", expectedNormalization(false, true), nil},
		{"example.org\uff61\uff61", "example.org", expectedNormalization(false, true), nil},
		{"\u3002example.org\uff0e\uff61", "example.org", expectedNormalization(true, true), nil},
		{"a..org", "", expectedNormalization(false, false), domain.ErrEmptyInteriorLabel},
		{"a\u3002\u3002org", "", expectedNormalization(false, false), domain.ErrEmptyInteriorLabel},
		{"a\uff0e\uff0eorg", "", expectedNormalization(false, false), domain.ErrEmptyInteriorLabel},
		{"a\uff61\uff61org", "", expectedNormalization(false, false), domain.ErrEmptyInteriorLabel},
		{"a\u3002\uff0eorg", "", expectedNormalization(false, false), domain.ErrEmptyInteriorLabel},
		{"*..a.org", "", expectedNormalization(false, false), domain.ErrEmptyInteriorLabel},
		{"*.a..org", "", expectedNormalization(false, false), domain.ErrEmptyInteriorLabel},
		{"*.example.org", "", expectedNormalization(false, false), domain.ErrWildcardSuffix},
		{"*.example.org..", "", expectedNormalization(false, true), domain.ErrWildcardSuffix},
		{"*", "", expectedNormalization(false, false), domain.ErrWildcardSuffix},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, normalization, err := domain.NewSuffix(tc.input)
			require.Equal(t, tc.expected, got)
			require.Equal(t, tc.normalization, normalization)
			if tc.err == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.err)
			}
		})
	}

	_, normalization, err := domain.NewSuffix("\u0080.com")
	require.Zero(t, normalization)
	require.EqualError(t, err, "idna: disallowed rune U+0080")
}

func TestNewSuffixWildcardError(t *testing.T) {
	t.Parallel()
	_, _, err := domain.NewSuffix("*.example.org")
	require.ErrorIs(t, err, domain.ErrWildcardSuffix)
}
