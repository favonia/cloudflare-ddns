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
	"github.com/favonia/cloudflare-ddns/internal/cron"
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
	waf         *setter.WAFResult
	results     []setter.DNSResult
	targets     string
	targets6    string
	unavailable map[ipnet.Family]bool
	cancel      bool
}
type notificationScenario struct {
	wholeWAF          bool
	waf               bool
	filtered          bool
	wafCleanupResult  *setter.WAFResult
	name              string
	rounds            []notificationRound
	targets           string
	targets6          string
	dnsFiltered       bool
	wafFiltered       bool
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
		return cron.Next(cron.MustNew(s.Describe()))
	}
	return time.Now().Add(time.Hour)
}

func (s *notificationSchedule) Describe() string {
	if s.zero {
		return "* * 30 2 *"
	}
	return "@every 1h"
}

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
	if round < len(p.rounds) {
		raw := p.rounds[round].targets
		if family == ipnet.IP6 {
			raw = p.rounds[round].targets6
		}
		if raw == "empty" {
			return provider.NewStaticEmpty().GetRawData(ctx, printer, family, prefix)
		}
		if raw != "" {
			return provider.MustNewStatic(family, prefix, raw).GetRawData(ctx, printer, family, prefix)
		}
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

// notificationProgressResult projects explicitly supplied progress facts into the
// existing API-effect shape. It does not decide which operations are required.
func notificationProgressResult(code setter.ResponseCode, before *setter.DNSSnapshot, satisfied []netip.Addr, operations ...setter.DNSOperation) setter.DNSResult {
	result := setter.DNSResult{Code: code, Before: before, Progress: nil, Effects: nil}
	if before == nil {
		return result
	}
	result.Progress = &setter.DNSProgress{Satisfied: satisfied, Operations: operations}
	for _, operation := range operations {
		var status api.MutationStatus
		switch operation.Status {
		case setter.DNSOperationCompleted:
			status = api.MutationCompleted
		case setter.DNSOperationUnconfirmed:
			status = api.MutationUnconfirmed
		case setter.DNSOperationNotAttempted, setter.DNSOperationBlocked:
			continue
		}
		record := operation.Record
		if operation.Kind == api.RecordDelete {
			record.IP = netip.Addr{}
		}
		result.Effects = append(result.Effects, api.RecordMutation{Kind: operation.Kind, Status: status, Record: record, Membership: operation.Membership})
	}
	return result
}

func notificationOperation(kind api.RecordMutationKind, status setter.DNSOperationStatus, id api.ID, address string, membership api.ManagedMembership) setter.DNSOperation {
	return setter.DNSOperation{Kind: kind, Record: api.RecordFact{ID: id, IP: netip.MustParseAddr(address)}, Status: status, Membership: membership}
}

func notificationNoop(before *setter.DNSSnapshot, satisfied ...string) setter.DNSResult {
	addresses := make([]netip.Addr, 0, len(satisfied))
	for _, address := range satisfied {
		addresses = append(addresses, netip.MustParseAddr(address))
	}
	return notificationProgressResult(setter.ResponseNoop, before, addresses)
}

func notificationResult(code setter.ResponseCode, before *setter.DNSSnapshot, kind api.RecordMutationKind, status api.MutationStatus, address string) setter.DNSResult {
	id := api.ID("created-" + address)
	membership := api.MembershipManaged
	progressStatus := setter.DNSOperationCompleted
	if status == api.MutationUnconfirmed {
		progressStatus = setter.DNSOperationUnconfirmed
		if kind == api.RecordCreate {
			id = ""
			membership = api.MembershipUnknown
		}
	}
	if kind != api.RecordCreate {
		id = before.Records[0].ID
	}
	if kind == api.RecordDelete {
		membership = api.MembershipUnknown
	}
	return notificationProgressResult(code, before, nil, notificationOperation(kind, progressStatus, id, address, membership))
}

func notificationScenarios() []notificationScenario {
	old := notificationSnapshot("192.0.2.1")
	fresh := notificationSnapshot("192.0.2.2")
	noop := notificationNoop(old, "192.0.2.1")
	created := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.1")
	cleared := notificationResult(setter.ResponseUpdated, old, api.RecordDelete, api.MutationCompleted, "192.0.2.1")
	failed := notificationProgressResult(setter.ResponseFailed, fresh, nil, notificationOperation(api.RecordUpdate, setter.DNSOperationBlocked, fresh.Records[0].ID, "192.0.2.1", api.MembershipUnknown))
	cleanupFailed := notificationProgressResult(setter.ResponseFailed, fresh, nil, notificationOperation(api.RecordDelete, setter.DNSOperationBlocked, fresh.Records[0].ID, "192.0.2.2", api.MembershipUnknown))
	historical := notificationSnapshot("192.0.2.1")
	historical.Cached = true
	uncertain := notificationResult(setter.ResponseFailed, historical, api.RecordUpdate, api.MutationUnconfirmed, "192.0.2.2")
	multi := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.1")
	second := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.2")
	multi.Effects = append(multi.Effects, second.Effects...)
	multi.Progress.Operations = append(multi.Progress.Operations, second.Progress.Operations...)
	groupedSecond := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.1")
	groupedSecond.Effects[0].Record.ID = "created-second-domain"
	groupedSecond.Progress.Operations[0].Record.ID = "created-second-domain"
	outside := notificationResult(setter.ResponseUpdated, old, api.RecordUpdate, api.MutationCompleted, "192.0.2.2")
	outside.Effects[0].Membership = api.MembershipUnmanaged
	outside.Progress.Operations[0].Membership = api.MembershipUnmanaged
	outsideCreate := notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.1")
	outsideCreate.Effects[0].Membership = api.MembershipUnmanaged
	outsideCreate.Progress.Operations[0].Membership = api.MembershipUnmanaged
	outsideMixed := notificationResult(setter.ResponseFailed, historical, api.RecordUpdate, api.MutationCompleted, "192.0.2.2")
	outsideMixed.Effects[0].Membership = api.MembershipUnmanaged
	outsideMixed.Progress.Operations[0].Membership = api.MembershipUnmanaged
	outsidePending := notificationResult(setter.ResponseFailed, historical, api.RecordCreate, api.MutationUnconfirmed, "192.0.2.3")
	outsideMixed.Effects = append(outsideMixed.Effects, outsidePending.Effects...)
	outsideMixed.Progress.Operations = append(outsideMixed.Progress.Operations, outsidePending.Progress.Operations...)
	partial := notificationResult(setter.ResponseFailed, old, api.RecordUpdate, api.MutationCompleted, "192.0.2.2")
	pendingCreate := notificationResult(setter.ResponseFailed, old, api.RecordCreate, api.MutationUnconfirmed, "192.0.2.3")
	partial.Effects = append(partial.Effects, pendingCreate.Effects...)
	partial.Progress.Operations = append(partial.Progress.Operations, pendingCreate.Progress.Operations...)
	uncertainDelete := notificationResult(setter.ResponseFailed, old, api.RecordDelete, api.MutationUnconfirmed, "192.0.2.1")
	scenarios := make([]notificationScenario, 0, 28)
	//nolint:exhaustruct_v5 // Matrix fixtures specify relevant variations; omitted options use their zero defaults.
	scenarios = append(scenarios, []notificationScenario{
		{name: "immediate-startup-noop", rounds: []notificationRound{{results: []setter.DNSResult{noop}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "immediate-startup-creation", rounds: []notificationRound{{results: []setter.DNSResult{created}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "immediate-startup-clearing", targets: "empty", rounds: []notificationRound{{results: []setter.DNSResult{cleared}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "immediate-startup-mixed-success-failure", rounds: []notificationRound{{results: []setter.DNSResult{created, failed}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "immediate-startup-detection-failure", rounds: []notificationRound{{unavailable: map[ipnet.Family]bool{ipnet.IP4: true}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "immediate-startup-interruption", rounds: []notificationRound{{results: []setter.DNSResult{failed}, cancel: true}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "deferred-startup-A-then-B", deferred: true, families: []ipnet.Family{ipnet.IP4, ipnet.IP6}, rounds: []notificationRound{{}, {results: []setter.DNSResult{noop}, unavailable: map[ipnet.Family]bool{ipnet.IP6: true}}, {results: []setter.DNSResult{noop, notificationNoop(notificationSnapshot("2001:db8::1"), "2001:db8::1")}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindUpdateFailure, notifier.KindStateObservation, notifier.KindShutdown}},
		{name: "ordinary-silence", rounds: []notificationRound{{results: []setter.DNSResult{noop}}, {results: []setter.DNSResult{noop}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "grouped-updates", rounds: []notificationRound{{results: []setter.DNSResult{created, groupedSecond}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "distinct-previous-sets", rounds: []notificationRound{{results: []setter.DNSResult{created, notificationResult(setter.ResponseUpdated, fresh, api.RecordUpdate, api.MutationCompleted, "192.0.2.1")}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "multi-address-updates", targets: "192.0.2.1,192.0.2.2", rounds: []notificationRound{{results: []setter.DNSResult{multi}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "unknown-before", rounds: []notificationRound{{results: []setter.DNSResult{{Code: setter.ResponseFailed, Before: nil, Progress: nil, Effects: nil}}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "completed-returned-outside-managed-scope", filtered: true, targets: "192.0.2.2", rounds: []notificationRound{{results: []setter.DNSResult{outside}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "completed-create-returned-outside-managed-scope", filtered: true, rounds: []notificationRound{{results: []setter.DNSResult{outsideCreate}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "partial-progress-comment-exclusion", filtered: true, targets: "192.0.2.2,192.0.2.3", rounds: []notificationRound{{results: []setter.DNSResult{outsideMixed}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "partial-progress-unconfirmed-create", targets: "192.0.2.2,192.0.2.3", rounds: []notificationRound{{results: []setter.DNSResult{partial}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "unconfirmed-cleanup-deletion", wholeWAF: true, families: []ipnet.Family{ipnet.IP4, ipnet.IP6}, deferred: true, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{uncertainDelete, notificationNoop(notificationSnapshot())}, wafCleanup: setter.ResponseUpdated, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindCleanupFailure}, wantExit: 1},
		{name: "unconfirmed-historical-before", targets: "192.0.2.2", rounds: []notificationRound{{results: []setter.DNSResult{uncertain}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "completed-DNS-pending-WAF-cleanup", deferred: true, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{cleared}, wafCleanup: setter.ResponseUpdating, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "failed-cleanup", deferred: true, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{cleanupFailed}, wafCleanup: setter.ResponseFailed, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindCleanupFailure}, wantExit: 1},
		{name: "shutdown-without-cleanup", deferred: true, rounds: []notificationRound{{}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "scheduling-failure-cleanup", deferred: true, rounds: []notificationRound{{}}, schedulingFailure: true, cleanup: true, cleanupResults: []setter.DNSResult{cleanupFailed}, wafCleanup: setter.ResponseFailed, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindSchedulingFailure}, wantExit: 1},
		{name: "once-update", once: true, rounds: []notificationRound{{results: []setter.DNSResult{created}}}, wantKinds: []notifier.Kind{notifier.KindUpdate}},
		{name: "once-noop", once: true, rounds: []notificationRound{{results: []setter.DNSResult{noop}}}},
		{name: "direct-CleanupWait", once: true, cleanup: true, targets: "empty", cleanupResults: []setter.DNSResult{cleared}, wafCleanup: setter.ResponseUpdated, wantKinds: []notifier.Kind{notifier.KindCleanup}},
		{name: "static.empty", targets: "empty", rounds: []notificationRound{{results: []setter.DNSResult{notificationNoop(notificationSnapshot())}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
		{name: "none", targets: "none", rounds: []notificationRound{{}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}},
	}...)
	//nolint:exhaustruct_v5 // Outcome fixtures vary only response and observed Before facts.
	for _, ending := range []struct {
		name    string
		result  setter.DNSResult
		kind    notifier.Kind
		targets string
	}{{"noop", notificationNoop(fresh, "192.0.2.2"), notifier.KindStateObservation, "192.0.2.2"}, {"success", notificationResult(setter.ResponseUpdated, fresh, api.RecordUpdate, api.MutationCompleted, "192.0.2.1"), notifier.KindUpdate, ""}, {"failure", failed, notifier.KindUpdateFailure, ""}} {
		//nolint:exhaustruct_v5 // Each sequence inherits ordinary scheduled defaults.
		scenarios = append(scenarios, notificationScenario{name: "unexplained-observation-then-" + ending.name, rounds: []notificationRound{{results: []setter.DNSResult{noop}}, {results: []setter.DNSResult{ending.result}, targets: ending.targets}}, wantKinds: []notifier.Kind{notifier.KindStartup, ending.kind, notifier.KindShutdown}})
	}
	return append(append(append(scenarios, notificationProgressScenarios()...), notificationCurrentScenarios()...), notificationFilteredScenarios()...)
}

// The operation lists below are hand-specified setter evidence for the canonical
// review cases. Each deletion preserves its own record identity and old address.
func notificationProgressScenarios() []notificationScenario {
	old := notificationSnapshot("192.0.2.1")
	completed := notificationOperation(api.RecordUpdate, setter.DNSOperationCompleted, old.Records[0].ID, "192.0.2.2", api.MembershipManaged)
	uncertain := notificationOperation(api.RecordCreate, setter.DNSOperationUnconfirmed, "", "192.0.2.3", api.MembershipUnknown)
	unvisited := notificationOperation(api.RecordCreate, setter.DNSOperationNotAttempted, "", "192.0.2.4", api.MembershipUnknown)
	blocked := notificationOperation(api.RecordCreate, setter.DNSOperationBlocked, "", "192.0.2.3", api.MembershipUnknown)
	already := notificationSnapshot("192.0.2.1", "192.0.2.4")
	leftovers := notificationSnapshot("192.0.2.1", "192.0.2.5", "192.0.2.6")
	deleteUncertain := notificationOperation(api.RecordDelete, setter.DNSOperationUnconfirmed, leftovers.Records[1].ID, "192.0.2.5", api.MembershipUnknown)
	deleteUnvisited := notificationOperation(api.RecordDelete, setter.DNSOperationNotAttempted, leftovers.Records[2].ID, "192.0.2.6", api.MembershipUnknown)
	finalBefore := notificationSnapshot("192.0.2.1", "192.0.2.5")
	finalResult := notificationProgressResult(setter.ResponseFailed, finalBefore, nil,
		notificationOperation(api.RecordDelete, setter.DNSOperationUnconfirmed, finalBefore.Records[0].ID, "192.0.2.1", api.MembershipUnknown),
		notificationOperation(api.RecordDelete, setter.DNSOperationCompleted, finalBefore.Records[1].ID, "192.0.2.5", api.MembershipUnknown))
	duplicates := &setter.DNSSnapshot{Cached: false, Records: []api.RecordFact{
		{ID: "duplicate-1", IP: netip.MustParseAddr("192.0.2.1")},
		{ID: "duplicate-2", IP: netip.MustParseAddr("192.0.2.1")},
		{ID: "duplicate-3", IP: netip.MustParseAddr("192.0.2.1")},
	}}
	duplicateResult := notificationProgressResult(setter.ResponseFailed, duplicates, nil,
		notificationOperation(api.RecordDelete, setter.DNSOperationCompleted, "duplicate-1", "192.0.2.1", api.MembershipUnknown),
		notificationOperation(api.RecordDelete, setter.DNSOperationUnconfirmed, "duplicate-2", "192.0.2.1", api.MembershipUnknown),
		notificationOperation(api.RecordDelete, setter.DNSOperationNotAttempted, "duplicate-3", "192.0.2.1", api.MembershipUnknown))
	cached := notificationSnapshot("192.0.2.1")
	cached.Cached = true
	cachedEmpty := notificationSnapshot()
	cachedEmpty.Cached = true
	//nolint:exhaustruct_v5 // Matrix options default to the ordinary scheduled lifecycle.
	return []notificationScenario{
		{name: "multi-ip-stopped-suffix", targets: "192.0.2.2,192.0.2.3,192.0.2.4", rounds: []notificationRound{{results: []setter.DNSResult{notificationProgressResult(setter.ResponseFailed, old, nil, completed, uncertain, unvisited)}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "multi-ip-prerequisite", targets: "192.0.2.2,192.0.2.3,192.0.2.4", rounds: []notificationRound{{results: []setter.DNSResult{notificationProgressResult(setter.ResponseFailed, old, nil, completed, blocked, unvisited)}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "multi-ip-already-satisfied", targets: "192.0.2.2,192.0.2.3,192.0.2.4", rounds: []notificationRound{{results: []setter.DNSResult{notificationProgressResult(setter.ResponseFailed, already, []netip.Addr{netip.MustParseAddr("192.0.2.4")}, completed, uncertain)}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "multi-ip-delete-stage", targets: "192.0.2.2", rounds: []notificationRound{{results: []setter.DNSResult{notificationProgressResult(setter.ResponseFailed, leftovers, nil, completed, deleteUncertain, deleteUnvisited)}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "multi-ip-final-delete", deferred: true, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{finalResult}, wafCleanup: setter.ResponseNoop, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindCleanupFailure}, wantExit: 1},
		{name: "cached-noop-context", deferred: true, rounds: []notificationRound{{}, {results: []setter.DNSResult{notificationNoop(cached, "192.0.2.1")}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindStateObservation, notifier.KindShutdown}},
		{name: "duplicate-address-deletion-progress", targets: "empty", rounds: []notificationRound{{results: []setter.DNSResult{duplicateResult}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}},
		{name: "cached-empty-noop-context", targets: "empty", deferred: true, rounds: []notificationRound{{}, {results: []setter.DNSResult{notificationNoop(cachedEmpty)}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindStateObservation, notifier.KindShutdown}},
	}
}

// TestNotificationRenderingCapture emits the complete ordered review matrix with
// -v. Real updater calls and lifecycle routing produce the captured output; no
// exact notification prose is locked by this test.
func TestNotificationRenderingCapture(t *testing.T) {
	t.Parallel()
	captures := make([]notificationCapture, 0, len(notificationScenarios())*2)
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
	cfg.ManagedWAFListItemsFiltered = scenario.filtered || scenario.wafFiltered
	cfg.ManagedRecordsFiltered = scenario.filtered || scenario.dnsFiltered
	count := 1
	for _, round := range scenario.rounds {
		if len(round.results) > count && len(families) == 1 {
			count = len(round.results)
		}
	}
	for _, family := range families {
		raw := notificationRawTargets(scenario, family)
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
		require.Empty(t, notificationDNSOwnershipProblem(result, cfg.ManagedRecordsFiltered))
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
		require.Empty(t, notificationDNSOwnershipProblem(result, cfg.ManagedRecordsFiltered))
		require.Empty(t, notificationEvidenceProblem(result, nil))
		cleanupIndex++
		return result
	}).Times(len(scenario.cleanupResults))
	if scenario.cleanup || scenario.waf {
		cfg.WAFLists = []api.WAFList{{AccountID: "account", Name: "list"}}
		for round := range scenario.rounds {
			if scenario.once && scenario.cleanup || scenario.deferred && round == 0 {
				continue
			}
			available := false
			for _, family := range families {
				available = available || cfg.Provider[family] != nil && !scenario.rounds[round].unavailable[family]
			}
			if !available {
				continue
			}
			result := scenario.rounds[round].waf
			if result == nil {
				result = &setter.WAFResult{Before: &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}, Batches: nil, Cleanup: nil, Code: setter.ResponseNoop}
			}
			expected := make(map[ipnet.Family]setter.WAFTargets)
			for _, family := range families {
				if cfg.Provider[family] == nil {
					continue
				}
				if scenario.rounds[round].unavailable[family] {
					expected[family] = setter.NewUnavailableWAFTargets()
					continue
				}
				prefixes := make([]netip.Prefix, 0)
				for _, address := range notificationTargets(scenario, round, family) {
					prefixes = append(prefixes, netip.PrefixFrom(address, address.BitLen()))
				}
				expected[family] = setter.NewAvailableWAFTargets(prefixes)
			}
			require.Empty(t, notificationWAFEvidenceProblem(*result, expected), scenario.name)
			s.EXPECT().SetWAFList(gomock.Any(), gomock.Any(), cfg.WAFLists[0], gomock.Any(), expected, gomock.Any()).Return(*result)
		}
		if scenario.cleanup {
			mode := api.CleanupAllowAsync
			if scenario.once {
				mode = api.CleanupWait
			}
			result := scenario.wafCleanupResult
			if result == nil {
				value := notificationWAFCleanup(scenario.wafCleanup, scenario.wholeWAF)
				result = &value
			}
			managedFamilies := make(map[ipnet.Family]bool)
			for family, p := range cfg.Provider {
				if p != nil {
					managedFamilies[family] = true
				}
			}
			require.Empty(t, notificationCleanupProblem(*result, managedFamilies, scenario.wholeWAF && !cfg.ManagedWAFListItemsFiltered, mode), scenario.name)
			s.EXPECT().FinalClearWAFList(gomock.Any(), gomock.Any(), cfg.WAFLists[0], gomock.Any(), managedFamilies, mode).Return(*result)
		}
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
		require.Len(t, msg.Message, 1, "one composed message element per Send envelope")
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

// notificationRawTargets supplies one family-specific source for provider setup
// and expected inputs. "empty" is an explicit empty provider, unlike "none".
func notificationRawTargets(scenario notificationScenario, family ipnet.Family) string {
	if family == ipnet.IP6 {
		if scenario.targets6 != "" {
			return scenario.targets6
		}
		return "2001:db8::1"
	}
	if scenario.targets != "" {
		return scenario.targets
	}
	return "192.0.2.1"
}

func notificationTargets(scenario notificationScenario, round int, family ipnet.Family) []netip.Addr {
	raw := notificationRawTargets(scenario, family)
	if family == ipnet.IP6 {
		if scenario.rounds[round].targets6 != "" {
			raw = scenario.rounds[round].targets6
		}
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
	if (result.Before == nil) != (result.Progress == nil) {
		return "snapshot and progress availability disagree"
	}
	if result.Before == nil {
		if result.Code != setter.ResponseFailed || len(result.Effects) != 0 {
			return "unknown initial read has outcome evidence"
		}
		return ""
	}
	before := make(map[api.ID]netip.Addr)
	observed := make([]netip.Addr, 0)
	for _, record := range result.Before.Records {
		if record.ID == "" || !record.IP.IsValid() {
			return "invalid snapshot record"
		}
		if _, exists := before[record.ID]; exists {
			return "duplicate snapshot record identity"
		}
		before[record.ID] = record.IP
		observed = append(observed, record.IP)
	}
	slices.SortFunc(observed, netip.Addr.Compare)
	observed = slices.Compact(observed)
	if result.Code == setter.ResponseNoop && (!slices.Equal(observed, targets) || len(result.Effects) > 0 || len(result.Progress.Operations) > 0) {
		return "no-op does not match requested targets"
	}
	covered := make(map[netip.Addr]bool)
	lastSatisfied := -1
	for _, address := range result.Progress.Satisfied {
		index := slices.Index(targets, address)
		if index <= lastSatisfied || !slices.Contains(observed, address) || covered[address] {
			return "satisfied target is not ordered observed target evidence"
		}
		covered[address] = true
		lastSatisfied = index
	}
	used := make(map[api.ID]bool)
	effectIndex := 0
	for _, operation := range result.Progress.Operations {
		if problem := notificationOperationProblem(operation, before, used, targets); problem != "" {
			return problem
		}
		if operation.Kind != api.RecordDelete {
			if covered[operation.Record.IP] {
				return "target has conflicting satisfied/write classifications"
			}
			covered[operation.Record.IP] = true
		}
		if result.Code != setter.ResponseFailed && operation.Status != setter.DNSOperationCompleted {
			return "successful result includes unsuccessful work"
		}
		if operation.Status == setter.DNSOperationBlocked || operation.Status == setter.DNSOperationNotAttempted {
			continue
		}
		if effectIndex >= len(result.Effects) {
			return "reached operation lacks mutation evidence"
		}
		effect := result.Effects[effectIndex]
		effectIndex++
		status := api.MutationCompleted
		if operation.Status == setter.DNSOperationUnconfirmed {
			status = api.MutationUnconfirmed
		}
		record := operation.Record
		if operation.Kind == api.RecordDelete {
			record.IP = netip.Addr{}
		}
		if effect.Kind != operation.Kind || effect.Status != status || effect.Record != record || effect.Membership != operation.Membership {
			return "progress and mutation evidence disagree"
		}
	}
	if effectIndex != len(result.Effects) {
		return "mutation has no reached progress operation"
	}
	for _, target := range targets {
		if !covered[target] {
			return "target has no progress classification"
		}
	}
	return ""
}

func notificationOperationProblem(operation setter.DNSOperation, before map[api.ID]netip.Addr, used map[api.ID]bool, targets []netip.Addr) string {
	if !operation.Record.IP.IsValid() {
		return "operation address is invalid"
	}
	switch operation.Status {
	case setter.DNSOperationNotAttempted, setter.DNSOperationBlocked:
		if operation.Membership != api.MembershipUnknown {
			return "unwritten operation invents membership"
		}
	case setter.DNSOperationUnconfirmed, setter.DNSOperationCompleted:
	default:
		return "invalid operation status"
	}
	switch operation.Kind {
	case api.RecordCreate:
		if operation.Status != setter.DNSOperationCompleted && operation.Record.ID != "" {
			return "unfinished create invents record identity"
		}
		if operation.Record.ID != "" {
			if _, exists := before[operation.Record.ID]; exists {
				return "create reuses snapshot identity"
			}
		}
	case api.RecordUpdate, api.RecordDelete:
		address, exists := before[operation.Record.ID]
		if !exists {
			return "operation names absent record"
		}
		if operation.Kind == api.RecordDelete && operation.Record.IP != address {
			return "deletion address does not belong to record"
		}
	default:
		return "invalid operation kind"
	}
	if operation.Record.ID != "" {
		if used[operation.Record.ID] {
			return "record identity used by multiple operations"
		}
		used[operation.Record.ID] = true
	}
	if operation.Kind != api.RecordDelete && !slices.Contains(targets, operation.Record.IP) {
		return "operation address absent from targets"
	}
	return ""
}
