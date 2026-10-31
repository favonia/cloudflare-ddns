package updater

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// These direct semantic fixtures exercise the actual constructors and composition
// for problem branches outside the scheduled capture matrix. The constructors do
// not emit CLI output; explicit empty adjacency fields prevent this fixture from
// implying it ran provider/configuration diagnostics or a lifecycle loop.
func TestNotificationProblemRenderingCapture(t *testing.T) {
	t.Parallel()
	type event struct {
		Channel string        `json:"channel"`
		Text    string        `json:"text"`
		Kind    notifier.Kind `json:"kind,omitempty"`
	}
	type capture struct {
		Scenario  string   `json:"scenario"`
		Verbosity string   `json:"verbosity"`
		Source    string   `json:"source"`
		CLIBefore []string `json:"cliBefore"`
		Events    []event  `json:"events"`
		CLIAfter  []string `json:"cliAfter"`
	}
	captures := make([]capture, 0, 6)
	for _, scenario := range append([]renderingScenario{
		{"direct-semantic-filter-rejection", generateFilterAbortDetectMessage(ipnet.IP4)},
		{"direct-semantic-derivation-failure", generateIP6DerivationFailureMessage([]domain.Domain{domain.FQDN("host0.example"), domain.FQDN("host1.example")})},
		{"direct-semantic-missing-target", generateMissingTargetSetsMessage(ipnet.IP6, []domain.Domain{domain.FQDN("host0.example")})},
	}, append(defensiveRenderingScenarios(), defensiveWAFRenderingScenarios()...)...) {
		for _, verbosity := range []string{"quiet", "normal"} {
			notification := Compose(NewReportingState(false).Assess(scenario.message), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification()
			kind := notifier.KindUpdateFailure
			if scenario.message.cleanup {
				kind = notifier.KindCleanupFailure
			}
			if strings.HasPrefix(scenario.name, "defensive-waf-") && scenario.message.waf[0].result.Code != setter.ResponseFailed {
				kind = notifier.KindUpdate
				if scenario.message.cleanup {
					kind = notifier.KindCleanup
				}
			}
			require.Equal(t, kind, notification.Kind, scenario.name)
			require.Len(t, notification.Message, 1)
			events := []event{{Channel: "notifier", Text: notification.Format(), Kind: notification.Kind}}
			if !strings.HasPrefix(scenario.name, "defensive-") {
				require.False(t, scenario.message.HeartbeatMessage.OK)
				events = append([]event{{Channel: "heartbeat-ping", Text: scenario.message.HeartbeatMessage.Format(), Kind: ""}}, events...)
			}
			captures = append(captures, capture{
				Scenario: scenario.name, Verbosity: verbosity,
				Source:    "direct semantic evidence and composition; no provider, setter execution, CLI diagnostics, or lifecycle loop; defensive fixtures do not establish service admissibility",
				CLIBefore: []string{}, CLIAfter: []string{},
				Events: events,
			})
		}
	}
	data, err := json.MarshalIndent(captures, "", "  ")
	require.NoError(t, err)
	t.Logf("Direct semantic problem capture matrix:\n%s", data)
}
