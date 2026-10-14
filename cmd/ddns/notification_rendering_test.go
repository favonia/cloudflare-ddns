package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipfilter"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// These fixtures supply setter evidence through its public contract. The actual
// updater, scheduled loop, composition and printer remain in the exercised path.
type notificationRound struct {
	results     []setter.DNSResult
	targets     string
	unavailable map[ipnet.Family]bool
	cancel      bool
}
type notificationScenario struct {
	name              string
	rounds            []notificationRound
	targets           string
	deferred          bool
	once              bool
	cleanup           bool
	cleanupResults    []setter.DNSResult
	wafCleanup        setter.ResponseCode
	schedulingFailure bool
	families          []ipnet.Family
	wantKinds         []notifier.Kind
	wantExit          int
}
type notificationEvent struct {
	Channel string        `json:"channel"`
	Text    string        `json:"text"`
	Kind    notifier.Kind `json:"kind,omitempty"`
}
type notificationCapture struct {
	Scenario  string              `json:"scenario"`
	Verbosity string              `json:"verbosity"`
	Events    []notificationEvent `json:"events"`
	Exit      int                 `json:"exit"`
}
type notificationRecorder struct{ events []notificationEvent }

func (r *notificationRecorder) Write(data []byte) (int, error) {
	r.add("cli", string(data), "")
	return len(data), nil
}

func (r *notificationRecorder) add(channel, text string, kind notifier.Kind) {
	r.events = append(r.events, notificationEvent{Channel: channel, Text: text, Kind: kind})
}

type notificationSchedule struct {
	round      int
	zero       bool
	beforeWork bool
}

func (s *notificationSchedule) Next() time.Time {
	s.round++
	s.beforeWork = true
	if s.zero {
		return time.Time{}
	}
	return time.Now().Add(time.Hour)
}
func (*notificationSchedule) Describe() string { return "@every 1h" }

type notificationWaiter struct {
	schedule *notificationSchedule
	rounds   int
}

func (w notificationWaiter) WaitForSignalsUntil(ppfmt pp.PP, _ time.Time) bool {
	if w.schedule.round < w.rounds {
		return false
	}
	ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v", syscall.SIGTERM)
	return true
}

type notificationProvider struct {
	schedule *notificationSchedule
	rounds   []notificationRound
	family   ipnet.Family
	fallback provider.Provider
}

func (p notificationProvider) Name() string          { return p.fallback.Name() }
func (p notificationProvider) IsExplicitEmpty() bool { return p.fallback.IsExplicitEmpty() }
func (p notificationProvider) GetRawData(ctx context.Context, printer pp.PP, family ipnet.Family, prefix int) provider.DetectionResult {
	round := max(p.schedule.round-1, 0)
	if round < len(p.rounds) && p.rounds[round].unavailable[p.family] {
		return provider.NewUnavailableDetectionResult()
	}
	if round < len(p.rounds) && p.rounds[round].targets != "" && family == ipnet.IP4 {
		return provider.MustNewStatic(family, prefix, p.rounds[round].targets).GetRawData(ctx, printer, family, prefix)
	}
	return p.fallback.GetRawData(ctx, printer, family, prefix)
}

func notificationSnapshot(addresses ...string) *setter.DNSSnapshot {
	snapshot := &setter.DNSSnapshot{Cached: false, Records: nil}
	for _, address := range addresses {
		snapshot.Records = append(snapshot.Records, api.RecordFact{ID: api.ID("record-" + address), IP: netip.MustParseAddr(address)})
	}
	return snapshot
}

func notificationResult(code setter.ResponseCode, before *setter.DNSSnapshot, kind api.RecordMutationKind, status api.MutationStatus, address string) setter.DNSResult {
	result := setter.DNSResult{Code: code, Before: before, Effects: nil}
	if address != "" {
		result.Effects = []api.RecordMutation{{Kind: kind, Status: status, Record: api.RecordFact{ID: api.ID("created-" + address), IP: netip.MustParseAddr(address)}, Membership: api.MembershipManaged}}
		if kind != api.RecordCreate && before != nil && len(before.Records) > 0 {
			result.Effects[0].Record.ID = before.Records[0].ID
		}
		if kind == api.RecordDelete {
			result.Effects[0].Record.IP = netip.Addr{}
			result.Effects[0].Membership = api.MembershipUnknown
		}
	}
	return result
}

