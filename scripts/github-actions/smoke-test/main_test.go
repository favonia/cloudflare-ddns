package main

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOptions(t *testing.T) {
	t.Parallel()

	opts, err := parseOptions([]string{
		"-binary", "/tmp/ddns",
		"-podman", "/usr/bin/podman",
		"-coverprofile", "/tmp/coverage.txt",
		"-run", "quiet",
	})

	require.NoError(t, err)
	require.Equal(t, options{
		BinaryPath:       "/tmp/ddns",
		CoverProfilePath: "/tmp/coverage.txt",
		PodmanPath:       "/usr/bin/podman",
		RunPattern:       "quiet",
	}, opts)
}

func TestParseOptionsRequiresBinary(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{
		"-podman", "/usr/bin/podman",
	})

	require.EqualError(t, err, "missing required flag: -binary")
}

func TestParseOptionsRequiresPodman(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{
		"-binary", "/tmp/ddns",
	})

	require.EqualError(t, err, "missing required flag: -podman")
}

func TestParseOptionsRejectsPositionalArgs(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{
		"-binary", "/tmp/ddns",
		"-podman", "/usr/bin/podman",
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

	env, err := childEnv(containerCoverageDir, map[string]string{
		"QUIET": "true",
		"EMOJI": "false",
	})

	require.NoError(t, err)
	require.Equal(t, []string{
		"GOCOVERDIR=" + containerCoverageDir,
		"EMOJI=false",
		"QUIET=true",
	}, env)
}

func TestChildEnvRejectsReservedCoverageKey(t *testing.T) {
	t.Parallel()

	_, err := childEnv(containerCoverageDir, map[string]string{
		"GOCOVERDIR": "/tmp/other",
	})

	require.EqualError(t, err, "smoke case cannot override GOCOVERDIR")
}

func TestSandboxCommandUsesPodman(t *testing.T) {
	t.Parallel()

	binaryDir := t.TempDir()
	coverageDir := t.TempDir()
	rootfsDir := t.TempDir()
	executable := filepath.Join(binaryDir, "ddns")

	command, err := sandboxCommand(
		context.Background(),
		"/usr/bin/podman",
		rootfsDir,
		executable,
		coverageDir,
		[]string{"GOCOVERDIR=" + containerCoverageDir, "QUIET=true"},
		"--help",
	)

	require.NoError(t, err)
	require.Equal(t, "/usr/bin/podman", command.Path)
	require.Equal(t, []string{
		"/usr/bin/podman",
		"run",
		"--rm",
		"--network", "none",
		"--no-hosts",
		"--no-hostname",
		"--read-only",
		"--read-only-tmpfs=false",
		"--tmpfs", "/tmp:rw,exec,nosuid,nodev",
		"--workdir", "/tmp",
		"--cap-drop=all",
		"--security-opt", "no-new-privileges",
		"--volume", binaryDir + ":" + containerBinaryDir + ":ro",
		"--volume", coverageDir + ":" + containerCoverageDir + ":rw",
		"--rootfs", rootfsDir,
		"-e", "GOCOVERDIR=" + containerCoverageDir,
		"-e", "QUIET=true",
		path.Join(containerBinaryDir, "ddns"),
		"--help",
	}, command.Args)
	require.Nil(t, command.Env)
}

func TestSandboxCommandBlocksNetwork(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("podman smoke sandbox targets Linux")
	}

	podmanPath := requirePodman(t)
	probePath := buildNetworkProbe(t)
	rootfsDir := t.TempDir()

	command, err := sandboxCommand(
		context.Background(),
		podmanPath,
		rootfsDir,
		probePath,
		t.TempDir(),
		nil,
	)
	require.NoError(t, err)

	output, err := command.CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(output), "1.1.1.1:443")
	require.NotContains(t, string(output), "connected")
}

func requirePodman(t *testing.T) string {
	t.Helper()

	podmanPath, err := exec.LookPath("podman")
	if err != nil {
		t.Skip("podman is unavailable")
	}

	command := exec.Command(podmanPath, "info")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Skipf("podman info is unavailable: %s", strings.TrimSpace(string(output)))
	}

	return podmanPath
}

func buildNetworkProbe(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "main.go")
	executablePath := filepath.Join(sourceDir, "probe")
	source := `package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	connection, err := net.DialTimeout("tcp", "1.1.1.1:443", time.Second)
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = connection.Close()
	fmt.Println("connected")
}
`
	require.NoError(t, os.WriteFile(sourcePath, []byte(source), 0o600))

	buildCommand := exec.Command("go", "build", "-o", executablePath, sourcePath)
	buildCommand.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+runtime.GOARCH)
	output, err := buildCommand.CombinedOutput()
	require.NoError(t, err, string(output))

	return executablePath
}
