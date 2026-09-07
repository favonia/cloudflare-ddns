package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/testenv"
)

type domainEmptyLabelScenario struct {
	name     string
	setting  string
	input    string
	accepted bool
	checks   bool
	render   func(*testing.T, pp.Verbosity) (string, bool)
}

//nolint:paralleltest // Scenario renderers mutate process environment variables.
func TestDomainEmptyLabelMessages(t *testing.T) {
	//nolint:paralleltest // Each scenario's renderers mutate process environment variables.
	for _, scenario := range domainEmptyLabelScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			require.NotEmpty(t, scenario.setting)
			require.NotEmpty(t, scenario.input)
			require.NotNil(t, scenario.render)
			for _, output := range []struct {
				name      string
				verbosity pp.Verbosity
			}{{"quiet", pp.Quiet}, {"verbose", pp.Verbose}} {
				//nolint:paralleltest // The renderer mutates process environment variables.
				t.Run(output.name, func(t *testing.T) {
					actual, accepted := scenario.render(t, output.verbosity)
					require.Equal(t, scenario.accepted, accepted)
					if scenario.accepted {
						require.Contains(t, actual, "😦")
					} else {
						require.Contains(t, actual, "😡")
					}
					if output.verbosity == pp.Verbose && scenario.name != "plain-list-leading-dot" {
						require.Contains(t, actual, "📖 Reading settings . . .")
						require.Contains(t, actual, "🔸")
						if scenario.checks {
							require.Contains(t, actual, "📖 Checking settings . . .")
						} else {
							require.NotContains(t, actual, "📖 Checking settings . . .")
						}
					}
					requireDomainEmptyLabelGolden(t, scenario.name, output.name, actual)
				})
			}
			if !scenario.accepted {
				t.Run("heartbeat", func(t *testing.T) {
					t.Parallel()
					requireDomainEmptyLabelGolden(t, scenario.name, "heartbeat", heartbeat.NewMessagef(false, "Configuration errors").Format())
				})
				t.Run("notifier", func(t *testing.T) {
					t.Parallel()
					requireDomainEmptyLabelGolden(t, scenario.name, "notifier", startupFailureNotification().Format())
				})
			}
		})
	}
}

func requireDomainEmptyLabelGolden(t *testing.T, scenario, surface, actual string) {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join("testdata", "domain-empty-label-messages", scenario, surface+".txt"))
	require.NoError(t, err)
	require.Equal(t, golden, []byte(actual))
}

func domainEmptyLabelScenarios() []domainEmptyLabelScenario {
	return []domainEmptyLabelScenario{
		{
			name:     "leading-dot",
			setting:  "IP4_DOMAINS",
			input:    ".leading.example",
			accepted: true,
			checks:   true,
			render:   renderBootstrapScenario("IP4_DOMAINS", ".leading.example"),
		},
		{
			name:     "extra-trailing-dots",
			setting:  "IP4_DOMAINS",
			input:    "trailing.example..",
			accepted: true,
			checks:   true,
			render:   renderBootstrapScenario("IP4_DOMAINS", "trailing.example.."),
		},
		{
			name:     "leading-and-extra-trailing-dots",
			setting:  "IP4_DOMAINS",
			input:    ".both.example..",
			accepted: true,
			checks:   true,
			render:   renderBootstrapScenario("IP4_DOMAINS", ".both.example.."),
		},
		{
			name:     "repeated-effective-target",
			setting:  "IP4_DOMAINS",
			input:    ".repeat.example,repeat.example..,.repeat.example",
			accepted: true,
			checks:   true,
			render:   renderBootstrapScenario("IP4_DOMAINS", ".repeat.example,repeat.example..,.repeat.example"),
		},
		{
			name:     "plain-interior-empty-label",
			setting:  "IP4_DOMAINS",
			input:    "plain..empty.example",
			accepted: false,
			checks:   false,
			render:   renderBootstrapScenario("IP4_DOMAINS", "plain..empty.example"),
		},
		{
			name:     "wildcard-immediate-empty-label",
			setting:  "PROXIED",
			input:    "sub(*..wildcard.example)",
			accepted: false,
			checks:   true,
			render:   renderBootstrapScenario("PROXIED", "sub(*..wildcard.example)"),
		},
		{
			name:     "wildcard-later-empty-label",
			setting:  "PROXIED",
			input:    "sub(*.later..empty.example)",
			accepted: false,
			checks:   true,
			render:   renderBootstrapScenario("PROXIED", "sub(*.later..empty.example)"),
		},
		{
			name:     "plain-list-leading-dot",
			setting:  "DOMAINS",
			input:    ".equivalent.example",
			accepted: true,
			checks:   true,
			render:   renderPlainListScenario("DOMAINS", ".equivalent.example"),
		},
		{
			name:     "proxied-is-leading-dot",
			setting:  "PROXIED",
			input:    "is(.equivalent.example)",
			accepted: true,
			checks:   true,
			render:   renderBootstrapScenario("PROXIED", "is(.equivalent.example)"),
		},
		{
			name:     "proxied-sub-leading-dot",
			setting:  "PROXIED",
			input:    "sub(.equivalent.example)",
			accepted: true,
			checks:   true,
			render:   renderBootstrapScenario("PROXIED", "sub(.equivalent.example)"),
		},
		{
			name:     "adjacent-warnings",
			setting:  "IP4_DOMAINS",
			input:    ",.extra.example .missing.example..",
			accepted: true,
			checks:   true,
			render:   renderBootstrapScenario("IP4_DOMAINS", ",.extra.example .missing.example.."),
		},
		{
			name:     "accepted-normalization-before-fatal",
			setting:  "IP4_DOMAINS",
			input:    ".accepted.example,fatal..empty.example",
			accepted: false,
			checks:   false,
			render:   renderBootstrapScenario("IP4_DOMAINS", ".accepted.example,fatal..empty.example"),
		},
		{
			name:     "multiple-consecutive-dot-runs",
			setting:  "IP4_DOMAINS",
			input:    "a......b.....c.....d",
			accepted: false,
			checks:   false,
			render:   renderBootstrapScenario("IP4_DOMAINS", "a......b.....c.....d"),
		},
		{
			name:     "wildcard-multiple-consecutive-dot-runs",
			setting:  "PROXIED",
			input:    "sub(*......a.....b)",
			accepted: false,
			checks:   true,
			render:   renderBootstrapScenario("PROXIED", "sub(*......a.....b)"),
		},
	}
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
