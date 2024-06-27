package updater_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/updater"
)

func TestListJoin(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  []string
		output string
	}{
		"none":  {nil, ""},
		"one":   {[]string{"hello"}, "hello"},
		"two":   {[]string{"hello", "hey"}, "hello, hey"},
		"three": {[]string{"hello", "hey", "hi"}, "hello, hey, hi"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, updater.ListJoin(tc.input))
		})
	}
}

func TestListEnglishJoin(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  []string
		output string
	}{
		"none":  {nil, "(none)"},
		"one":   {[]string{"hello"}, "hello"},
		"two":   {[]string{"hello", "hey"}, "hello and hey"},
		"three": {[]string{"hello", "hey", "hi"}, "hello, hey, and hi"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, updater.ListEnglishJoin(tc.input))
		})
	}
}
