package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseShoutrrrLines(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		raw      string
		expected []shoutrrrURLLine
	}{
		"empty": {
			raw:      "",
			expected: []shoutrrrURLLine{},
		},
		"blank lines skipped, line numbers preserved": {
			raw: "\n\ngeneric://x\n",
			expected: []shoutrrrURLLine{
				{lineNum: 3, rawURL: "generic://x"},
			},
		},
		"leading hash comment skipped": {
			raw: "# a comment\ngeneric://x",
			expected: []shoutrrrURLLine{
				{lineNum: 2, rawURL: "generic://x"},
			},
		},
		"indented hash comment skipped": {
			raw: "   #   indented comment\ngeneric://x",
			expected: []shoutrrrURLLine{
				{lineNum: 2, rawURL: "generic://x"},
			},
		},
		"inline hash preserved inside URL": {
			raw: "generic://x?a=b#frag",
			expected: []shoutrrrURLLine{
				{lineNum: 1, rawURL: "generic://x?a=b#frag"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, parseShoutrrrLines(tc.raw))
		})
	}
}
