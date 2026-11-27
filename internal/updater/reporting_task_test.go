package updater_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

type reportingSink struct {
	notifications []notifier.Notification
}

func (*reportingSink) Describe(func(string, string) bool) {}

func (s *reportingSink) Send(_ context.Context, _ pp.PP, notification notifier.Notification) bool {
	s.notifications = append(s.notifications, notification)
	return false // Failed delivery still constitutes submission.
}

func TestReportOneShotTask(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		result  setter.DNSResult
		cleanup bool
		want    notifier.Kind
		empty   bool
	}{
		{"no-op", reportingNoop("192.0.2.2"), false, "", true},
		{"updated", reportingUpdateResult(api.MutationCompleted), false, notifier.KindUpdate, false},
		{"failed", reportingReadFailure(), false, notifier.KindUpdateFailure, false},
		{"cleanup-failed", reportingReadFailure(), true, notifier.KindCleanupFailure, false},
		{"cleanup-completed", reportingCompletedDeletion(), true, notifier.KindCleanup, false},
		{"cleanup-no-op", reportingNoop(""), true, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var message updater.Message
			if tc.cleanup {
				message = reportingCleanup(t, tc.result)
			} else {
				message = reportingUpdate(t, "192.0.2.2", map[domain.Domain]setter.DNSResult{domain.FQDN("a.example"): tc.result})
			}
			sink := &reportingSink{notifications: nil}
			updater.ReportOneShot(t.Context(), pp.NewSilent(), sink, message)
			if tc.empty {
				require.Empty(t, sink.notifications)
				return
			}
			require.Len(t, sink.notifications, 1)
			first := sink.notifications[0]
			require.Equal(t, tc.want, first.Kind)
			require.NotContains(t, first.Format(), "Cloudflare DDNS started")
			require.NotContains(t, first.Format(), "Cloudflare DDNS stopped")
			updater.ReportOneShot(t.Context(), pp.NewSilent(), sink, message)
			require.Len(t, sink.notifications, 2)
			require.Equal(t, first, sink.notifications[1])
			first.Message[0] = "changed by recipient"
			updater.ReportOneShot(t.Context(), pp.NewSilent(), sink, message)
			require.Len(t, sink.notifications, 3)
			require.NotContains(t, sink.notifications[2].Format(), "changed by recipient")
		})
	}
}

func TestScheduledReporterSubmissionOwnsCoverage(t *testing.T) {
	t.Parallel()
	sink := &reportingSink{notifications: nil}
	reporter := updater.NewScheduledReporter(sink)
	lifecycle := updater.NewNoLifecycleContext()
	a := domain.FQDN("a.example")
	b := domain.FQDN("b.example")
	first := reportingUpdate(t, "192.0.2.1", map[domain.Domain]setter.DNSResult{a: reportingNoop("192.0.2.1"), b: reportingReadFailure()})
	reporter.Report(t.Context(), pp.NewSilent(), &first, lifecycle)
	require.Len(t, sink.notifications, 1)
	require.Equal(t, notifier.KindUpdateFailure, sink.notifications[0].Kind)
	second := reportingUpdate(t, "192.0.2.1", map[domain.Domain]setter.DNSResult{a: reportingNoop("192.0.2.1"), b: reportingNoop("192.0.2.1")})
	reporter.Report(t.Context(), pp.NewSilent(), &second, lifecycle)
	require.Len(t, sink.notifications, 2)
	require.Equal(t, notifier.KindStateObservation, sink.notifications[1].Kind)
	require.Contains(t, sink.notifications[1].Format(), "b.example")
	require.NotContains(t, sink.notifications[1].Format(), "a.example")
	third := reportingUpdate(t, "192.0.2.1", map[domain.Domain]setter.DNSResult{a: reportingNoop("192.0.2.1"), b: reportingNoop("192.0.2.1")})
	reporter.Report(t.Context(), pp.NewSilent(), &third, lifecycle)
	require.Len(t, sink.notifications, 2)
}

