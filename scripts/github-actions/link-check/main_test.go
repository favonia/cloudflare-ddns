package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/testutil"
)

func TestRunRequiresSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(nil, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage: link-check <local|external>") {
		t.Fatalf("expected root usage in stderr, got %q", stderr.String())
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"all"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), `unknown subcommand "all"`) {
		t.Fatalf("expected unknown subcommand message, got %q", stderr.String())
	}
}

func TestRunHelpWritesUsageToStdout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-h"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "Usage: link-check <local|external>") {
		t.Fatalf("expected root usage in stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
}

func TestRunLocalSubcommand(t *testing.T) {
	oldRoot := root
	root = testutil.InitTrackedRepo(t)
	t.Cleanup(func() { root = oldRoot })

	testutil.WriteTrackedFile(t, root, "docs/example.markdown", "No links here.\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"local"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Repo root:") {
		t.Fatalf("expected repo root in stdout, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Local link checks passed.") {
		t.Fatalf("unexpected stdout %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
}

func TestRunExternalSubcommand(t *testing.T) {
	oldRoot := root
	root = testutil.InitTrackedRepo(t)
	t.Cleanup(func() { root = oldRoot })

	testutil.WriteTrackedFile(t, root, "docs/example.markdown", "No URLs here.\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"external"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", exitCode, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "Repo root:") {
		t.Fatalf("expected repo root in stdout, got %q", output)
	}
	if !strings.Contains(output, "Collected 0 external URLs.") {
		t.Fatalf("expected collected count, got %q", output)
	}
	if !strings.Contains(output, "External link probes passed.") {
		t.Fatalf("expected success message, got %q", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
}
