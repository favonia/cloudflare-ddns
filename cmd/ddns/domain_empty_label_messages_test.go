package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/testenv"
)

var domainEmptyLabelMatrixOutput = flag.String(
	"domain-empty-label-matrix-output",
	"",
	"write the rendered domain empty-label operator matrix to this path",
)

const domainEmptyLabelMatrixGolden = "testdata/domain_empty_label_messages.txt"

type domainEmptyLabelMatrixScenario struct {
	name      string
	setting   string
	input     string
	accepted  bool
	checks    bool
	encounter string
	render    func(*testing.T, pp.Verbosity) (string, bool)
}

// TestRenderDomainEmptyLabelMessageMatrix renders the complete operator-facing
// matrix without making a network request or requiring usable credentials.
//
//nolint:paralleltest // the bootstrap deliberately clears process environment variables.
func TestRenderDomainEmptyLabelMessageMatrix(t *testing.T) {
	matrix := renderDomainEmptyLabelMessageMatrix(t)
	golden, err := os.ReadFile(domainEmptyLabelMatrixGolden)
	require.NoError(t, err)
	require.Equal(t, string(golden), matrix)
	if *domainEmptyLabelMatrixOutput != "" {
		outputPath, err := domainEmptyLabelMatrixOutputPath(*domainEmptyLabelMatrixOutput)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(outputPath, []byte(matrix), 0o600))
	}
}

func domainEmptyLabelMatrixOutputPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return filepath.Join(directory, path), nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("cannot find module root for %q", path)
		}
		directory = parent
	}
}

func renderDomainEmptyLabelMessageMatrix(t *testing.T) string {
	t.Helper()

	scenarios := []domainEmptyLabelMatrixScenario{
		{
			name:      "01. leading-dot compatibility",
			setting:   "IP4_DOMAINS",
			input:     ".leading.example",
			accepted:  true,
			checks:    true,
			encounter: "Read settings, then check settings",
			render:    renderBootstrapScenario("IP4_DOMAINS", ".leading.example"),
		},
		{
			name:      "02. extra-trailing-dot compatibility",
			setting:   "IP4_DOMAINS",
			input:     "trailing.example..",
			accepted:  true,
			checks:    true,
			encounter: "Read settings, then check settings",
			render:    renderBootstrapScenario("IP4_DOMAINS", "trailing.example.."),
		},
		{
			name:      "03. one atom with both normalizations",
			setting:   "IP4_DOMAINS",
			input:     ".both.example..",
			accepted:  true,
			checks:    true,
			encounter: "Read settings, then check settings",
			render:    renderBootstrapScenario("IP4_DOMAINS", ".both.example.."),
		},
		{
			name:      "04. repeated effective targets retain encounter order",
			setting:   "IP4_DOMAINS",
			input:     ".repeat.example,repeat.example..,.repeat.example",
			accepted:  true,
			checks:    true,
			encounter: "Read settings in source-atom encounter order, then check settings",
			render:    renderBootstrapScenario("IP4_DOMAINS", ".repeat.example,repeat.example..,.repeat.example"),
		},
		{
			name:      "05. plain interior empty label",
			setting:   "IP4_DOMAINS",
			input:     "plain..empty.example",
			accepted:  false,
			checks:    false,
			encounter: "Read settings, then report the startup failure",
			render:    renderBootstrapScenario("IP4_DOMAINS", "plain..empty.example"),
		},
		{
			name:      "06. empty label immediately after wildcard marker",
			setting:   "PROXIED",
			input:     "sub(*..wildcard.example)",
			accepted:  false,
			checks:    true,
			encounter: "Read settings, check settings, then report the startup failure",
			render:    renderBootstrapScenario("PROXIED", "sub(*..wildcard.example)"),
		},
		{
			name:      "07. later interior empty label in a wildcard suffix",
			setting:   "PROXIED",
			input:     "sub(*.later..empty.example)",
			accepted:  false,
			checks:    true,
			encounter: "Read settings, check settings, then report the startup failure",
			render:    renderBootstrapScenario("PROXIED", "sub(*.later..empty.example)"),
		},
		{
			name:      "08. equivalent plain-list, is(...), and sub(...) cases",
			setting:   "DOMAINS / PROXIED",
			input:     ".equivalent.example / is(.equivalent.example) / sub(.equivalent.example)",
			accepted:  true,
			checks:    true,
			encounter: "Render the plain list, then is(...), then sub(...) in that order",
			render:    renderEquivalentFormsScenario,
		},
		{
			name:      "09. compatibility output beside extra- and missing-comma warnings",
			setting:   "IP4_DOMAINS",
			input:     ",.extra.example .missing.example..",
			accepted:  true,
			checks:    true,
			encounter: "Read settings in the existing comma-warning and source-atom order, then check settings",
			render:    renderBootstrapScenario("IP4_DOMAINS", ",.extra.example .missing.example.."),
		},
		{
			name:      "10. accepted normalization before a fatal entry",
			setting:   "IP4_DOMAINS",
			input:     ".accepted.example,fatal..empty.example",
			accepted:  false,
			checks:    false,
			encounter: "Read settings in source-atom encounter order, then report the startup failure",
			render:    renderBootstrapScenario("IP4_DOMAINS", ".accepted.example,fatal..empty.example"),
		},
		{
			name:      "11. multiple long consecutive-dot runs",
			setting:   "IP4_DOMAINS",
			input:     "a......b.....c.....d",
			accepted:  false,
			checks:    false,
			encounter: "Read settings, then report the startup failure",
			render:    renderBootstrapScenario("IP4_DOMAINS", "a......b.....c.....d"),
		},
		{
			name:      "12. wildcard-marker and later consecutive-dot runs",
			setting:   "PROXIED",
			input:     "sub(*......a.....b)",
			accepted:  false,
			checks:    true,
			encounter: "Read settings, check settings, then report the startup failure",
			render:    renderBootstrapScenario("PROXIED", "sub(*......a.....b)"),
		},
	}

	var output strings.Builder
	output.WriteString("Domain empty-label operator-message matrix\n")
	output.WriteString("The harness makes no network request and uses a deliberately unusable API token.\n")
	for _, scenario := range scenarios {
		writeDomainEmptyLabelScenario(t, &output, scenario)
	}
	return output.String()
}

