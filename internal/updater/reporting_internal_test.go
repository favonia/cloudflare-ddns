package updater

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// reportingOutcome supplies consistent default progress and reached effects.
// Tests that replace individual fields to isolate rendering qualifications are
// projections, not complete setter traces; stateful tests rebuild whole rounds.
func reportingOutcome(name string, code setter.ResponseCode, before *setter.DNSSnapshot) dnsOutcome {
	var progress *setter.DNSProgress
	if before != nil {
		progress = &setter.DNSProgress{Satisfied: nil, Operations: nil}
		if code == setter.ResponseNoop {
			for _, record := range before.Records {
				progress.Satisfied = append(progress.Satisfied, record.IP)
			}
		} else {
			status := setter.DNSOperationCompleted
			if code == setter.ResponseFailed {
				status = setter.DNSOperationBlocked
			}
			if len(before.Records) == 0 {
				progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordCreate, status, "created", "192.0.2.2")}
			} else {
				progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordUpdate, status, string(before.Records[0].ID), "192.0.2.2")}
				for _, record := range before.Records[1:] {
					progress.Operations = append(progress.Operations, reportingOperation(api.RecordDelete, status, string(record.ID), record.IP.String()))
				}
			}
			for i := range progress.Operations {
				op := &progress.Operations[i]
				if status == setter.DNSOperationCompleted && op.Kind != api.RecordDelete {
					op.Membership = api.MembershipManaged
				}
				if status == setter.DNSOperationBlocked && i > 0 {
					op.Status = setter.DNSOperationNotAttempted
				}
				if op.Kind == api.RecordCreate && status != setter.DNSOperationCompleted {
					op.Record.ID = ""
				}
			}
		}
	}
	o := dnsOutcome{
		filtered: false,
		scope:    dnsScope{domain: domain.FQDN(name), family: ipnet.IP4},
		cleanup:  false,
		target:   []netip.Addr{netip.MustParseAddr("192.0.2.2")},
		result:   setter.DNSResult{Progress: progress, Code: code, Before: before, Effects: nil},
	}
	if code == setter.ResponseNoop && progress != nil {
		o.target = canonicalAddresses(progress.Satisfied)
		progress.Satisfied = canonicalAddresses(progress.Satisfied)
	}
	reportingSyncEffects(&o)
	return o
}

// reportingSyncEffects rebuilds the API projection after a test changes progress.
// It does not infer progress from effects or change the facts under test.
func reportingSyncEffects(o *dnsOutcome) {
	o.result.Effects = nil
	if o.result.Progress == nil {
		return
	}
	for _, op := range o.result.Progress.Operations {
		var status api.MutationStatus
		switch op.Status {
		case setter.DNSOperationCompleted:
			status = api.MutationCompleted
		case setter.DNSOperationUnconfirmed:
			status = api.MutationUnconfirmed
		default:
			continue
		}
		fact := op.Record
		if op.Kind == api.RecordDelete {
			fact.IP = netip.Addr{}
		}
		o.result.Effects = append(o.result.Effects, api.RecordMutation{
			Status: status, Kind: op.Kind, Record: fact, Membership: op.Membership,
		})
	}
}

func reportingSnapshot(address string) *setter.DNSSnapshot {
	before := &setter.DNSSnapshot{Cached: false, Records: nil}
	if address != "" {
		before.Records = []api.RecordFact{{ID: "record", IP: netip.MustParseAddr(address)}}
	}
	return before
}

func reportingMessage(outcomes ...dnsOutcome) Message {
	m := newMessage()
	m.dns = outcomes
	return m
}