func TestScheduledReporterObservesSilentRounds(t *testing.T) {
	t.Parallel()
	sink := &reportingSink{notifications: nil}
	reporter := updater.NewScheduledReporter(sink)
	lifecycle := updater.NewNoLifecycleContext()
	name := domain.FQDN("a.example")
	initial := reportingUpdate(t, "192.0.2.1", map[domain.Domain]setter.DNSResult{name: reportingNoop("192.0.2.1")})
	reporter.Report(t.Context(), pp.NewSilent(), &initial, lifecycle)
	attempted := reportingUpdate(t, "192.0.2.2", map[domain.Domain]setter.DNSResult{name: reportingUpdateResult(api.MutationUnconfirmed)})
	reporter.Report(t.Context(), pp.NewSilent(), &attempted, lifecycle)
	require.Len(t, sink.notifications, 2)
	after := reportingUpdate(t, "192.0.2.2", map[domain.Domain]setter.DNSResult{name: reportingNoop("192.0.2.2")})
	reporter.Report(t.Context(), pp.NewSilent(), &after, lifecycle)
	require.Len(t, sink.notifications, 2)
	// The silent fresh read resolved the uncertain write. Returning to the old
	// address is now unexplained, not a still-possible result of that write.
	reverted := reportingUpdate(t, "192.0.2.1", map[domain.Domain]setter.DNSResult{name: reportingNoop("192.0.2.1")})
	reporter.Report(t.Context(), pp.NewSilent(), &reverted, lifecycle)
	require.Len(t, sink.notifications, 3)
	require.Equal(t, notifier.KindStateObservation, sink.notifications[2].Kind)
	require.Contains(t, sink.notifications[2].Format(), "Detected an unexpected address change")
}

func TestScheduledReporterLifecycleOnlyAndFailurePriority(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		lifecycle     updater.LifecycleContext
		failedCleanup bool
		want          notifier.Kind
	}{
		{"startup without update", updater.NewStartupLifecycleContext(), false, notifier.KindStartup},
		{"shutdown without cleanup", updater.NewShutdownLifecycleContext(), false, notifier.KindShutdown},
		{"scheduling failure without cleanup", updater.NewSchedulingFailureLifecycleContext("schedule"), false, notifier.KindSchedulingFailure},
		{"failed shutdown cleanup", updater.NewShutdownLifecycleContext(), true, notifier.KindCleanupFailure},
		{"failed scheduling cleanup", updater.NewSchedulingFailureLifecycleContext("schedule"), true, notifier.KindSchedulingFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sink := &reportingSink{notifications: nil}
			reporter := updater.NewScheduledReporter(sink)
			var message *updater.Message
			if tc.failedCleanup {
				cleanup := reportingCleanup(t, reportingReadFailure())
				message = &cleanup
			}
			reporter.Report(t.Context(), pp.NewSilent(), message, tc.lifecycle)
			require.Len(t, sink.notifications, 1)
			require.Equal(t, tc.want, sink.notifications[0].Kind)
			if tc.failedCleanup {
				require.Contains(t, sink.notifications[0].Format(), "a.example")
			}
		})
	}
}

func TestScheduledReporterLifecycleContext(t *testing.T) {
	t.Parallel()
	var zero updater.LifecycleContext
	for _, tc := range []struct {
		name      string
		lifecycle updater.LifecycleContext
		want      string
	}{
		{"zero value", zero, ""},
		{"no event", updater.NewNoLifecycleContext(), ""},
		{"startup", updater.NewStartupLifecycleContext(), "Cloudflare DDNS started"},
		{"shutdown", updater.NewShutdownLifecycleContext(), "Cloudflare DDNS stopped"},
		{"scheduling failure", updater.NewSchedulingFailureLifecycleContext("every 2 hours"), "Cloudflare DDNS stopped because no future updates could be scheduled with every 2 hours"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sink := &reportingSink{notifications: nil}
			reporter := updater.NewScheduledReporter(sink)
			reporter.Report(t.Context(), pp.NewSilent(), nil, tc.lifecycle)
			if tc.want == "" {
				require.Empty(t, sink.notifications)
			} else {
				require.Len(t, sink.notifications, 1)
				require.Equal(t, tc.want, sink.notifications[0].Format())
			}
		})
	}
}

func TestScheduledReporterSuppressesAbsentResultAndLifecycle(t *testing.T) {
	t.Parallel()
	sink := &reportingSink{notifications: nil}
	reporter := updater.NewScheduledReporter(sink)
	var lifecycle updater.LifecycleContext
	reporter.Report(t.Context(), pp.NewSilent(), nil, lifecycle)
	require.Empty(t, sink.notifications)
	message := reportingUpdate(t, "192.0.2.1", map[domain.Domain]setter.DNSResult{domain.FQDN("a.example"): reportingNoop("192.0.2.1")})
	reporter.Report(t.Context(), pp.NewSilent(), &message, updater.NewNoLifecycleContext())
	require.Len(t, sink.notifications, 1)
	require.Equal(t, notifier.KindStateObservation, sink.notifications[0].Kind)
}

// Reporting consumes real updater Messages. The mocked dependency supplies its
// documented result evidence; the updater owns message construction and scope.
func reportingUpdate(t *testing.T, target string, results map[domain.Domain]setter.DNSResult) updater.Message {
	t.Helper()
	conf := initUpdateConfig()
	conf.Provider[ipnet.IP4] = provider.MustNewStatic(ipnet.IP4, 32, target)
	p := pp.NewSilent()
	handle := mocks.NewMockSetter(gomock.NewController(t))
	for name, result := range results {
		conf.Domains[ipnet.IP4] = append(conf.Domains[ipnet.IP4], name)
		handle.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, name, []netip.Addr{netip.MustParseAddr(target)}, gomock.Any()).Return(result)
	}
	return updater.UpdateIPs(t.Context(), p, conf, handle)
}

