package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
)

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
		input    string
		expected domain.Suffix
		ok       bool
	}{
		{"example.org", "example.org", true},
		{"org", "org", true},                  // single label accepted (looser than New)
		{".", "", true},                       // root accepted
		{"", "", true},                        // empty is the root
		{"example.org.", "example.org", true}, // trailing dot trimmed
		{"tHe.CaPiTaL.cAsE", "the.capital.case", true},
		{"*", "", false},             // wildcard rejected
		{"*.example.org", "", false}, // wildcard rejected
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewSuffix(tc.input)
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expected, got)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestNewSuffixWildcardError(t *testing.T) {
	t.Parallel()
	_, err := domain.NewSuffix("*.example.org")
	require.ErrorIs(t, err, domain.ErrWildcardSuffix)
}
