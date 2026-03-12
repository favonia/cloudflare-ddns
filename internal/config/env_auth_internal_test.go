package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTokenHasMatchingQuotes(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		token    string
		expected bool
	}{
		"too short":     {token: "\"", expected: false},
		"double quoted": {token: "\"token\"", expected: true},
		"single quoted": {token: "'token'", expected: true},
		"mismatched":    {token: "\"token'", expected: false},
		"unquoted":      {token: "token", expected: false},
		"leading only":  {token: "\"token", expected: false},
		"trailing only": {token: "token\"", expected: false},
		"empty quoted":  {token: "\"\"", expected: true},
		"empty single":  {token: "''", expected: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tokenHasMatchingQuotes(tc.token))
		})
	}
}
