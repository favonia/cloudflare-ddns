package updater

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// renderingScenario carries its expected output and checks explicitly; name is
// used only to identify the case in test reports and capture metadata.
type renderingScenario struct {
	name             string
	message          Message
	wantKind         notifier.Kind
	captureHeartbeat bool
	assertMessage    func(*testing.T, Message)
}

// These consumer-contract fixtures do not assert remote duplicate admissibility
// or ordinary setter reachability for missing/conflicting evidence.
func defensiveRenderingScenarios() []renderingScenario {
	cases := make([]renderingScenario, 0, 6)
	for _, tc := range []struct {
		name      string
		configure func(*dnsOutcome)
		wantKind  notifier.Kind
		check     func(*testing.T, Message)
	}{
		{name: "other-anchor", configure: func(_ *dnsOutcome) {}, wantKind: notifier.KindUpdateFailure, check: func(t *testing.T, message Message) {
			t.Helper()
			text := compose(snapshotAssessment(message), NewNoLifecycleContext()).notificationValue().Format()
			require.Contains(t, text, "(1 other record)")
			require.Contains(t, text, "192.0.2.1 (cached) →")
		}},
		{name: "cleanup-no-anchor", configure: func(o *dnsOutcome) {
			o.cleanup = true
			o.target = nil
			o.result.Progress.Operations = []setter.DNSOperation{o.result.Progress.Operations[1], reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "updated", "192.0.2.1"), reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "unvisited", "192.0.2.1")}
		}, wantKind: notifier.KindCleanupFailure, check: func(t *testing.T, message Message) {
			t.Helper()
			text := compose(snapshotAssessment(message), NewNoLifecycleContext()).notificationValue().Format()
			require.NotContains(t, text, "other record")
			require.Contains(t, text, "(1 record)")
		}},
		{name: "missing-update-identity", configure: func(o *dnsOutcome) {
			o.result.Progress.Operations[0].Record.ID = ""
		}, wantKind: notifier.KindUpdateFailure, check: func(t *testing.T, message Message) {
			t.Helper()
			text := compose(snapshotAssessment(message), NewNoLifecycleContext()).notificationValue().Format()
			require.NotContains(t, text, "other record")
			require.Contains(t, text, "(1 record)")
		}},
		{name: "conflicting-removal-identity", configure: func(o *dnsOutcome) {
			o.result.Progress.Operations[1].Record.IP = netip.MustParseAddr("192.0.2.99")
		}, wantKind: notifier.KindUpdateFailure, check: func(t *testing.T, message Message) {
			t.Helper()
			text := compose(snapshotAssessment(message), NewNoLifecycleContext()).notificationValue().Format()
			require.NotContains(t, text, "192.0.2.99 (1")
			require.NotContains(t, text, "Removal unconfirmed (cached)")
		}},
		{name: "unknown-multiplicity", configure: func(o *dnsOutcome) {
			o.result.Before = nil
			o.result.Progress.Operations = []setter.DNSOperation{o.result.Progress.Operations[1]}
		}, wantKind: notifier.KindUpdateFailure, check: func(t *testing.T, message Message) {
			t.Helper()
			text := compose(snapshotAssessment(message), NewNoLifecycleContext()).notificationValue().Format()
			require.NotContains(t, text, "other record")
			require.Contains(t, text, "(1 record)")
		}},
		{name: "unique-singleton", configure: func(o *dnsOutcome) {
			o.result.Before.Records = o.result.Before.Records[1:2]
			o.result.Progress.Operations = []setter.DNSOperation{o.result.Progress.Operations[1]}
		}, wantKind: notifier.KindUpdateFailure, check: func(t *testing.T, message Message) {
			t.Helper()
			text := compose(snapshotAssessment(message), NewNoLifecycleContext()).notificationValue().Format()
			require.NotContains(t, text, "record)")
		}},
		{name: "same-ip-comment-exclusion", configure: func(o *dnsOutcome) {
			o.filtered = true
			excluded := o.result.Progress.Operations[0]
			excluded.Membership = api.MembershipUnmanaged
			other := reportingOperation(api.RecordCreate, setter.DNSOperationUnconfirmed, "", "192.0.2.2")
			o.result.Progress.Operations = []setter.DNSOperation{excluded, other}
		}, wantKind: notifier.KindUpdateFailure, check: func(t *testing.T, message Message) {
			t.Helper()
			text := compose(snapshotAssessment(message), NewNoLifecycleContext()).notificationValue().Format()
			require.Contains(t, text, "Update confirmed:")
			require.Contains(t, text, "Addition unconfirmed:")
			require.Equal(t, 1, strings.Count(text, "excluded from management"))
			require.Less(t, strings.Index(text, "Update confirmed:"), strings.Index(text, "excluded from management"))
			require.Less(t, strings.Index(text, "excluded from management"), strings.Index(text, "Addition unconfirmed:"))
		}},
	} {
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
		tc.configure(&o)
		message := reportingMessage(o)
		message.cleanup = o.cleanup
		cases = append(cases, renderingScenario{name: "defensive-" + tc.name, message: message, wantKind: tc.wantKind, captureHeartbeat: false, assertMessage: tc.check})
	}
	return cases
}

func TestRenderDefensiveCountQualifications(t *testing.T) {
	t.Parallel()
	for _, scenario := range defensiveRenderingScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			scenario.assertMessage(t, scenario.message)
		})
	}
}
