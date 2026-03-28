package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectedCasesDefaultIncludesAllCases(t *testing.T) {
	t.Parallel()

	selected, err := selectedCases("")

	require.NoError(t, err)
	require.Equal(t, allCases, selected)
}

func TestSelectedCasesFiltersByRegexp(t *testing.T) {
	t.Parallel()

	selected, err := selectedCases("quiet|emoji")

	require.NoError(t, err)
	require.Len(t, selected, 2)
	require.Equal(t, "emoji-invalid", selected[0].Name)
	require.Equal(t, "quiet-invalid", selected[1].Name)
}

func TestSelectedCasesRejectsNoMatch(t *testing.T) {
	t.Parallel()

	_, err := selectedCases("does-not-exist")

	require.EqualError(t, err, `no smoke cases match -run "does-not-exist"`)
}

func TestSelectedCasesRejectsInvalidRegexp(t *testing.T) {
	t.Parallel()

	_, err := selectedCases("(")

	require.ErrorContains(t, err, "invalid -run pattern")
}
