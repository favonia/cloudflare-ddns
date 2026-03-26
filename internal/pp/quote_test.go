package pp_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestQuoteOrEmptyLabel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "(empty)", pp.QuoteOrEmptyLabel("", "(empty)"))
	require.Equal(t, `"hello"`, pp.QuoteOrEmptyLabel("hello", "(empty)"))
	require.Equal(t, `"hello\tworld"`, pp.QuoteOrEmptyLabel("hello\tworld", "(empty)"))
}

func TestQuoteIfNotHumanReadable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "readable regex stays raw",
			input: "^managed$",
			want:  "^managed$",
		},
		{
			name:  "graphic unicode stays raw",
			input: "^貓+$",
			want:  "^貓+$",
		},
		{
			name:  "leading whitespace is quoted",
			input: " ^managed$",
			want:  `" ^managed$"`,
		},
		{
			name:  "control characters are quoted",
			input: "^managed\titem$",
			want:  `"^managed\titem$"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.want, pp.QuoteIfNotHumanReadable(test.input))
		})
	}
}

func TestQuotePreviewOrEmptyLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		limit      int
		emptyLabel string
		want       string
	}{
		{
			name:       "empty",
			input:      "",
			limit:      48,
			emptyLabel: "nothing",
			want:       "nothing",
		},
		{
			name:       "short value is quoted",
			input:      "hello",
			limit:      48,
			emptyLabel: "empty",
			want:       `"hello"`,
		},
		{
			name:       "rune-safe truncation",
			input:      "貓貓貓",
			limit:      2,
			emptyLabel: "貓貓不見了",
			want:       `"貓貓..."`,
		},
		{
			name:       "non-positive limit disables truncation",
			input:      strings.Repeat("a", 5),
			limit:      0,
			emptyLabel: "---",
			want:       `"aaaaa"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.want, pp.QuotePreviewOrEmptyLabel(test.input, test.limit, test.emptyLabel))
		})
	}
}

func TestQuotePreviewIfNotHumanReadable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{
			name:  "human-readable short stays raw",
			input: "^managed$",
			limit: 48,
			want:  "^managed$",
		},
		{
			name:  "non-human-readable short is quoted",
			input: "^managed\titem$",
			limit: 48,
			want:  `"^managed\titem$"`,
		},
		{
			name:  "truncated preview is quoted",
			input: strings.Repeat("a", 49),
			limit: 48,
			want:  `"` + strings.Repeat("a", 48) + `..."`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.want, pp.QuotePreviewIfNotHumanReadable(test.input, test.limit))
		})
	}
}
