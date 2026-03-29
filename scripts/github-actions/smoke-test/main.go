// Command smoke-test runs black-box CLI smoke cases against a built ddns binary.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

type smokeCase struct {
	Name             string
	Env              map[string]string
	ExpectedExitCode int
	ExactOutput      string
	OrderedFragments []string
}

const (
	usageExitCode  = 2
	coverageEnvKey = "GOCOVERDIR"
)

type options struct {
	BinaryPath       string
	CoverProfilePath string
	RunPattern       string
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		exitf("%v", err)
	}

	cases, err := selectedCases(opts.RunPattern)
	if err != nil {
		exitf("%v", err)
	}

	coverageRoot, err := os.MkdirTemp("", "cloudflare-ddns-smoke-test-*")
	if err != nil {
		exitf("create temporary coverage root: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(coverageRoot); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "remove temporary coverage root: %v\n", err)
		}
	}()

	for _, testCase := range cases {
		_, _ = fmt.Fprintf(os.Stdout, "Running smoke case: %s\n", testCase.Name)

		coverageOutputDir := filepath.Join(coverageRoot, testCase.Name)
		if err := os.MkdirAll(coverageOutputDir, 0o750); err != nil {
			exitf("create coverage output dir for %s: %v", testCase.Name, err)
		}

		if err := runCase(testCase, opts.BinaryPath, coverageOutputDir); err != nil {
			exitf("%s: %v", testCase.Name, err)
		}
	}

	if opts.CoverProfilePath != "" {
		if err := writeCoverProfile(coverageRoot, opts.CoverProfilePath); err != nil {
			exitf("write cover profile: %v", err)
		}
	}
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("smoke-test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var opts options
	flags.StringVar(&opts.BinaryPath, "binary", "", "path to the ddns binary")
	flags.StringVar(&opts.CoverProfilePath, "coverprofile", "", "write combined coverage profile to this file")
	flags.StringVar(&opts.RunPattern, "run", "", "regular expression selecting smoke cases to run")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if opts.BinaryPath == "" {
		return options{}, errors.New("missing required flag: -binary")
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	return opts, nil
}

func selectedCases(runPattern string) ([]smokeCase, error) {
	cases := slices.Clone(allCases)
	if runPattern == "" {
		return cases, nil
	}

	pattern, err := regexp.Compile(runPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid -run pattern: %w", err)
	}

	selected := make([]smokeCase, 0, len(cases))
	for _, testCase := range cases {
		if pattern.MatchString(testCase.Name) {
			selected = append(selected, testCase)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no smoke cases match -run %q", runPattern)
	}
	return selected, nil
}

func runCase(definition smokeCase, binaryPath, coverageOutputDir string) error {
	if definition.Name == "" {
		return errors.New("smoke case must define a name")
	}

	if definition.ExactOutput == "" && len(definition.OrderedFragments) == 0 {
		return errors.New("smoke case does not define any output assertion")
	}

	if len(definition.OrderedFragments) > 0 && definition.ExactOutput != "" {
		return errors.New("smoke case cannot define both exact output and ordered fragments")
	}

	output, err := runDDNS(binaryPath, coverageOutputDir, definition)
	if err != nil {
		return err
	}

	if definition.ExactOutput != "" && output != definition.ExactOutput {
		return fmt.Errorf("unexpected full output\n--- expected ---\n%s\n--- actual ---\n%s", definition.ExactOutput, output)
	}

	if len(definition.OrderedFragments) > 0 {
		return assertInOrder(output, definition.OrderedFragments)
	}

	return nil
}

func runDDNS(binaryPath, coverageOutputDir string, definition smokeCase) (string, error) {
	env, err := childEnv(coverageOutputDir, definition.Env)
	if err != nil {
		return "", err
	}

	command, err := testCommand(
		context.Background(),
		binaryPath,
		env,
	)
	if err != nil {
		return "", err
	}

	output, err := command.CombinedOutput()
	actualExitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			actualExitCode = exitError.ExitCode()
		} else {
			return "", fmt.Errorf("run ddns: %w", err)
		}
	}

	if actualExitCode != definition.ExpectedExitCode {
		trimmedOutput := bytes.TrimRight(output, "\n")
		return "", fmt.Errorf(
			"expected exit code %d, got %d\n%s",
			definition.ExpectedExitCode,
			actualExitCode,
			trimmedOutput,
		)
	}

	return string(bytes.TrimRight(output, "\n")), nil
}

func childEnv(coverageOutputDir string, caseEnv map[string]string) ([]string, error) {
	keys := make([]string, 0, len(caseEnv))
	for key := range caseEnv {
		if key == coverageEnvKey {
			return nil, fmt.Errorf("smoke case cannot override %s", coverageEnvKey)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys)+1)
	env = append(env, coverageEnvKey+"="+coverageOutputDir)
	for _, key := range keys {
		env = append(env, key+"="+caseEnv[key])
	}
	return env, nil
}

func testCommand(
	ctx context.Context,
	executable string,
	env []string,
	args ...string,
) (*exec.Cmd, error) {
	executablePath, err := filepath.Abs(executable)
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	command := exec.CommandContext(ctx, executablePath, args...) //nolint:gosec // This is intentional
	command.Env = env
	return command, nil
}

func assertInOrder(output string, fragments []string) error {
	searchFrom := 0
	for _, fragment := range fragments {
		offset := strings.Index(output[searchFrom:], fragment)
		if offset < 0 {
			return fmt.Errorf("missing output fragment: %s\n%s", fragment, output)
		}
		searchFrom += offset + len(fragment)
	}
	return nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(usageExitCode)
}

func writeCoverProfile(coverageRoot, coverProfilePath string) error {
	coverageInputDirs, err := coverageInputDirs(coverageRoot)
	if err != nil {
		return fmt.Errorf("collect coverage input dirs: %w", err)
	}

	//nolint:gosec // The command name is fixed and the paths are direct harness inputs.
	command := exec.CommandContext(
		context.Background(),
		"go",
		"tool",
		"covdata",
		"textfmt",
		"-i",
		strings.Join(coverageInputDirs, ","),
		"-o",
		coverProfilePath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput == "" {
			return fmt.Errorf("go tool covdata textfmt: %w", err)
		}
		return fmt.Errorf("go tool covdata textfmt: %w\n%s", err, trimmedOutput)
	}
	return nil
}

func coverageInputDirs(coverageRoot string) ([]string, error) {
	entries, err := os.ReadDir(coverageRoot)
	if err != nil {
		return nil, err
	}

	inputDirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		inputDirs = append(inputDirs, filepath.Join(coverageRoot, entry.Name()))
	}
	if len(inputDirs) == 0 {
		return nil, fmt.Errorf("no coverage output directories found under %s", coverageRoot)
	}
	return inputDirs, nil
}
