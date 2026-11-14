package main

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestOnceNoopSkipsNotification(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	hb := mocks.NewMockHeartbeat(ctrl)
	nt := mocks.NewMockNotifier(ctrl)
	hb.EXPECT().Ping(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
	nt.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, pp.PP, notifier.Notification) bool {
		t.Error("empty one-shot operation must not submit a notification")
		return true
	}).MaxTimes(1)
	var cfg config.UpdateConfig
	require.True(t, runOnceUpdate(t.Context(), t.Context(), pp.NewSilent(), &cfg, hb, nt, mocks.NewMockSetter(ctrl)))
}

// Lifecycle assertions observe the actual shared loop, rather than reconstructing
// lifecycle composition in a test. Reporter failures in this harness are deliberate.
func TestScheduledNotificationOrder(t *testing.T) {
	t.Parallel()
	for _, scenario := range notificationScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			capture := renderNotificationScenario(t, scenario, pp.Quiet)
			var events []string
			var notifications []notificationEvent
			for _, event := range capture.Events {
				if event.Channel != "cli" {
					events = append(events, event.Channel)
				}
				if event.Channel == "notifier" {
					notifications = append(notifications, event)
				}
			}
			require.Equal(t, expectedNotificationChannels(scenario), events, "complete heartbeat/notifier event order")
			require.Equal(t, "heartbeat-start", events[0])
			if scenario.assertNotifications != nil {
				scenario.assertNotifications(t, notifications)
			}
			if scenario.once {
				require.Equal(t, "heartbeat-ping", events[1])
				if len(notifications) > 0 {
					require.Equal(t, "notifier", events[2])
				}
				return
			}
			if !scenario.deferred {
				require.Equal(t, []string{"heartbeat-start", "heartbeat-ping", "notifier"}, events[:3])
			} else {
				require.Equal(t, []string{"heartbeat-start", "notifier", "heartbeat-ping"}, events[:3])
			}
			if scenario.cleanup {
				require.Len(t, notifications, 2, "cleanup and terminal lifecycle share one submission")
				lines := 1 + len(scenario.cleanupResults)
				if scenario.wafCleanup != setter.ResponseNoop || scenario.wantWAFCleanupNotification {
					lines++
				}
				require.Len(t, strings.Split(notifications[1].Text, "\n\n"), lines, "lifecycle and constituent cleanup outcomes are retained")
				require.Equal(t, "heartbeat-log", events[len(events)-3])
			}
			if scenario.schedulingFailure {
				require.Equal(t, "heartbeat-ping", events[len(events)-2])
			} else {
				require.Equal(t, "heartbeat-exit", events[len(events)-2])
			}
			require.Equal(t, "notifier", events[len(events)-1])
		})
	}
}

func TestFirstInterruptionPrecedesSchedulingFailure(t *testing.T) {
	t.Parallel()
	//nolint:exhaustruct_v5 // This fixture isolates cancellation with a zero next deadline.
	scenario := notificationScenario{name: "interrupted-zero-next", schedulingFailure: true, rounds: []notificationRound{{results: []setter.DNSResult{{Code: setter.ResponseFailed}}, cancel: true}}}
	capture := renderNotificationScenario(t, scenario, pp.Quiet)
	var kinds []notifier.Kind
	for _, event := range capture.Events {
		if event.Channel == "notifier" {
			kinds = append(kinds, event.Kind)
		}
	}
	require.Equal(t, []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}, kinds)
	require.Zero(t, capture.Exit, "successful shutdown is independent of interrupted update")
}

// The matrix explicitly expects one ping per scheduled round, with notification
// suppression only for the repeated ordinary no-op. Terminal work is a separate
// log followed by Exit (or the scheduling-failure ping) and one submission.
func expectedNotificationChannels(scenario notificationScenario) []string {
	channels := []string{"heartbeat-start"}
	if scenario.once {
		channels = append(channels, "heartbeat-ping")
		if len(scenario.wantKinds) > 0 {
			channels = append(channels, "notifier")
		}
		return channels
	}
	if scenario.deferred {
		channels = append(channels, "notifier")
	}
	for round := range scenario.rounds {
		channels = append(channels, "heartbeat-ping")
		if scenario.deferred && round == 0 || scenario.rounds[round].suppressNotification {
			continue
		}
		channels = append(channels, "notifier")
	}
	if scenario.cleanup {
		channels = append(channels, "heartbeat-log")
	}
	if scenario.schedulingFailure {
		channels = append(channels, "heartbeat-ping")
	} else {
		channels = append(channels, "heartbeat-exit")
	}
	return append(channels, "notifier")
}

