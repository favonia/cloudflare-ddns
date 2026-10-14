package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func reportingOutcome(name string, code setter.ResponseCode, before *setter.DNSSnapshot) dnsOutcome {
	return dnsOutcome{
		scope:   dnsScope{domain: domain.FQDN(name), family: ipnet.IP4},
		cleanup: false,
		target:  []netip.Addr{netip.MustParseAddr("192.0.2.2")},
		result:  setter.DNSResult{Code: code, Before: before, Effects: nil},
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
	b.result.Code = setter.ResponseNoop
	b.result.Before = reportingSnapshot("")
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
				o.result.Before = reportingSnapshot("")
			}
			s.Submitted(Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
			o.result.Code = setter.ResponseNoop
			o.result.Before = reportingSnapshot("")
			require.Equal(t, known, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
		})
	}
}

func TestReportingOneShotNoCrossRoundTracking(t *testing.T) {
	t.Parallel()
	s := NewReportingState(false)
	o := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	require.True(t, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
	o.result.Before = reportingSnapshot("192.0.2.2")
	require.True(t, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
	o.result.Code = setter.ResponseUpdated
	require.Equal(t, notifier.KindUpdate, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Kind)
}

func TestReportingObservationAdvancesOnAssess(t *testing.T) {
	t.Parallel()
	s := NewReportingState(true)
	o := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	s.Submitted(Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
	o.result.Before = reportingSnapshot("192.0.2.2")
	a := s.Assess(reportingMessage(o))
	first := Compose(a, LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Equal(t, notifier.KindStateObservation, first.Notification().Kind)
	require.Equal(t, first.Notification(), Compose(a, LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification())
	// No submission is needed to resolve the inference discrepancy.
	require.True(t, Compose(s.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
}

func TestReportingGroupingUsesAllRenderedFacts(t *testing.T) {
	t.Parallel()
	for name, modify := range map[string]func(*dnsOutcome){
		"unknown before":       func(o *dnsOutcome) { o.result.Before = nil },
		"different before":     func(o *dnsOutcome) { o.result.Before = reportingSnapshot("192.0.2.1") },
		"different target":     func(o *dnsOutcome) { o.target = nil },
		"different ownership":  func(o *dnsOutcome) { o.result.Effects[0].Membership = api.MembershipUnmanaged },
		"different completion": func(o *dnsOutcome) { o.result.Effects[0].Status = api.MutationUnconfirmed },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot(""))
			a.result.Effects = []api.RecordMutation{{Status: api.MutationCompleted, Kind: api.RecordCreate, Record: api.RecordFact{ID: "r", IP: a.target[0]}, Membership: api.MembershipManaged}}
			b := a.clone()
			b.scope.domain = domain.FQDN("b.example")
			modify(&b)
			e := Compose(NewReportingState(false).Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
			require.Len(t, e.Notification().Message, 2)
		})
	}
}

func TestReportingGroupingIgnoresIdentityMultiplicityAndCache(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	b := a.clone()
	b.scope.domain = domain.FQDN("b.example")
	b.result.Before.Cached = true
	b.result.Before.Records[0].ID = "another"
	b.result.Before.Records = append(b.result.Before.Records, b.result.Before.Records[0])
	e := Compose(NewReportingState(false).Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, e.Notification().Message, 1)
	require.Contains(t, e.Notification().Format(), "a.example")
	require.Contains(t, e.Notification().Format(), "b.example")
}

func TestReportingGroupingCanonicalizesMappedIPv4WithoutMutatingAssessment(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("::ffff:192.0.2.10"))
	a.result.Before.Records = append(a.result.Before.Records,
		api.RecordFact{ID: "ordinary", IP: netip.MustParseAddr("192.0.2.10")})
	b := reportingOutcome("b.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.10"))
	assessment := NewReportingState(false).Assess(reportingMessage(a, b))
	envelope := Compose(assessment, LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, envelope.Notification().Message, 1)
	require.Contains(t, envelope.Notification().Format(), "before=[192.0.2.10]")
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
			m.waf = []wafOutcome{{list: api.WAFList{AccountID: "account", Name: "failed-list"}, code: setter.ResponseFailed, cleanup: tc.cleanup}}
			m.problems = []operationProblem{{kind: problemDetection, family: ipnet.IP4, domains: nil}, {kind: problemFiltering, family: ipnet.IP6, domains: nil}, {kind: problemDerivation, family: ipnet.IP6, domains: []domain.Domain{domain.FQDN("derive.example")}}, {kind: problemMissingTarget, family: ipnet.IP4, domains: []domain.Domain{domain.FQDN("missing.example")}}}
			e := Compose(NewReportingState(true).Assess(m), LifecycleContext{Kind: tc.lifecycle, Schedule: "schedule"})
			require.Equal(t, tc.want, e.Notification().Kind)
			require.Len(t, e.Notification().Message, 7)
			for _, value := range []string{"failed.example", "failed-list", "derive.example", "missing.example", "schedule"} {
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
	e := Compose(NewReportingState(false).Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	text := e.Notification().Format()
	for _, fact := range []string{"192.0.2.1", "address=192.0.2.2 membership=2", "address=192.0.2.3 membership=0", "status=2", "status=1"} {
		require.Contains(t, text, fact)
	}
	require.NotContains(t, text, "private-id")
}

func TestReportingGroupingIgnoresEquivalentMechanicsAndCoverageReason(t *testing.T) {
	t.Parallel()
	s := NewReportingState(true)
	a := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	s.Submitted(Compose(s.Assess(reportingMessage(a)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
	a.result.Code = setter.ResponseUpdated
	a.result.Effects = []api.RecordMutation{{Status: api.MutationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "record", IP: a.target[0]}, Membership: api.MembershipManaged}}
	b := a.clone()
	b.scope.domain = domain.FQDN("b.example")
	b.result.Effects[0].Kind = api.RecordCreate
	b.result.Effects[0].Record.ID = "different-id"
	b.result.Effects = append(b.result.Effects, b.result.Effects[0])
	e := Compose(s.Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, e.Notification().Message, 1)
	s.Submitted(e)
	b.result.Code = setter.ResponseNoop
	b.result.Before.Cached = true
	b.result.Effects = nil
	require.True(t, Compose(s.Assess(reportingMessage(b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).IsEmpty())
}

func TestReportingGroupingRetainsObservationContext(t *testing.T) {
	t.Parallel()
	s := NewReportingState(true)
	a := reportingOutcome("a.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1"))
	s.Submitted(Compose(s.Assess(reportingMessage(a)), LifecycleContext{Kind: LifecycleNone, Schedule: ""}))
	a.result.Before = reportingSnapshot("192.0.2.2")
	b := a.clone()
	b.scope.domain = domain.FQDN("b.example")
	e := Compose(s.Assess(reportingMessage(a, b)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
	require.Len(t, e.Notification().Message, 2)
	require.Contains(t, e.Notification().Format(), "unexplained=true")
	require.Contains(t, e.Notification().Format(), "unexplained=false")
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
			m.waf = []wafOutcome{{list: api.WAFList{AccountID: "account", Name: "list"}, code: tc.code, cleanup: true}}
			e := Compose(NewReportingState(false).Assess(m), LifecycleContext{Kind: tc.lifecycle, Schedule: ""})
			require.Equal(t, tc.empty, e.IsEmpty())
			require.Equal(t, tc.want, e.Notification().Kind)
		})
	}
}
