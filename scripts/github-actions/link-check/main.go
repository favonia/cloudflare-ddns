package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

var root = mustProjectRoot()

func mustProjectRoot() string {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", ".."))
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("link-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	mode := flags.String("mode", "all", "which checks to run: local, external, or all")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if *mode != "local" && *mode != "external" && *mode != "all" {
		_, _ = fmt.Fprintf(stderr, "unknown mode %q\n", *mode)
		return 1
	}

	cfg := defaultConfig()

	files, err := gitTrackedFiles()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	markdownFiles := filterFiles(
		files,
		cfg.LocalMarkdown.SourceFiles.IncludeGlobs,
		cfg.LocalMarkdown.SourceFiles.ExcludeGlobs,
	)
	textFiles := filterFiles(
		files,
		cfg.RepoPaths.SourceFiles.IncludeGlobs,
		cfg.RepoPaths.SourceFiles.ExcludeGlobs,
	)
	externalFiles := filterFiles(
		files,
		cfg.External.SourceFiles.IncludeGlobs,
		cfg.External.SourceFiles.ExcludeGlobs,
	)

	exitCode := 0
	if *mode == "local" || *mode == "all" {
		localIssues := validateLocalMarkdownLinks(markdownFiles)
		localIssues = append(localIssues, validateRepoPaths(textFiles, files, cfg.RepoPaths.TargetPaths.IgnoreExact)...)
		if len(localIssues) > 0 {
			slices.SortFunc(localIssues, func(a, b linkIssue) int {
				return strings.Compare(a.Render(), b.Render())
			})
			for _, issue := range localIssues {
				_, _ = fmt.Fprintln(stderr, issue.Render())
			}
			exitCode = 1
		} else {
			_, _ = fmt.Fprintln(stdout, "Local link checks passed.")
		}
	}

	if *mode == "external" || *mode == "all" {
		externalURLs := collectExternalURLs(
			externalFiles,
			cfg.External.TargetURLs.IgnoreExact,
			cfg.External.TargetURLs.IgnorePatterns,
		)
		_, _ = fmt.Fprintf(stdout, "Collected %d external URLs.\n", len(externalURLs))
		failures, warnings := runExternalProbe(externalURLs, cfg.ExternalProbe)
		if writeExternalFindings(stderr, failures, warnings) {
			exitCode = 1
		} else {
			_, _ = fmt.Fprintln(stdout, "External link probes passed.")
		}
	}

	return exitCode
}

func gitTrackedFiles() ([]string, error) {
	command := exec.CommandContext(
		context.Background(), "git", "ls-files", "-z",
	)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("run git ls-files: %w", err)
	}
	rawPaths := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(rawPaths))
	for _, rawPath := range rawPaths {
		if len(rawPath) == 0 {
			continue
		}
		files = append(files, filepath.ToSlash(string(rawPath)))
	}
	return files, nil
}
