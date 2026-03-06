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
			input: "^猫+$",
			want:  "^猫+$",
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

func TestQuotePreview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{
			name:  "short value is quoted",
			input: "hello",
			limit: 48,
			want:  `"hello"`,
		},
		{
			name:  "rune-safe truncation",
			input: "猫猫猫",
			limit: 2,
			want:  `"猫猫..."`,
		},
		{
			name:  "non-positive limit disables truncation",
			input: strings.Repeat("a", 5),
			limit: 0,
			want:  `"aaaaa"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.want, pp.QuotePreview(test.input, test.limit))
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