func TestNotificationEvidenceGuard(t *testing.T) {
	t.Parallel()
	before := notificationSnapshot("192.0.2.1")
	targets := []netip.Addr{netip.MustParseAddr("192.0.2.2")}
	valid := notificationResult(setter.ResponseUpdated, before, api.RecordUpdate, api.MutationCompleted, "192.0.2.2")
	wrongID := valid
	wrongID.Effects = append([]api.RecordMutation(nil), valid.Effects...)
	wrongID.Effects[0].Record.ID = "unrelated-record"
	duplicate := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.2")
	duplicate.Effects = append(duplicate.Effects, duplicate.Effects[0])
	for _, tc := range []struct {
		name    string
		result  setter.DNSResult
		targets []netip.Addr
		valid   bool
	}{
		{name: "correct-update", result: valid, targets: targets, valid: true},
		{name: "correct-empty-noop", result: notificationNoop(notificationSnapshot()), targets: nil, valid: true},
		{name: "noop-target-mismatch", result: notificationNoop(before, "192.0.2.1"), targets: targets, valid: false},
		{name: "unrelated-mutation-identity", result: wrongID, targets: targets, valid: false},
		{name: "duplicate-created-identity", result: duplicate, targets: targets, valid: false},
		{name: "mutation-target-mismatch", result: valid, targets: []netip.Addr{netip.MustParseAddr("192.0.2.3")}, valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			problem := notificationEvidenceProblem(tc.result, tc.targets)
			if tc.valid {
				require.Empty(t, problem)
			} else {
				require.NotEmpty(t, problem)
			}
		})
	}
}

// Invalid progress must never reach the operator review as apparently faithful
// setter evidence, even when the older mutation-only guard would admit it.
func TestNotificationProgressEvidenceGuard(t *testing.T) {
	t.Parallel()
	before := notificationSnapshot("192.0.2.1")
	target := netip.MustParseAddr("192.0.2.2")
	operation := setter.DNSOperation{Kind: api.RecordUpdate, Record: api.RecordFact{ID: before.Records[0].ID, IP: target}, Status: setter.DNSOperationCompleted, Membership: api.MembershipManaged}
	effect := api.RecordMutation{Kind: api.RecordUpdate, Record: operation.Record, Status: api.MutationCompleted, Membership: api.MembershipManaged}
	for _, tc := range []struct {
		name     string
		before   *setter.DNSSnapshot
		progress *setter.DNSProgress
		effects  []api.RecordMutation
	}{
		{name: "missing-progress", before: before, progress: nil, effects: []api.RecordMutation{effect}},
		{name: "progress-without-read", before: nil, progress: &setter.DNSProgress{Satisfied: nil, Operations: nil}, effects: nil},
		{name: "satisfied-target-not-observed", before: before, progress: &setter.DNSProgress{Satisfied: []netip.Addr{target}, Operations: nil}, effects: nil},
		{name: "completed-without-effect", before: before, progress: &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{operation}}, effects: nil},
		{name: "blocked-with-effect", before: before, progress: &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{{Kind: operation.Kind, Record: operation.Record, Status: setter.DNSOperationBlocked, Membership: api.MembershipUnknown}}}, effects: []api.RecordMutation{effect}},
		{name: "delete-address-mismatch", before: before, progress: &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{{Kind: api.RecordDelete, Record: operation.Record, Status: setter.DNSOperationNotAttempted, Membership: api.MembershipUnknown}}}, effects: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := setter.DNSResult{Code: setter.ResponseFailed, Before: tc.before, Progress: tc.progress, Effects: tc.effects}
			require.NotEmpty(t, notificationEvidenceProblem(result, []netip.Addr{target}))
		})
	}
}

