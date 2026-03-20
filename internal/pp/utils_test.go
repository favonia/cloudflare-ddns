package pp_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestOrdinal(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  int
		output string
	}{
		"1":   {1, "1st"},
		"2":   {2, "2nd"},
		"3":   {3, "3rd"},
		"4":   {4, "4th"},
		"10":  {10, "10th"},
		"11":  {11, "11th"},
		"12":  {12, "12th"},
		"13":  {13, "13th"},
		"14":  {14, "14th"},
		"21":  {21, "21st"},
		"22":  {22, "22nd"},
		"23":  {23, "23rd"},
		"100": {100, "100th"},
		"101": {101, "101st"},
		"111": {111, "111th"},
		"112": {112, "112th"},
		"113": {113, "113th"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, pp.Ordinal(tc.input))
		})
	}
}

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
