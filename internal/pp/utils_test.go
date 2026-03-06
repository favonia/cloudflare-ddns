package pp_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestJoin(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  []string
		output string
	}{
		"none":  {nil, "(none)"},
		"one":   {[]string{"hello"}, "hello"},
		"two":   {[]string{"hello", "hey"}, "hello, hey"},
		"three": {[]string{"hello", "hey", "hi"}, "hello, hey, hi"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, pp.Join(tc.input))
		})
	}
}

func TestEnglishJoin(t *testing.T) {
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
			require.Equal(t, tc.output, pp.EnglishJoin(tc.input))
		})
	}
}

func TestJoinMap(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input  []int
		output string
	}{
		"none":  {nil, "(none)"},
		"one":   {[]int{1}, "n=1"},
		"three": {[]int{1, 2, 3}, "n=1, n=2, n=3"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, pp.JoinMap(func(v int) string {
				return fmt.Sprintf("n=%d", v)
			}, tc.input))
		})
	}
}

func TestEnglishJoinMap(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input  []int
		output string
	}{
		"none":  {nil, "(none)"},
		"one":   {[]int{1}, "n=1"},
		"three": {[]int{1, 2, 3}, "n=1, n=2, and n=3"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, pp.EnglishJoinMap(func(v int) string {
				return fmt.Sprintf("n=%d", v)
			}, tc.input))
		})
	}
}