func reportingCleanup(t *testing.T, result setter.DNSResult) updater.Message {
	t.Helper()
	conf := initUpdateConfig()
	conf.Provider[ipnet.IP4] = provider.NewStaticEmpty()
	name := domain.FQDN("a.example")
	conf.Domains[ipnet.IP4] = []domain.Domain{name}
	p := pp.NewSilent()
	handle := mocks.NewMockSetter(gomock.NewController(t))
	handle.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, name, gomock.Any()).Return(result)
	return updater.FinalDeleteIPs(t.Context(), p, conf, handle, api.CleanupWait)
}

func reportingReadFailure() setter.DNSResult {
	return setter.DNSResult{Code: setter.ResponseFailed, Before: nil, Progress: nil, Effects: nil}
}

func reportingNoop(address string) setter.DNSResult {
	before := &setter.DNSSnapshot{Cached: false, Records: nil}
	progress := &setter.DNSProgress{Satisfied: nil, Operations: nil}
	if address != "" {
		ip := netip.MustParseAddr(address)
		before.Records = []api.RecordFact{{ID: "record", IP: ip}}
		progress.Satisfied = []netip.Addr{ip}
	}
	return setter.DNSResult{Code: setter.ResponseNoop, Before: before, Progress: progress, Effects: nil}
}

// Only completed and unconfirmed updates are used by these public traces.
func reportingUpdateResult(status api.MutationStatus) setter.DNSResult {
	result := reportingNoop("192.0.2.1")
	result.Code = setter.ResponseFailed
	operation := setter.DNSOperation{Kind: api.RecordUpdate, Record: api.RecordFact{ID: "record", IP: netip.MustParseAddr("192.0.2.2")}, Status: setter.DNSOperationUnconfirmed, Membership: api.MembershipUnknown}
	if status == api.MutationCompleted {
		result.Code = setter.ResponseUpdated
		operation.Status = setter.DNSOperationCompleted
		operation.Membership = api.MembershipManaged
	}
	result.Progress = &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{operation}}
	result.Effects = []api.RecordMutation{{Kind: operation.Kind, Status: status, Record: operation.Record, Membership: operation.Membership}}
	return result
}

func reportingCompletedDeletion() setter.DNSResult {
	result := reportingNoop("192.0.2.1")
	result.Code = setter.ResponseUpdated
	result.Progress = &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{{Kind: api.RecordDelete, Record: result.Before.Records[0], Status: setter.DNSOperationCompleted, Membership: api.MembershipUnknown}}}
	result.Effects = []api.RecordMutation{{Kind: api.RecordDelete, Status: api.MutationCompleted, Record: api.RecordFact{ID: "record", IP: netip.Addr{}}, Membership: api.MembershipUnknown}}
	return result
}

func TestReportOneShotRetainsWAFEmptyFallbackAttempt(t *testing.T) {
	t.Parallel()
	conf := initUpdateConfig()
	conf.Provider[ipnet.IP4] = provider.NewStaticEmpty()
	conf.Provider[ipnet.IP6] = provider.NewStaticEmpty()
	list := api.WAFList{AccountID: "account", Name: "list"}
	conf.WAFLists = []api.WAFList{list}
	before := &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}
	result := setter.WAFResult{Code: setter.ResponseNoop, Before: before, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: api.WAFListCleanupNoop, Before: before, Stages: []api.WAFCleanupStage{{WholeList: true, Items: nil, Result: api.WAFBatchResult{ListCreationDispatched: false, OK: false, Status: api.WAFBatchUnconfirmed, ReturnedManaged: nil, ReturnedAllItems: nil}}}}}
	p := pp.NewSilent()
	handle := mocks.NewMockSetter(gomock.NewController(t))
	handle.EXPECT().FinalClearWAFList(gomock.Any(), p, list, conf.WAFListDescription, map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}, api.CleanupWait).Return(result)
	message := updater.FinalDeleteIPs(t.Context(), p, conf, handle, api.CleanupWait)
	require.False(t, message.Failed())
	sink := &reportingSink{notifications: nil}
	updater.ReportOneShot(t.Context(), p, sink, message)
	require.Len(t, sink.notifications, 1)
	notification := sink.notifications[0]
	require.Equal(t, notifier.KindCleanup, notification.Kind)
	require.Contains(t, notification.Format(), "List deletion unconfirmed")
	require.Contains(t, notification.Format(), "Current: no items")
}
