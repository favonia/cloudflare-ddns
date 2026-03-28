package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOptions(t *testing.T) {
	t.Parallel()

	opts, err := parseOptions([]string{
		"-binary", "/tmp/ddns",
		"-coverprofile", "/tmp/coverage.txt",
		"-run", "quiet",
	})

	require.NoError(t, err)
	require.Equal(t, options{
		BinaryPath:       "/tmp/ddns",
		CoverProfilePath: "/tmp/coverage.txt",
		RunPattern:       "quiet",
	}, opts)
}

func TestParseOptionsRequiresBinary(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{})

	require.EqualError(t, err, "missing required flag: -binary")
}

func TestParseOptionsRejectsPositionalArgs(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{
		"-binary", "/tmp/ddns",
		"extra",
	})

	require.EqualError(t, err, "unexpected positional arguments: extra")
}

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

func TestChildEnvUsesOnlyCoverageAndCaseEnv(t *testing.T) {
	t.Parallel()

	env, err := childEnv("/tmp/cover", map[string]string{
		"QUIET": "true",
		"EMOJI": "false",
	})

	require.NoError(t, err)
	require.Equal(t, []string{
		"GOCOVERDIR=/tmp/cover",
		"EMOJI=false",
		"QUIET=true",
	}, env)
}

func TestChildEnvRejectsReservedCoverageKey(t *testing.T) {
	t.Parallel()

	_, err := childEnv("/tmp/cover", map[string]string{
		"GOCOVERDIR": "/tmp/other",
	})

	require.EqualError(t, err, "smoke case cannot override GOCOVERDIR")
}

func TestCommandUsesBinaryDirectly(t *testing.T) {
	t.Parallel()

	binaryDir := t.TempDir()
	executable := filepath.Join(binaryDir, "ddns")

	command, err := testCommand(
		context.Background(),
		executable,
		[]string{"GOCOVERDIR=/tmp/cover", "QUIET=true"},
		"--help",
	)

	require.NoError(t, err)
	require.Equal(t, executable, command.Path)
	require.Equal(t, []string{
		executable,
		"--help",
	}, command.Args)
	require.Equal(t, []string{"GOCOVERDIR=/tmp/cover", "QUIET=true"}, command.Env)
}