func TestReportingDeferredCoverage(t *testing.T) {
	t.Parallel()
	s := NewReportingState(true)
	a := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	b := reportingOutcome("b.example", setter.ResponseFailed, nil)
	first := Compose(s.Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Equal(t, notifier.KindUpdateFailure, first.Notification().Kind)
	s.Submitted(first)
	b = reportingOutcome("b.example", setter.ResponseNoop, reportingSnapshot(""))
	second := Compose(s.Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Equal(t, notifier.KindStateObservation, second.Notification().Kind)
	require.NotContains(t, second.Notification().Format(), "a.example")
	require.Contains(t, second.Notification().Format(), "b.example")
	// Composing and even reading the notification must not consume coverage.
	require.False(t, Compose(s.Assess(reportingMessage(b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
	s.Submitted(second)
	require.True(t, Compose(s.Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
}

func TestReportingCoverageUsesHistoricalBeforeNotTarget(t *testing.T) {
	t.Parallel()
	for _, known := range []bool{false, true} {
		t.Run(map[bool]string{false: "target only", true: "historical before"}[known], func(t *testing.T) {
			t.Parallel()
			s := NewReportingState(true)
			o := reportingOutcome("a.example", setter.ResponseFailed, nil)
			if known {
				o = reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot(""))
			}
			s.Submitted(Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
			o = reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot(""))
			require.Equal(t, known, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
		})
	}
}

func TestReportingOneShotNoCrossRoundTracking(t *testing.T) {
	t.Parallel()
	s := NewReportingState(false)
	o := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	require.True(t, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
	o = reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.2"))
	require.True(t, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
	o = reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	require.Equal(t, notifier.KindUpdate, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Kind)
}

func TestReportingObservationAdvancesOnAssess(t *testing.T) {
	t.Parallel()
	s := NewReportingState(true)
	o := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	s.Submitted(Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
	o = reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.2"))
	a := s.Assess(reportingMessage(o))
	first := Compose(a, LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Equal(t, notifier.KindStateObservation, first.Notification().Kind)
	require.Equal(t, first.Notification(), Compose(a, LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification())
	// No submission is needed to resolve the inference discrepancy.
	require.True(t, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
}

func TestReportingReachedEffectsExplainNextFreshRead(t *testing.T) {
	t.Parallel()
	state := NewReportingState(true)
	context := LifecycleContext{Kind: LifecycleNone, Schedule: ""}
	updated := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	state.Submitted(Compose(state.Assess(reportingMessage(updated)), context))
	after := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.2"))
	require.True(t, Compose(state.Assess(reportingMessage(after)), context).IsEmpty())
	unexpected := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.3"))
	require.False(t, Compose(state.Assess(reportingMessage(unexpected)), context).IsEmpty())
}

// Each row below isolates one grouping qualification, rather than claiming the
// modified outcome is a complete producer trace.
func TestReportingGroupingUsesAllRenderedFacts(t *testing.T) {
	t.Parallel()
	for name, modify := range map[string]func(*dnsOutcome){
		"unknown before":      func(o *dnsOutcome) { o.result.Before = nil },
		"different before":    func(o *dnsOutcome) { o.result.Before = reportingSnapshot("192.0.2.1") },
		"different target":    func(o *dnsOutcome) { o.target = nil },
		"different ownership": func(o *dnsOutcome) { o.result.Progress.Operations[0].Membership = api.MembershipUnmanaged },
		"different completion": func(o *dnsOutcome) {
			o.result.Code = setter.ResponseFailed
			o.result.Progress.Operations[0].Status = setter.DNSOperationUnconfirmed
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot(""))
			a.result.Effects = []api.RecordMutation{{Status: api.MutationCompleted, Kind: api.RecordCreate, Record: api.RecordFact{ID: "r", IP: a.target[0]}, Membership: api.MembershipManaged}}
			b := a.clone()
			b.scope.domain = domain.FQDN("b.example")
			modify(&b)
			e := Compose(NewReportingState(false).Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
			require.Len(t, strings.Split(e.Notification().Format(), "\n\n"), 2)
		})
	}
}

func TestReportingGroupingPreservesCacheProvenance(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	b := a.clone()
	b.scope.domain = domain.FQDN("b.example")
	b.result.Before.Cached = true
	b.result.Before.Records[0].ID = "another"
	b.result.Before.Records = append(b.result.Before.Records, b.result.Before.Records[0])
	e := Compose(NewReportingState(false).Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, strings.Split(e.Notification().Format(), "\n\n"), 2)
	require.Contains(t, e.Notification().Format(), "a.example")
	require.Contains(t, e.Notification().Format(), "b.example")
}

func TestReportingGroupingCanonicalizesMappedIPv4WithoutMutatingAssessment(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("::ffff:192.0.2.10"))
	a.result.Before.Records = append(a.result.Before.Records,
		api.RecordFact{ID: "ordinary", IP: netip.MustParseAddr("192.0.2.10")})
	a.result.Progress.Operations = append(a.result.Progress.Operations, reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "ordinary", "192.0.2.10"))
	b := reportingOutcome("b.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.10"))
	assessment := NewReportingState(false).Assess(reportingMessage(a, b))
	envelope := Compose(assessment, LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, strings.Split(envelope.Notification().Format(), "\n\n"), 1)
	require.Contains(t, envelope.Notification().Format(), "192.0.2.10")
	require.NotContains(t, envelope.Notification().Format(), "::ffff:")
	// Assessment owns the admitted snapshot; pure composition must canonicalize a
	// separate rendering copy rather than rewrite those operation facts.
	require.Equal(t, a.result.Before, assessment.dns[0].outcome.result.Before)
	require.True(t, assessment.dns[0].outcome.result.Before.Records[0].IP.Is4In6())
	require.Len(t, assessment.dns[0].outcome.result.Before.Records, 2)
}

func TestReportingLifecyclePriorityAndConstituentFailures(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		lifecycle LifecycleKind
		cleanup   bool
		want      notifier.Kind
	}{
		"startup failure":    {LifecycleStartup, false, notifier.KindUpdateFailure},
		"shutdown failure":   {LifecycleShutdown, true, notifier.KindCleanupFailure},
		"scheduling failure": {LifecycleSchedulingFailure, true, notifier.KindSchedulingFailure},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := reportingMessage(reportingOutcome("failed.example", setter.ResponseFailed, nil))
			m.cleanup = tc.cleanup
			m.dns[0].cleanup = tc.cleanup
			m.waf = []wafOutcome{{result: setter.WAFResult{Code: setter.ResponseFailed, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, scope: nil, list: api.WAFList{AccountID: "account", Name: "failed-list"}, cleanup: tc.cleanup}}
			m.problems = []operationProblem{{kind: problemDetection, family: ipnet.IP4, domains: nil}, {kind: problemFiltering, family: ipnet.IP6, domains: nil}, {kind: problemDerivation, family: ipnet.IP6, domains: []domain.Domain{domain.FQDN("derive.example")}}, {kind: problemMissingTarget, family: ipnet.IP4, domains: []domain.Domain{domain.FQDN("missing.example")}}}
			e := Compose(NewReportingState(true).Assess(m), LifecycleContext{Kind: tc.lifecycle, Schedule: "schedule"})
			require.Equal(t, tc.want, e.Notification().Kind)
			require.Len(t, strings.Split(e.Notification().Format(), "\n\n"), 7)
			if tc.lifecycle != LifecycleShutdown {
				require.Contains(t, e.Notification().Format(), "schedule")
			}
			for _, value := range []string{"failed.example", "failed-list", "derive.example", "missing.example"} {
				require.Contains(t, e.Notification().Format(), value)
			}
		})
	}
}

func TestReportingStartupIncludesNoopScopes(t *testing.T) {
	t.Parallel()
	s := NewReportingState(true)
	o := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot(""))
	s.Submitted(Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
	e := Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleStartup, Schedule: "schedule"})
	require.Equal(t, notifier.KindStartup, e.Notification().Kind)
	require.Contains(t, e.Notification().Format(), "a.example")
	require.Equal(t, notifier.KindShutdown, Compose(Assessment{dns: nil, waf: nil, problems: nil, cleanup: false}, LifecycleContext{Kind: LifecycleShutdown, Schedule: ""}).Notification().Kind)
}

func TestReportingOwnsSnapshots(t *testing.T) {
	t.Parallel()
	m := reportingMessage(reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1")))
	a := NewReportingState(true).Assess(m)
	e := Compose(a, LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	want := e.Notification().Format()
	m.dns[0].target[0] = netip.MustParseAddr("192.0.2.99")
	m.dns[0].result.Before.Records[0].IP = netip.MustParseAddr("192.0.2.98")
	require.Equal(t, want, Compose(a, LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format())
	n := e.Notification()
	require.NotEmpty(t, n.Message)
	n.Message[0] = "mutated"
	require.Equal(t, want, e.Notification().Format())
}

func TestReportingCompletionFactsRemainSupportedAndReadable(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("partial.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
	o.result.Effects = []api.RecordMutation{
		{Status: api.MutationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "private-id", IP: netip.MustParseAddr("192.0.2.2")}, Membership: api.MembershipUnmanaged},
		{Status: api.MutationUnconfirmed, Kind: api.RecordCreate, Record: api.RecordFact{ID: "", IP: netip.MustParseAddr("192.0.2.3")}, Membership: api.MembershipUnknown},
	}
	o.result.Progress = &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{
		{Status: setter.DNSOperationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "private-id", IP: netip.MustParseAddr("192.0.2.2")}, Membership: api.MembershipUnmanaged},
		reportingOperation(api.RecordCreate, setter.DNSOperationUnconfirmed, "", "192.0.2.3"),
	}}
	e := Compose(NewReportingState(false).Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	text := e.Notification().Format()
	for _, fact := range []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"} {
		require.Contains(t, text, fact)
	}
	require.NotContains(t, text, "private-id")
	require.Contains(t, text, "Record for 192.0.2.2 excluded from management: its unexpected new comment does not match the filter.")
	require.NotContains(t, text, "membership")
	require.NotContains(t, text, "inconsistent")
}

func TestReportingGroupingIgnoresEquivalentMechanicsAndCoverageReason(t *testing.T) {
	t.Parallel()
	s := NewReportingState(true)
	a := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	s.Submitted(Compose(s.Assess(reportingMessage(a)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
	a = reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	a.result.Progress.Operations = []setter.DNSOperation{{Status: setter.DNSOperationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "record", IP: a.target[0]}, Membership: api.MembershipManaged}}
	b := a.clone()
	b.scope.domain = domain.FQDN("b.example")
	b.result.Progress.Operations[0].Kind = api.RecordCreate
	b.result.Progress.Operations[0].Record.ID = "different-id"
	b.result.Progress.Operations = append(b.result.Progress.Operations, reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "record", "192.0.2.1"))
	reportingSyncEffects(&a)
	reportingSyncEffects(&b)
	e := Compose(s.Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, strings.Split(e.Notification().Format(), "\n\n"), 1)
	s.Submitted(e)
	b = reportingOutcome("b.example", setter.ResponseNoop, reportingSnapshot("192.0.2.2"))
	b.result.Before.Records[0].ID = "different-id"
	b.result.Before.Cached = true
	require.True(t, Compose(s.Assess(reportingMessage(b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
}

func TestReportingGroupingCanonicalizesMappedCompletionAddresses(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
	a.result.Progress.Operations = []setter.DNSOperation{
		{Status: setter.DNSOperationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "record", IP: netip.MustParseAddr("::ffff:192.0.2.2")}, Membership: api.MembershipManaged},
		{Status: setter.DNSOperationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "record", IP: netip.MustParseAddr("192.0.2.2")}, Membership: api.MembershipManaged},
	}
	b := a.clone()
	b.scope.domain = domain.FQDN("b.example")
	b.result.Progress.Operations = b.result.Progress.Operations[1:]
	assessment := NewReportingState(false).Assess(reportingMessage(a, b))
	envelope := Compose(assessment, LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, strings.Split(envelope.Notification().Format(), "\n\n"), 1)
	require.NotContains(t, envelope.Notification().Format(), "::ffff:")
	require.Equal(t, 1, strings.Count(envelope.Notification().Format(), "192.0.2.2"))
	require.Equal(t, a.result.Progress, assessment.dns[0].outcome.result.Progress)
}

func TestReportingGroupingRetainsObservationContext(t *testing.T) {
	t.Parallel()
	s := NewReportingState(true)
	a := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	s.Submitted(Compose(s.Assess(reportingMessage(a)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
	a = reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.2"))
	b := a.clone()
	b.scope.domain = domain.FQDN("b.example")
	e := Compose(s.Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, strings.Split(e.Notification().Format(), "\n\n"), 2)
	lines := strings.Split(e.Notification().Format(), "\n\n")
	require.NotEqual(t, lines[0], lines[1])
	require.Contains(t, lines[0], "Detected an unexpected address change")
	require.Less(t, strings.Index(lines[0], "Detected an unexpected address change"), strings.Index(lines[0], "No IPv4 update needed"))
	require.NotContains(t, lines[1], "unexpected")
}

func TestReportingWAFPendingCleanupAndLifecycleOnly(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		code      setter.ResponseCode
		lifecycle LifecycleKind
		want      notifier.Kind
		empty     bool
	}{
		"pending cleanup":  {setter.ResponseUpdating, LifecycleNone, notifier.KindCleanup, false},
		"pending shutdown": {setter.ResponseUpdating, LifecycleShutdown, notifier.KindShutdown, false},
		"noop cleanup":     {setter.ResponseNoop, LifecycleNone, notifier.KindStateObservation, true},
		"startup noop":     {setter.ResponseNoop, LifecycleStartup, notifier.KindStartup, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := newMessage()
			m.cleanup = true
			m.waf = []wafOutcome{{result: setter.WAFResult{Code: tc.code, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, scope: nil, list: api.WAFList{AccountID: "account", Name: "list"}, cleanup: true}}
			e := Compose(NewReportingState(false).Assess(m), LifecycleContext{Kind: tc.lifecycle, Schedule: ""})
			require.Equal(t, tc.empty, e.IsEmpty())
			require.Equal(t, tc.want, e.Notification().Kind)
		})
	}
}

// Composition owns sentence boundaries without changing generic notifier formatting.
func TestReportingComposedPunctuation(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	b := reportingOutcome("b.example", setter.ResponseFailed, nil)
	e := Compose(NewReportingState(false).Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleStartup, Schedule: "@every 1h"})
	fragments := strings.Split(e.Notification().Format(), "\n\n")
	require.Len(t, fragments, 3)
	for _, fragment := range fragments {
		require.NotContains(t, fragment, ";")
		require.Equal(t, strings.TrimSpace(fragment), fragment)
	}
}

// Unknown Before contributes no address-context prose; a known empty snapshot does.
func TestReportingOmitsUnknownBeforeContext(t *testing.T) {
	t.Parallel()
	outcome := reportingOutcome("unknown.example", setter.ResponseFailed, nil)
	unknown := Compose(NewReportingState(false).Assess(reportingMessage(outcome)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
	require.NotContains(t, unknown, "(")
	require.NotContains(t, unknown, "none")
	require.Contains(t, unknown, "unknown.example")
	require.Contains(t, unknown, "192.0.2.2")
	outcome.result.Before = reportingSnapshot("")
	empty := Compose(NewReportingState(false).Assess(reportingMessage(outcome)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
	require.NotEqual(t, unknown, empty)
	require.Contains(t, empty, "Current: no addresses")
}

// These fixtures exercise the consumer's progress contract independently of setter
// execution. Partial progress must never be reconstructed from inference Effects.
func reportingOperation(kind api.RecordMutationKind, status setter.DNSOperationStatus, id, address string) setter.DNSOperation {
	return setter.DNSOperation{Kind: kind, Status: status, Record: api.RecordFact{ID: api.ID(id), IP: netip.MustParseAddr(address)}, Membership: api.MembershipUnknown}
}

func TestReportingProgressOwnsAssessmentAndEnvelope(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("partial.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
	o.result.Progress = &setter.DNSProgress{
		Satisfied:  []netip.Addr{netip.MustParseAddr("192.0.2.4")},
		Operations: []setter.DNSOperation{reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, "private-id", "192.0.2.2")},
	}
	a := NewReportingState(false).Assess(reportingMessage(o))
	e := Compose(a, LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	before := e.Notification().Format()
	o.result.Progress.Satisfied[0] = netip.MustParseAddr("192.0.2.99")
	o.result.Progress.Operations[0].Record.IP = netip.MustParseAddr("192.0.2.98")
	require.Equal(t, "192.0.2.4", a.dns[0].outcome.result.Progress.Satisfied[0].String())
	require.Equal(t, "192.0.2.2", a.dns[0].outcome.result.Progress.Operations[0].Record.IP.String())
	require.Equal(t, before, Compose(a, LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format())
	a.dns[0].outcome.result.Progress.Operations[0].Status = setter.DNSOperationNotAttempted
	require.Equal(t, before, e.Notification().Format())
}

func TestReportingProgressDistinguishesRequiredWork(t *testing.T) {
	t.Parallel()
	for name, status := range map[string]setter.DNSOperationStatus{"unconfirmed": setter.DNSOperationUnconfirmed, "blocked": setter.DNSOperationBlocked} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			o := reportingOutcome("partial.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
			o.target = []netip.Addr{netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.3"), netip.MustParseAddr("192.0.2.4"), netip.MustParseAddr("192.0.2.5")}
			o.result.Progress = &setter.DNSProgress{
				Satisfied: []netip.Addr{o.target[3]},
				Operations: []setter.DNSOperation{
					reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, "private-record", "192.0.2.2"),
					reportingOperation(api.RecordCreate, status, "", "192.0.2.3"),
					reportingOperation(api.RecordCreate, setter.DNSOperationNotAttempted, "", "192.0.2.4"),
				},
			}
			text := Compose(NewReportingState(false).Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
			require.Contains(t, text, "Update confirmed: 192.0.2.2")
			require.Contains(t, text, "Addition not attempted: 192.0.2.4")
			require.Contains(t, text, "Unchanged: 192.0.2.5")
			if status == setter.DNSOperationBlocked {
				require.Contains(t, text, "Addition blocked: 192.0.2.3")
			} else {
				require.Contains(t, text, "Addition unconfirmed: 192.0.2.3")
			}
			require.NotContains(t, text, "updated to")
			require.NotContains(t, text, "private-record")
			require.NotContains(t, text, "inconsistent")
			require.NotContains(t, text, "membership")
		})
	}
}

func TestReportingProgressRetainsDeletionMultiplicity(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("partial.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
	o.result.Progress = &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{
		reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, "write", "192.0.2.2"),
		reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "removed", "192.0.2.1"),
		reportingOperation(api.RecordDelete, setter.DNSOperationUnconfirmed, "uncertain", "192.0.2.1"),
		reportingOperation(api.RecordDelete, setter.DNSOperationNotAttempted, "unvisited-one", "192.0.2.1"),
		reportingOperation(api.RecordDelete, setter.DNSOperationNotAttempted, "unvisited-two", "192.0.2.1"),
	}}
	text := Compose(NewReportingState(false).Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
	require.Contains(t, text, "Update confirmed: 192.0.2.2")
	require.Contains(t, text, "Removal confirmed: 192.0.2.1 (1 record)")
	require.Contains(t, text, "Removal unconfirmed: 192.0.2.1 (1 record)")
	require.Contains(t, text, "Removal not attempted: 192.0.2.1 (2 records)")
	require.NotContains(t, text, "cleared")
	require.NotContains(t, text, "unvisited-one")
	b := o.clone()
	b.scope.domain = domain.FQDN("different.example")
	b.result.Progress.Operations = b.result.Progress.Operations[:4]
	e := Compose(NewReportingState(false).Assess(reportingMessage(o, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, strings.Split(e.Notification().Format(), "\n\n"), 2)
}

func TestReportingNoopCacheProvenance(t *testing.T) {
	t.Parallel()
	for _, address := range []string{"", "192.0.2.1"} {
		t.Run(address, func(t *testing.T) {
			t.Parallel()
			a := reportingOutcome("fresh.example", setter.ResponseNoop, reportingSnapshot(address))
			a.result.Progress = &setter.DNSProgress{Satisfied: nil, Operations: nil}
			b := a.clone()
			b.scope.domain = domain.FQDN("cached.example")
			b.result.Before.Cached = true
			text := Compose(NewReportingState(true).Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleStartup, Schedule: ""}).Notification().Format()
			lines := strings.Split(text, "\n\n")
			require.Len(t, lines, 3)
			require.Contains(t, lines[1], "Current: ")
			require.Contains(t, lines[2], "Current (cached): ")
			if address == "" {
				require.Contains(t, lines[1], "no addresses")
				require.Contains(t, lines[2], "no addresses")
			}
		})
	}
}

func TestReportingProgressRemovalGroups(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		status    setter.DNSOperationStatus
		addresses []string
		want      []string
	}{
		"distinct unvisited": {setter.DNSOperationNotAttempted, []string{"192.0.2.5", "192.0.2.6", "192.0.2.7"}, []string{"Removal not attempted:", "192.0.2.5 (1 record), 192.0.2.6 (1 record), and 192.0.2.7 (1 record)"}},
		"blocked":            {setter.DNSOperationBlocked, []string{"192.0.2.5"}, []string{"Removal blocked: 192.0.2.5", "(1 record)"}},
		"mixed multiplicity": {setter.DNSOperationCompleted, []string{"192.0.2.5", "192.0.2.5", "192.0.2.6"}, []string{"Removal confirmed:", "192.0.2.5 (2 records)", "192.0.2.6 (1 record)"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			o := reportingOutcome("cleanup.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
			o.cleanup = true
			o.result.Progress = &setter.DNSProgress{Satisfied: nil, Operations: nil}
			for i, address := range tc.addresses {
				o.result.Progress.Operations = append(o.result.Progress.Operations, reportingOperation(api.RecordDelete, tc.status, string(rune('a'+i)), address))
			}
			text := Compose(NewReportingState(false).Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
			for _, fragment := range tc.want {
				require.Contains(t, text, fragment)
			}
			require.NotContains(t, text, "cleared")
			require.NotContains(t, text, "192.0.2.5 and 192.0.2.5")
		})
	}
}

func TestReportingSuccessfulProgressUsesWholeOutcome(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("success.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	o.result.Progress.Operations[0].Membership = api.MembershipManaged
	text := Compose(NewReportingState(false).Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
	require.Contains(t, text, "Updated:")
	require.Contains(t, text, "192.0.2.1 → 192.0.2.2")
	require.NotContains(t, text, "confirmed update")
	require.NotContains(t, text, "(")
}

func TestReportingWAFEmptyFallbackRetainsAttempt(t *testing.T) {
	t.Parallel()
	for lifecycle, kind := range map[LifecycleKind]notifier.Kind{LifecycleNone: notifier.KindCleanup, LifecycleShutdown: notifier.KindShutdown, LifecycleSchedulingFailure: notifier.KindSchedulingFailure} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			before := &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}
			result := setter.WAFResult{Code: setter.ResponseNoop, Before: before, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: api.WAFListCleanupNoop, Before: before, Stages: []api.WAFCleanupStage{{WholeList: true, Items: nil, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchUnconfirmed, Returned: nil, ReturnedItems: nil}}}}}
			m := newMessage()
			m.cleanup = true
			m.waf = []wafOutcome{{result: result, filtered: false, scope: wafScope{ipnet.IP4: true, ipnet.IP6: true}, list: api.WAFList{AccountID: "account", Name: "list"}, cleanup: true}}
			e := Compose(NewReportingState(false).Assess(m), LifecycleContext{Kind: lifecycle, Schedule: ""})
			require.Contains(t, e.Notification().Format(), "List deletion unconfirmed")
			require.Contains(t, e.Notification().Format(), "Current: no items")
			require.Equal(t, kind, e.Notification().Kind)
		})
	}
}

// Deliberately cross status and membership, including combinations the current
// producer never emits, to verify conservative exclusion-label eligibility.
func TestReportingCommentExclusionRequiresCompletedReturnedMismatch(t *testing.T) {
	t.Parallel()
	for _, kind := range []api.RecordMutationKind{api.RecordCreate, api.RecordUpdate} {
		for _, status := range []setter.DNSOperationStatus{setter.DNSOperationCompleted, setter.DNSOperationUnconfirmed, setter.DNSOperationBlocked} {
			for _, membership := range []api.ManagedMembership{api.MembershipUnknown, api.MembershipManaged, api.MembershipUnmanaged} {
				o := reportingOutcome("filtered.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
				o.filtered = true
				op := reportingOperation(kind, status, "record", "192.0.2.2")
				op.Membership = membership
				o.result.Progress.Operations = []setter.DNSOperation{op}
				text := Compose(NewReportingState(false).Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
				if status == setter.DNSOperationCompleted && membership == api.MembershipUnmanaged {
					require.Contains(t, text, "Record for 192.0.2.2 excluded from management: its unexpected new comment does not match the filter.")
					lines := strings.Split(text, "\n")
					for i, line := range lines {
						if strings.Contains(line, "excluded from management") {
							require.Positive(t, i)
							require.Contains(t, lines[i-1], "confirmed:")
						}
					}
				} else {
					require.NotContains(t, text, "excluded from management")
				}
			}
		}
	}
}