// Cancellation stops ordinary work, but cleanup and its terminal reports use the
// surviving reporting context. The capture harness makes all reporters fail.
func TestInterruptedCleanupReportingPreservesWorkResult(t *testing.T) {
	t.Parallel()
	for _, failed := range []bool{false, true} {
		code, mutation, kind, exit := setter.ResponseUpdated, api.MutationCompleted, notifier.KindShutdown, 0
		if failed {
			code, mutation, kind, exit = setter.ResponseFailed, api.MutationUnconfirmed, notifier.KindCleanupFailure, 1
		}
		var scenario notificationScenario
		scenario.name = "interrupted-cleanup"
		scenario.rounds = []notificationRound{{suppressNotification: false, waf: nil, results: []setter.DNSResult{{Code: setter.ResponseFailed, Before: nil, Progress: nil, Effects: nil}}, targets: "", targets6: "", unavailable: nil, cancel: true}}
		scenario.cleanup = true
		scenario.cleanupResults = []setter.DNSResult{notificationResult(code, notificationSnapshot("192.0.2.1"), api.RecordDelete, mutation, "192.0.2.1")}
		capture := renderNotificationScenario(t, scenario, pp.Quiet)
		var channels []string
		var kinds []notifier.Kind
		for _, event := range capture.Events {
			if event.Channel != "cli" {
				channels = append(channels, event.Channel)
			}
			if event.Channel == "notifier" {
				kinds = append(kinds, event.Kind)
			}
		}
		require.Equal(t, []string{"heartbeat-start", "heartbeat-ping", "notifier", "heartbeat-log", "heartbeat-exit", "notifier"}, channels)
		require.Equal(t, []notifier.Kind{notifier.KindUpdateFailure, kind}, kinds)
		require.Equal(t, exit, capture.Exit)
	}
}

// Cleanup status and content must survive the move to caller-owned reporting.
// Delivery failures cannot turn successful cleanup into a failed exit.
func TestScheduledCleanupHeartbeatResult(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		code setter.ResponseCode
		ok   bool
		text string
	}{
		{name: "completed", code: setter.ResponseUpdated, ok: true, text: "Cleaned WAF list(s) account/list"},
		{name: "accepted", code: setter.ResponseUpdating, ok: true, text: "Cleaning WAF list(s) account/list"},
		{name: "failed", code: setter.ResponseFailed, ok: false, text: "Could not confirm cleanup of WAF list(s) account/list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			hb := mocks.NewMockHeartbeat(ctrl)
			nt := mocks.NewMockNotifier(ctrl)
			s := mocks.NewMockSetter(ctrl)
			ppfmt := pp.NewSilent()
			workCtx, cancel := context.WithCancel(t.Context())
			cancel()
			reportCtx := t.Context()
			schedule := &notificationSchedule{round: 0, zero: false, beforeWork: false}
			list := api.WAFList{AccountID: "account", Name: "list"}
			var cfg config.UpdateConfig
			cfg.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewStaticEmpty()}
			cfg.WAFLists = []api.WAFList{list}
			cfg.UpdateTimeout = time.Second
			cleanup := notificationWAFCleanup(tc.code, false)
			gomock.InOrder(
				nt.EXPECT().Send(reportCtx, ppfmt, gomock.Any()).Return(false),
				hb.EXPECT().Ping(reportCtx, ppfmt, gomock.Any()).Return(false),
				s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, "", map[ipnet.Family]bool{ipnet.IP4: true}, api.CleanupAllowAsync).DoAndReturn(
					func(ctx context.Context, _ pp.PP, _ api.WAFList, _ string, _ map[ipnet.Family]bool, _ api.CleanupMode) setter.WAFResult {
						require.NoError(t, ctx.Err())
						return cleanup
					}),
				hb.EXPECT().Log(reportCtx, ppfmt, gomock.Any()).DoAndReturn(
					func(ctx context.Context, _ pp.PP, message heartbeat.Message) bool {
						require.NoError(t, ctx.Err())
						require.Equal(t, tc.ok, message.OK)
						require.Equal(t, tc.text, message.Format())
						return false
					}),
				hb.EXPECT().Exit(reportCtx, ppfmt, "Stopped").Return(false),
				nt.EXPECT().Send(reportCtx, ppfmt, gomock.Any()).Return(false),
			)
			exit := runScheduled(workCtx, reportCtx, ppfmt,
				&config.LifecycleConfig{UpdateCron: schedule, UpdateOnStart: false, DeleteOnStop: true},
				&cfg, hb, nt, s, notificationWaiter{schedule: schedule, rounds: 1})
			if tc.ok {
				require.Zero(t, exit)
			} else {
				require.Equal(t, 1, exit)
			}
		})
	}
}
