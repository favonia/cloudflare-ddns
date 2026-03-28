package main

import (
	"context"
	"os/exec"
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

func TestSandboxCommandUsesBubblewrap(t *testing.T) {
	t.Parallel()

	writableDir := t.TempDir()
	bwrapPath, err := exec.LookPath("bwrap")
	require.NoError(t, err)

	command, err := sandboxCommand(
		context.Background(),
		"/tmp/ddns",
		[]string{writableDir},
		[]string{"GOCOVERDIR=" + writableDir, "QUIET=true"},
	)

	require.NoError(t, err)
	require.Equal(t, bwrapPath, command.Path)
	require.Equal(t, []string{
		bwrapPath,
		"--unshare-net",
		"--ro-bind", "/", "/",
		"--bind", writableDir, writableDir,
		"/tmp/ddns",
	}, command.Args)
	require.Equal(t, []string{"GOCOVERDIR=" + writableDir, "QUIET=true"}, command.Env)
}

func TestSandboxCommandBlocksNetwork(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is unavailable")
	}
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is unavailable")
	}

	command, err := sandboxCommand(
		context.Background(),
		pythonPath,
		nil,
		nil,
		"-c",
		`import socket
s=socket.socket()
s.settimeout(1)
try:
    s.connect(("1.1.1.1", 443))
    print("connected")
except Exception as err:
    print(type(err).__name__, err)`,
	)
	require.NoError(t, err)

	output, err := command.CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(output), "OSError")
	require.NotContains(t, string(output), "connected")
}