func writeDomainEmptyLabelScenario(t *testing.T, output *strings.Builder, scenario domainEmptyLabelMatrixScenario) {
	t.Helper()

	require.NotEmpty(t, scenario.setting, "%s setting", scenario.name)
	require.NotEmpty(t, scenario.input, "%s input", scenario.name)
	require.NotNil(t, scenario.render, "%s renderer", scenario.name)
	quiet, quietAccepted := scenario.render(t, pp.Quiet)
	verbose, verboseAccepted := scenario.render(t, pp.Verbose)
	require.Equalf(t, scenario.accepted, quietAccepted, "%s quiet result", scenario.name)
	require.Equalf(t, scenario.accepted, verboseAccepted, "%s verbose result", scenario.name)
	if scenario.accepted {
		require.Containsf(t, quiet, "😦", "%s quiet warning emoji", scenario.name)
		require.Containsf(t, verbose, "😦", "%s verbose warning emoji", scenario.name)
	} else {
		require.Containsf(t, quiet, "😡", "%s quiet error emoji", scenario.name)
		require.Containsf(t, verbose, "😡", "%s verbose error emoji", scenario.name)
	}
	require.Containsf(t, verbose, "📖 Reading settings . . .", "%s verbose reading envelope", scenario.name)
	require.Containsf(t, verbose, "🔸", "%s verbose default-setting bullet", scenario.name)
	if scenario.checks {
		require.Containsf(t, verbose, "📖 Checking settings . . .", "%s verbose checking envelope", scenario.name)
	} else {
		require.NotContainsf(t, verbose, "📖 Checking settings . . .", "%s verbose setup", scenario.name)
	}

	fmt.Fprintf(output, "\n%s\n", scenario.name)
	fmt.Fprintf(output, "setting/input: %s=%q\n", scenario.setting, scenario.input)
	writeDomainEmptyLabelOutput(output, "quiet CLI output", quiet)
	writeDomainEmptyLabelOutput(output, "verbose CLI output", verbose)
	if scenario.accepted {
		output.WriteString("accepted/rejected result: accepted\n")
		output.WriteString("heartbeat output: no feature-specific heartbeat result\n")
		output.WriteString("notifier output: no feature-specific notifier result\n")
	} else {
		output.WriteString("accepted/rejected result: rejected\n")
		fmt.Fprintf(output, "heartbeat output: %s\n", heartbeat.NewMessagef(false, "Configuration errors").Format())
		fmt.Fprintf(output, "notifier output: %s\n", startupFailureNotification().Format())
	}
	fmt.Fprintf(output, "encounter order: %s\n", scenario.encounter)
}

func writeDomainEmptyLabelOutput(output *strings.Builder, label, value string) {
	fmt.Fprintf(output, "%s:\n", label)
	if value == "" {
		output.WriteString("<none>\n")
		return
	}
	output.WriteString(value)
}

func renderBootstrapScenario(setting, input string) func(*testing.T, pp.Verbosity) (string, bool) {
	return func(t *testing.T, verbosity pp.Verbosity) (string, bool) {
		t.Helper()
		setDomainEmptyLabelBootstrap(t, setting, input)

		var output bytes.Buffer
		formatter := pp.New(&output, true, verbosity)
		raw := config.DefaultRaw()
		readOK := raw.ReadEnv(formatter)
		if !readOK {
			return output.String(), false
		}
		_, buildOK := raw.BuildConfig(formatter)
		return output.String(), buildOK
	}
}

func renderPlainListScenario(setting, input string) func(*testing.T, pp.Verbosity) (string, bool) {
	return func(t *testing.T, verbosity pp.Verbosity) (string, bool) {
		t.Helper()

		var output bytes.Buffer
		_, ok := domainexp.ParseList(pp.New(&output, true, verbosity), setting, input)
		return output.String(), ok
	}
}

func renderEquivalentFormsScenario(t *testing.T, verbosity pp.Verbosity) (string, bool) {
	t.Helper()

	plainOutput, plainOK := renderPlainListScenario("DOMAINS", ".equivalent.example")(t, verbosity)
	isOutput, isOK := renderBootstrapScenario("PROXIED", "is(.equivalent.example)")(t, verbosity)
	subOutput, subOK := renderBootstrapScenario("PROXIED", "sub(.equivalent.example)")(t, verbosity)
	return "plain-list:\n" + plainOutput + "is(...):\n" + isOutput + "sub(...):\n" + subOutput, plainOK && isOK && subOK
}

func setDomainEmptyLabelBootstrap(t *testing.T, setting, input string) {
	t.Helper()
	testenv.ClearAll(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "deadbeef")
	t.Setenv("IP4_PROVIDER", "local")
	t.Setenv("IP6_PROVIDER", "none")
	if setting != "IP4_DOMAINS" {
		t.Setenv("IP4_DOMAINS", "target.example")
	}
	t.Setenv(setting, input)
}