func notificationScenarios() []notificationScenario {
	old := notificationSnapshot("192.0.2.1")
	fresh := notificationSnapshot("192.0.2.2")
	noop := setter.DNSResult{Code: setter.ResponseNoop, Before: old, Effects: nil}
	created := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.1")
	cleared := notificationResult(setter.ResponseUpdated, old, api.RecordDelete, api.MutationCompleted, "192.0.2.1")
	failed := setter.DNSResult{Code: setter.ResponseFailed, Before: fresh, Effects: nil}
	historical := notificationSnapshot("192.0.2.1")
	historical.Cached = true
	uncertain := notificationResult(setter.ResponseFailed, historical, api.RecordUpdate, api.MutationUnconfirmed, "192.0.2.2")
	multi := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.1")
	second := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.2")
	multi.Effects = append(multi.Effects, second.Effects...)
	groupedSecond := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.1")
	groupedSecond.Effects[0].Record.ID = "created-second-domain"
	scenarios := make([]notificationScenario, 0, 25)
	//nolint:exhaustruct_v5 // Matrix fixtures specify relevant variations; omitted options use their zero defaults.
	scenarios = append(scenarios, []notificationScenario{
		{name: "immediate-startup-noop", rounds: []notificationRound{{results: []setter.DNSResult{noop}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "immediate-startup-creation", rounds: []notificationRound{{results: []setter.DNSResult{created}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "immediate-startup-clearing", targets: "empty", rounds: []notificationRound{{results: []setter.DNSResult{cleared}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "immediate-startup-mixed-success-failure", rounds: []notificationRound{{results: []setter.DNSResult{created, failed}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "immediate-startup-detection-failure", rounds: []notificationRound{{unavailable: map[ipnet.Family]bool{ipnet.IP4: true}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "immediate-startup-interruption", rounds: []notificationRound{{results: []setter.DNSResult{failed}, cancel: true}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "deferred-startup-A-then-B", deferred: true, families: []ipnet.Family{ipnet.IP4, ipnet.IP6}, rounds: []notificationRound{{}, {results: []setter.DNSResult{noop}, unavailable: map[ipnet.Family]bool{ipnet.IP6: true}}, {results: []setter.DNSResult{noop, {Code: setter.ResponseNoop, Before: notificationSnapshot("2001:db8::1")}}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindUpdateFailure, notifier.KindStateObservation, notifier.KindShutdown}},
		{name: "ordinary-silence", rounds: []notificationRound{{results: []setter.DNSResult{noop}}, {results: []setter.DNSResult{noop}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "grouped-updates", rounds: []notificationRound{{results: []setter.DNSResult{created, groupedSecond}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "distinct-previous-sets", rounds: []notificationRound{{results: []setter.DNSResult{created, notificationResult(setter.ResponseUpdated, fresh, api.RecordUpdate, api.MutationCompleted, "192.0.2.1")}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "multi-address-updates", targets: "192.0.2.1,192.0.2.2", rounds: []notificationRound{{results: []setter.DNSResult{multi}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "unknown-before", rounds: []notificationRound{{results: []setter.DNSResult{{Code: setter.ResponseFailed}}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "unconfirmed-historical-before", targets: "192.0.2.2", rounds: []notificationRound{{results: []setter.DNSResult{uncertain}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "completed-DNS-pending-WAF-cleanup", deferred: true, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{cleared}, wafCleanup: setter.ResponseUpdating, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "failed-cleanup", deferred: true, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{failed}, wafCleanup: setter.ResponseFailed, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindCleanupFailure}, wantExit: 1},
		{name: "shutdown-without-cleanup", deferred: true, rounds: []notificationRound{{}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "scheduling-failure-cleanup", deferred: true, rounds: []notificationRound{{}}, schedulingFailure: true, cleanup: true, cleanupResults: []setter.DNSResult{failed}, wafCleanup: setter.ResponseFailed, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindSchedulingFailure}, wantExit: 1},
		{name: "once-update", once: true, rounds: []notificationRound{{results: []setter.DNSResult{created}}}, wantKinds: []notifier.Kind{notifier.KindUpdate}},
		{name: "once-noop", once: true, rounds: []notificationRound{{results: []setter.DNSResult{noop}}}},
		{name: "direct-CleanupWait", once: true, cleanup: true, targets: "empty", cleanupResults: []setter.DNSResult{cleared}, wafCleanup: setter.ResponseUpdated, wantKinds: []notifier.Kind{notifier.KindCleanup}},
		{name: "static.empty", targets: "empty", rounds: []notificationRound{{results: []setter.DNSResult{{Code: setter.ResponseNoop, Before: notificationSnapshot()}}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "none", targets: "none", rounds: []notificationRound{{}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
	}...)
	//nolint:exhaustruct_v5 // Outcome fixtures vary only response and observed Before facts.
	for _, ending := range []struct {
		name    string
		result  setter.DNSResult
		kind    notifier.Kind
		targets string
	}{{"noop", setter.DNSResult{Code: setter.ResponseNoop, Before: fresh}, notifier.KindStateObservation, "192.0.2.2"}, {"success", notificationResult(setter.ResponseUpdated, fresh, api.RecordUpdate, api.MutationCompleted, "192.0.2.1"), notifier.KindUpdate, ""}, {"failure", setter.DNSResult{Code: setter.ResponseFailed, Before: fresh}, notifier.KindUpdateFailure, ""}} {
		//nolint:exhaustruct_v5 // Each sequence inherits ordinary scheduled defaults.
		scenarios = append(scenarios, notificationScenario{name: "unexplained-observation-then-" + ending.name, rounds: []notificationRound{{results: []setter.DNSResult{noop}}, {results: []setter.DNSResult{ending.result}, targets: ending.targets}}, wantKinds: []notifier.Kind{notifier.KindStartup, ending.kind, notifier.KindShutdown}})
	}
	return scenarios
}

// TestNotificationRenderingCapture emits the complete ordered review matrix with
// -v. Real updater calls and lifecycle routing produce the captured output; no
// exact notification prose is locked by this test.
func TestNotificationRenderingCapture(t *testing.T) {
	t.Parallel()
	captures := make([]notificationCapture, 0, 50)
	for _, scenario := range notificationScenarios() {
		for _, verbosity := range []pp.Verbosity{pp.Quiet, pp.Verbose} {
			capture := renderNotificationScenario(t, scenario, verbosity)
			captures = append(captures, capture)
			var kinds []notifier.Kind
			var channels []string
			for _, event := range capture.Events {
				if event.Channel != "cli" {
					channels = append(channels, event.Channel)
				}
				if event.Channel == "notifier" {
					kinds = append(kinds, event.Kind)
					require.NotEmpty(t, event.Text)
				}
			}
			require.Equal(t, expectedNotificationChannels(scenario), channels, scenario.name)
			require.Equal(t, scenario.wantKinds, kinds, scenario.name)
			require.Equal(t, scenario.wantExit, capture.Exit, scenario.name)
		}
	}
	data, err := json.MarshalIndent(captures, "", "  ")
	require.NoError(t, err)
	t.Logf("Ordered notification capture matrix:\n%s", data)
}

func renderNotificationScenario(t *testing.T, scenario notificationScenario, verbosity pp.Verbosity) notificationCapture {
	t.Helper()
	ctrl := gomock.NewController(t)
	schedule := &notificationSchedule{round: 0, zero: scenario.schedulingFailure, beforeWork: false}
	recorder := &notificationRecorder{events: nil}
	printer := pp.New(recorder, true, verbosity)
	workCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	families := scenario.families
	if len(families) == 0 {
		families = []ipnet.Family{ipnet.IP4}
	}
	//nolint:exhaustruct_v5 // Unused metadata options remain zero in evidence-only setter fixtures.
	cfg := &config.UpdateConfig{HostID6: make(map[domain.Domain]hostid6.Set), Provider: make(map[ipnet.Family]provider.Provider), Domains: make(map[ipnet.Family][]domain.Domain), DefaultPrefixLen: map[ipnet.Family]int{ipnet.IP4: 32, ipnet.IP6: 128}, DetectionFilter: map[ipnet.Family]ipfilter.Filter{ipnet.IP4: ipfilter.KeepAll(), ipnet.IP6: ipfilter.KeepAll()}, DetectionTimeout: time.Second, UpdateTimeout: time.Second}
	count := 1
	for _, round := range scenario.rounds {
		if len(round.results) > count && len(families) == 1 {
			count = len(round.results)
		}
	}
	for _, family := range families {
		raw := scenario.targets
		if raw == "" {
			raw = "192.0.2.1"
		}
		if family == ipnet.IP6 {
			raw = "2001:db8::1"
		}
		if raw != "none" {
			var p provider.Provider
			if raw == "empty" {
				p = provider.NewStaticEmpty()
			} else {
				p = provider.MustNewStatic(family, cfg.DefaultPrefixLen[family], raw)
			}
			cfg.Provider[family] = notificationProvider{schedule: schedule, rounds: scenario.rounds, family: family, fallback: p}
		}
		for i := range count {
			d := domain.FQDN(fmt.Sprintf("host%d.example", i))
			cfg.Domains[family] = append(cfg.Domains[family], d)
			cfg.HostID6[d] = hostid6.DefaultSet()
		}
	}
	s := mocks.NewMockSetter(ctrl)
	hb := mocks.NewMockHeartbeat(ctrl)
	nt := mocks.NewMockNotifier(ctrl)
	expectedCalls := make([][]notificationDNSCall, len(scenario.rounds))
	totalCalls := 0
	for round := range scenario.rounds {
		if scenario.once && scenario.cleanup || scenario.deferred && round == 0 {
			continue
		}
		for _, family := range families {
			if cfg.Provider[family] == nil || scenario.rounds[round].unavailable[family] {
				continue
			}
			for _, d := range cfg.Domains[family] {
				expectedCalls[round] = append(expectedCalls[round], notificationDNSCall{family: family, domain: d, targets: notificationTargets(scenario, round, family)})
			}
		}
		require.Len(t, scenario.rounds[round].results, len(expectedCalls[round]), "fixture must supply exactly one result per reachable scope in round %d", round)
		totalCalls += len(expectedCalls[round])
	}
	consumed := make([]int, len(scenario.rounds))
	s.EXPECT().SetIPs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ pp.PP, family ipnet.Family, d domain.Domain, targets []netip.Addr, _ api.RecordParams) setter.DNSResult {
		round := schedule.round - 1
		if scenario.once {
			round = 0
		} else {
			require.True(t, schedule.beforeWork, "cron.Next must precede work")
		}
		require.GreaterOrEqual(t, round, 0)
		require.Less(t, round, len(expectedCalls))
		index := consumed[round]
		require.Less(t, index, len(expectedCalls[round]))
		expected := expectedCalls[round][index]
		require.Equal(t, expected.family, family)
		require.Equal(t, expected.domain, d)
		require.True(t, slices.Equal(expected.targets, targets), "actual requested targets in round %d: expected %v, got %v", round, expected.targets, targets)
		result := scenario.rounds[round].results[index]
		require.Empty(t, notificationEvidenceProblem(result, targets), "invalid setter evidence for round %d", round)
		consumed[round]++
		if scenario.rounds[round].cancel {
			cancel()
			require.ErrorIs(t, ctx.Err(), context.Canceled)
		}
		return result
	}).Times(totalCalls)
	cleanupIndex := 0
	s.EXPECT().FinalDelete(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ pp.PP, family ipnet.Family, d domain.Domain, _ api.RecordParams) setter.DNSResult {
		require.Equal(t, families[cleanupIndex/count], family)
		require.Equal(t, cfg.Domains[family][cleanupIndex%count], d)
		require.NoError(t, ctx.Err())
		require.Less(t, cleanupIndex, len(scenario.cleanupResults))
		result := scenario.cleanupResults[cleanupIndex]
		require.Empty(t, notificationEvidenceProblem(result, nil))
		cleanupIndex++
		return result
	}).Times(len(scenario.cleanupResults))
	if scenario.cleanup {
		cfg.WAFLists = []api.WAFList{{AccountID: "account", Name: "list"}}
		wafRounds := len(scenario.rounds)
		if scenario.deferred {
			wafRounds--
		}
		if scenario.once && scenario.cleanup {
			wafRounds = 0
		}
		s.EXPECT().SetWAFList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(setter.ResponseNoop).Times(wafRounds)
		mode := api.CleanupAllowAsync
		if scenario.once {
			mode = api.CleanupWait
		}
		s.EXPECT().FinalClearWAFList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), mode).Return(scenario.wafCleanup)
	}
	heartbeatMessage := func(channel string) func(context.Context, pp.PP, heartbeat.Message) bool {
		return func(ctx context.Context, _ pp.PP, msg heartbeat.Message) bool {
			require.NoError(t, ctx.Err())
			recorder.add(channel, msg.Format(), "")
			return false
		}
	}
	hb.EXPECT().Ping(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(heartbeatMessage("heartbeat-ping")).AnyTimes()
	hb.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(heartbeatMessage("heartbeat-log")).AnyTimes()
	hb.EXPECT().Exit(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ pp.PP, msg string) bool {
		require.NoError(t, ctx.Err())
		recorder.add("heartbeat-exit", msg, "")
		return false
	}).AnyTimes()
	hb.EXPECT().Start(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ pp.PP, msg string) bool {
		require.NoError(t, ctx.Err())
		recorder.add("heartbeat-start", msg, "")
		return false
	})
	nt.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ pp.PP, msg notifier.Notification) bool {
		require.NoError(t, ctx.Err())
		recorder.add("notifier", msg.Format(), msg.Kind)
		return false
	}).AnyTimes()
	printer.Infof(pp.EmojiStar, "%s", formatName())
	hb.Start(t.Context(), printer, formatName())
	exit := 0
	if scenario.once {
		printer.BlankLineIfVerbose()
		var ok bool
		if scenario.cleanup {
			ok = runOnceCleanup(workCtx, t.Context(), printer, cfg, hb, nt, s)
		} else {
			ok = runOnceUpdate(workCtx, t.Context(), printer, cfg, hb, nt, s)
		}
		printer.Infof(pp.EmojiBye, "Bye!")
		if !ok {
			exit = 1
		}
	} else {
		exit = runScheduled(workCtx, t.Context(), printer, &config.LifecycleConfig{UpdateCron: schedule, UpdateOnStart: !scenario.deferred, DeleteOnStop: scenario.cleanup}, cfg, hb, nt, s, notificationWaiter{schedule: schedule, rounds: len(scenario.rounds)})
	}
	if scenario.cleanup {
		require.Equal(t, len(scenario.cleanupResults), cleanupIndex)
	}
	for round := range expectedCalls {
		require.Equal(t, len(expectedCalls[round]), consumed[round], "consumed results in round %d", round)
	}
	label := "normal"
	if verbosity == pp.Quiet {
		label = "quiet"
	}
	// These are adjacent output assertions, not complete transcript goldens.
	if verbosity == pp.Verbose {
		require.Contains(t, fmt.Sprint(recorder.events), "Bye!")
	}
	if scenario.schedulingFailure && workCtx.Err() == nil {
		require.Contains(t, fmt.Sprint(recorder.events), "No scheduled updates")
	}
	return notificationCapture{Scenario: scenario.name, Verbosity: label, Events: recorder.events, Exit: exit}
}

// notificationDNSCall describes the invocation promised by a scenario, separately
// from the provider execution which supplies actual updater inputs.
type notificationDNSCall struct {
	family  ipnet.Family
	domain  domain.Domain
	targets []netip.Addr
}

func notificationTargets(scenario notificationScenario, round int, family ipnet.Family) []netip.Addr {
	raw := scenario.targets
	if raw == "" {
		raw = "192.0.2.1"
	}
	if family == ipnet.IP6 {
		raw = "2001:db8::1"
	} else if scenario.rounds[round].targets != "" {
		raw = scenario.rounds[round].targets
	}
	if raw == "none" || raw == "empty" {
		return nil
	}
	targets := make([]netip.Addr, 0)
	for address := range strings.SplitSeq(raw, ",") {
		targets = append(targets, netip.MustParseAddr(address))
	}
	slices.SortFunc(targets, netip.Addr.Compare)
	return slices.Compact(targets)
}

// notificationEvidenceProblem rejects fixture contradictions before those facts
// can become review evidence. It checks only invariants relevant to these fixtures;
// setter implementation tests remain responsible for the full producer contract.
func notificationEvidenceProblem(result setter.DNSResult, targets []netip.Addr) string {
	before := make(map[api.ID]netip.Addr)
	observed := make([]netip.Addr, 0)
	if result.Before != nil {
		for _, record := range result.Before.Records {
			before[record.ID] = record.IP
			observed = append(observed, record.IP)
		}
	}
	slices.SortFunc(observed, netip.Addr.Compare)
	observed = slices.Compact(observed)
	if result.Code == setter.ResponseNoop && (result.Before == nil || !slices.Equal(observed, targets) || len(result.Effects) > 0) {
		return "no-op does not match requested targets"
	}
	created := make(map[api.ID]bool)
	for _, effect := range result.Effects {
		switch effect.Kind {
		case api.RecordCreate:
			if effect.Record.ID != "" {
				if _, exists := before[effect.Record.ID]; exists || created[effect.Record.ID] {
					return fmt.Sprintf("create reuses record identity %s", effect.Record.ID)
				}
				created[effect.Record.ID] = true
			}
		case api.RecordUpdate, api.RecordDelete:
			if _, exists := before[effect.Record.ID]; !exists {
				return fmt.Sprintf("mutation names absent record %s", effect.Record.ID)
			}
		}
		if effect.Kind != api.RecordDelete && !slices.Contains(targets, effect.Record.IP) {
			return "mutation address is absent from requested targets"
		}
	}
	return ""
}
