package updater

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type renderingScenario struct {
	name    string
	message Message
}

// These consumer-contract fixtures do not assert remote duplicate admissibility
// or ordinary setter reachability for missing/conflicting evidence.
func defensiveRenderingScenarios() []renderingScenario {
	cases := make([]renderingScenario, 0, 6)
	for _, variant := range []string{"other-anchor", "cleanup-no-anchor", "missing-update-identity", "conflicting-removal-identity", "unknown-multiplicity", "unique-singleton", "same-ip-comment-exclusion"} {
		before := &setter.DNSSnapshot{Cached: true, Records: []api.RecordFact{
			{ID: "updated", IP: netip.MustParseAddr("192.0.2.1")},
			{ID: "removed", IP: netip.MustParseAddr("192.0.2.1")},
			{ID: "unvisited", IP: netip.MustParseAddr("192.0.2.1")},
		}}
		o := reportingOutcome("defensive.example", setter.ResponseFailed, before)
		update := reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, "updated", "192.0.2.2")
		update.Membership = api.MembershipManaged
		removal := reportingOperation(api.RecordDelete, setter.DNSOperationUnconfirmed, "removed", "192.0.2.1")
		o.result.Progress.Operations = []setter.DNSOperation{update, removal, reportingOperation(api.RecordDelete, setter.DNSOperationNotAttempted, "unvisited", "192.0.2.1")}
		switch variant {
		case "other-anchor":
		case "cleanup-no-anchor":
			o.cleanup = true
			o.target = nil
			o.result.Progress.Operations = []setter.DNSOperation{removal, reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "updated", "192.0.2.1"), reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "unvisited", "192.0.2.1")}
		case "missing-update-identity":
			o.result.Progress.Operations[0].Record.ID = ""
		case "conflicting-removal-identity":
			o.result.Progress.Operations[1].Record.IP = netip.MustParseAddr("192.0.2.99")
		case "unknown-multiplicity":
			o.result.Before = nil
			o.result.Progress.Operations = []setter.DNSOperation{removal}
		case "same-ip-comment-exclusion":
			o.filtered = true
			excluded := update
			excluded.Membership = api.MembershipUnmanaged
			other := reportingOperation(api.RecordCreate, setter.DNSOperationUnconfirmed, "", "192.0.2.2")
			o.result.Progress.Operations = []setter.DNSOperation{excluded, other}
		case "unique-singleton":
			o.result.Before.Records = o.result.Before.Records[1:2]
			o.result.Progress.Operations = []setter.DNSOperation{removal}
		}
		message := reportingMessage(o)
		message.cleanup = o.cleanup
		cases = append(cases, renderingScenario{name: "defensive-" + variant, message: message})
	}
	return cases
}

func TestRenderDefensiveCountQualifications(t *testing.T) {
	t.Parallel()
	for _, scenario := range defensiveRenderingScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			text := Compose(NewReportingState(false).Assess(scenario.message), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
			switch scenario.name {
			case "defensive-other-anchor":
				require.Contains(t, text, "(1 other record)")
				require.Contains(t, text, "192.0.2.1 (cached) →")
			case "defensive-cleanup-no-anchor", "defensive-missing-update-identity", "defensive-unknown-multiplicity":
				require.NotContains(t, text, "other record")
				require.Contains(t, text, "(1 record)")
			case "defensive-conflicting-removal-identity":
				require.NotContains(t, text, "192.0.2.99 (1")
				require.NotContains(t, text, "Removal unconfirmed (cached)")
			case "defensive-same-ip-comment-exclusion":
				require.Contains(t, text, "Update confirmed:")
				require.Contains(t, text, "Addition unconfirmed:")
				require.Equal(t, 1, strings.Count(text, "excluded from management"))
				require.Less(t, strings.Index(text, "Update confirmed:"), strings.Index(text, "excluded from management"))
				require.Less(t, strings.Index(text, "excluded from management"), strings.Index(text, "Addition unconfirmed:"))
			case "defensive-unique-singleton":
				require.NotContains(t, text, "record)")
			}
		})
	}
}
