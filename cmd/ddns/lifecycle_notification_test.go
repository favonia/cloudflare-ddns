package main

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
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
				require.Contains(t, notifications[1].Text, "cleanup=true")
				require.Equal(t, "heartbeat-log", events[len(events)-3])
			}
			if scenario.schedulingFailure {
				require.Equal(t, "heartbeat-ping", events[len(events)-2])
			} else {
				require.Equal(t, "heartbeat-exit", events[len(events)-2])
			}
			require.Equal(t, "notifier", events[len(events)-1])
			if scenario.name == "immediate-startup-noop" {
				require.Contains(t, notifications[0].Text, "before-known=true")
				require.Contains(t, notifications[0].Text, "dns domains=")
			}
			if scenario.name == "grouped-updates" {
				require.Equal(t, 1, strings.Count(notifications[0].Text, "dns domains="))
				require.Contains(t, notifications[0].Text, "host0.example")
				require.Contains(t, notifications[0].Text, "host1.example")
			}
			if scenario.name == "distinct-previous-sets" {
				require.Equal(t, 2, strings.Count(notifications[0].Text, "dns domains="))
			}
			if strings.HasPrefix(scenario.name, "unexplained-observation-then-") {
				require.Contains(t, notifications[1].Text, "unexplained=true")
			}
			if scenario.name == "deferred-startup-A-then-B" {
				require.NotContains(t, notifications[0].Text, "dns domains=")
				require.Contains(t, notifications[1].Text, "family=4")
				require.NotContains(t, notifications[2].Text, "family=4")
				require.Contains(t, notifications[2].Text, "family=6")
			}
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
		if scenario.deferred && round == 0 || scenario.name == "ordinary-silence" && round > 0 {
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
		{name: "correct-empty-noop", result: setter.DNSResult{Code: setter.ResponseNoop, Before: notificationSnapshot(), Effects: nil}, targets: nil, valid: true},
		{name: "noop-target-mismatch", result: setter.DNSResult{Code: setter.ResponseNoop, Before: before, Effects: nil}, targets: targets, valid: false},
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
