package updater

import (
	"context"
	"net/netip"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// lifecycleKind identifies context supplied by the composition root.
type lifecycleKind uint8

// Lifecycle contexts supported by ScheduledReporter.Report.
const (
	lifecycleNone lifecycleKind = iota
	lifecycleStartup
	lifecycleShutdown
	lifecycleSchedulingFailure
)

// LifecycleContext describes the service event accompanying an operation.
// Its zero value means no lifecycle event. Use constructors for service events;
// the context affects notification presentation, not work or scheduling.
type LifecycleContext struct {
	kind     lifecycleKind
	schedule string
}

// NewNoLifecycleContext describes an ordinary round without a service event.
// It is equivalent to the zero value of LifecycleContext.
func NewNoLifecycleContext() LifecycleContext {
	return LifecycleContext{kind: lifecycleNone, schedule: ""}
}

// NewStartupLifecycleContext describes service startup.
func NewStartupLifecycleContext() LifecycleContext {
	return LifecycleContext{kind: lifecycleStartup, schedule: ""}
}

// NewShutdownLifecycleContext describes service shutdown.
func NewShutdownLifecycleContext() LifecycleContext {
	return LifecycleContext{kind: lifecycleShutdown, schedule: ""}
}

// NewSchedulingFailureLifecycleContext describes termination because the schedule
// has no future execution. schedule is the caller's human-readable description;
// it is included verbatim and is not interpreted as a scheduling expression.
func NewSchedulingFailureLifecycleContext(schedule string) LifecycleContext {
	return LifecycleContext{kind: lifecycleSchedulingFailure, schedule: schedule}
}

// ReportOneShot selects and submits one operation's notification through a nonnil
// notifier, without lifecycle context or cross-round state. Ordinary no-ops
// without reportable reached-operation evidence do not call Send. Submission
// is synchronous; delivery success does not change the supplied work result.
// The caller retains responsibility for heartbeat delivery and its ordering.
func ReportOneShot(ctx context.Context, ppfmt pp.PP, nt notifier.Notifier, message Message) {
	envelope := compose(snapshotAssessment(message), LifecycleContext{kind: lifecycleNone, schedule: ""})
	if !envelope.isEmpty() {
		nt.Send(ctx, ppfmt, envelope.notificationValue())
	}
}

// ScheduledReporter owns notification inference, submitted address coverage,
// and delivery for one scheduled process with a stable managed ownership scope.
// Construct it with NewScheduledReporter and use Report serially.
type ScheduledReporter struct {
	state    *reportingState
	notifier notifier.Notifier
}

// NewScheduledReporter binds a nonnil notifier to a fresh scheduled reporting
// state. The notifier remains responsible for transport and delivery diagnostics.
func NewScheduledReporter(nt notifier.Notifier) *ScheduledReporter {
	return &ScheduledReporter{state: newReportingState(), notifier: nt}
}

// Report handles one completed update or cleanup result and its lifecycle context.
// Pass nil when no operation took place, such as startup without an update or
// shutdown without cleanup. Call once per completed result, including silent
// no-ops: inference advances even when no notification is selected. Nonempty
// notifications advance coverage immediately before Send, regardless of delivery
// success. Report is synchronous and does not change the supplied work result.
func (r *ScheduledReporter) Report(ctx context.Context, ppfmt pp.PP, message *Message, lifecycle LifecycleContext) {
	a := assessment{dns: nil, waf: nil, problems: nil, cleanup: false}
	if message != nil {
		a = r.state.observeRound(*message)
	}
	envelope := compose(a, lifecycle)
	if envelope.isEmpty() {
		return
	}
	r.state.recordSubmission(envelope)
	r.notifier.Send(ctx, ppfmt, envelope.notificationValue())
}

// reportingState owns process-local DNS inference and submitted Before coverage.
// ScheduledReporter owns the observation/submission sequence; pure composition
// and immutable assessments remain separate internal units.
type reportingState struct {
	inference map[dnsScope]*dnsInference
	covered   map[dnsScope]bool
}

func newReportingState() *reportingState {
	return &reportingState{
		inference: make(map[dnsScope]*dnsInference),
		covered:   make(map[dnsScope]bool),
	}
}

type assessedDNS struct {
	outcome     dnsOutcome
	firstKnown  bool
	unexplained bool
}

// assessment owns an immutable snapshot of one assessed round and its selection
// context. Recomposition retains this original context even after submission;
// it does not query later reporting state. Its zero value supports lifecycle-only context.
type assessment struct {
	dns      []assessedDNS
	waf      []wafOutcome
	problems []operationProblem
	cleanup  bool
}

// snapshotAssessment detaches one round's facts without interpreting history or
// changing state. One-shot reporting and scheduled observation share this snapshot.
func snapshotAssessment(message Message) assessment {
	a := assessment{
		dns:      make([]assessedDNS, 0, len(message.dns)),
		waf:      make([]wafOutcome, 0, len(message.waf)),
		problems: make([]operationProblem, 0, len(message.problems)),
		cleanup:  message.cleanup,
	}
	for _, o := range message.waf {
		a.waf = append(a.waf, o.clone())
	}
	for _, o := range message.dns {
		a.dns = append(a.dns, assessedDNS{outcome: o.clone(), firstKnown: false, unexplained: false})
	}
	for _, p := range message.problems {
		p.domains = slices.Clone(p.domains)
		a.problems = append(a.problems, p)
	}
	return a
}

// observeRound advances inference exactly once for each invoked scope and
// snapshots missing coverage without consuming it.
func (s *reportingState) observeRound(message Message) assessment {
	a := snapshotAssessment(message)
	for i := range a.dns {
		entry := &a.dns[i]
		o := entry.outcome
		model := s.inference[o.scope]
		if model == nil {
			model = &dnsInference{known: false, factors: nil}
			s.inference[o.scope] = model
		}
		entry.unexplained = model.assess(o.result)
		entry.firstKnown = !s.covered[o.scope] && o.result.Before != nil
	}
	return a
}

// envelope owns the selected, grouped notification and the known Before scopes
// actually rendered in it. Reading or composing an envelope never advances coverage.
type envelope struct {
	notification notifier.Notification
	beforeScopes []dnsScope
}

// notificationValue returns an independently owned message for existing transports.
func (e envelope) notificationValue() notifier.Notification {
	return notifier.NewNotification(e.notification.Kind, slices.Clone(e.notification.Message))
}

// isEmpty reports whether there is any selected notification content.
func (e envelope) isEmpty() bool { return e.notification.IsEmpty() }

// recordSubmission marks the known Before information in a nonempty envelope
// as covered.
// The envelope must come from this state and ownership scope. Call immediately
// before Notifier.Send, irrespective of its eventual return value.
func (s *reportingState) recordSubmission(e envelope) {
	if e.isEmpty() {
		return
	}
	for _, scope := range e.beforeScopes {
		s.covered[scope] = true
	}
}

type dnsRow struct {
	// Each grouped constituent retains its own identity/visibility qualifications.
	// A renderer must check removal anchors per source before sharing a line.
	sources      []dnsEvidence
	evidence     dnsEvidence
	disclosure   dnsDisclosure
	filtered     bool
	domains      []domain.Domain
	family       ipnet.Family
	cleanup      bool
	code         setter.ResponseCode
	beforeKnown  bool
	beforeCached bool
	before       []netip.Addr
	target       []netip.Addr
	satisfied    []netip.Addr
	operations   []setter.DNSOperation
	unexplained  bool
}

// compose purely selects and groups semantic facts from an owned assessment.
// Startup includes all assessed scopes; ordinary rounds include operation outcomes,
// problems, uncovered Before context and unexplained fresh observations.
func compose(a assessment, lifecycle LifecycleContext) envelope {
	rows := make([]dnsRow, 0, len(a.dns))
	beforeScopes := make([]dnsScope, 0, len(a.dns))
	for _, entry := range a.dns {
		if lifecycle.kind != lifecycleStartup && entry.outcome.result.Code == setter.ResponseNoop &&
			!entry.firstKnown && !entry.unexplained {
			continue
		}
		row := makeDNSRow(entry)
		if row.evidence.coveredBy(row.disclosure) {
			beforeScopes = append(beforeScopes, entry.outcome.scope)
		}
		grouped := false
		body, _ := renderDNSBody(row)
		for i := range rows {
			otherBody, _ := renderDNSBody(rows[i])
			if rows[i].sameFacts(row) && slices.Equal(body, otherBody) {
				rows[i].domains = append(rows[i].domains, row.domains...)
				rows[i].sources = append(rows[i].sources, row.sources...)
				grouped = true
				break
			}
		}
		if !grouped {
			rows = append(rows, row)
		}
	}
	message := make(notifier.Message, 0, len(rows)+len(a.waf)+len(a.problems)+1)
	if lifecycle.kind != lifecycleNone {
		message = append(message, renderLifecycle(lifecycle))
	}
	for _, family := range []ipnet.Family{ipnet.IP4, ipnet.IP6} {
		for _, problem := range a.problems {
			if problem.family == family {
				message = append(message, renderProblem(problem))
			}
		}
		for _, row := range rows {
			if row.family == family {
				message = append(message, renderDNS(row))
			}
		}
	}
	for _, o := range a.waf {
		if wafHasOperation(o) || lifecycle.kind == lifecycleStartup {
			message = append(message, renderWAF(o))
		}
	}
	// This producer owns block boundaries. One element preserves its newlines
	// through Message.Format without changing unrelated notifier constructors.
	if len(message) > 0 {
		message = notifier.Message{strings.Join(message, "\n\n")}
	}
	return envelope{
		notification: notifier.NewNotification(notificationKind(a, lifecycle), message),
		beforeScopes: beforeScopes,
	}
}

func makeDNSRow(entry assessedDNS) dnsRow {
	o := entry.outcome
	row := dnsRow{
		sources:      nil,
		evidence:     makeDNSEvidence(o),
		disclosure:   dnsDisclosure{addresses: nil, knownEmpty: false},
		filtered:     o.filtered,
		domains:      []domain.Domain{o.scope.domain},
		family:       o.scope.family,
		cleanup:      o.cleanup,
		code:         o.result.Code,
		beforeKnown:  o.result.Before != nil,
		beforeCached: false,
		before:       nil,
		target:       canonicalAddresses(o.target),
		satisfied:    nil,
		operations:   nil,
		unexplained:  entry.unexplained,
	}
	if o.result.Before != nil {
		row.beforeCached = o.result.Before.Cached
		for _, r := range o.result.Before.Records {
			row.before = append(row.before, r.IP)
		}
		row.before = canonicalAddresses(row.before)
	}
	if o.result.Progress != nil {
		row.satisfied = canonicalAddresses(o.result.Progress.Satisfied)
		row.operations = slices.Clone(o.result.Progress.Operations)
		for i := range row.operations {
			row.operations[i].Record.IP = row.operations[i].Record.IP.Unmap()
		}
	}
	_, row.disclosure = renderDNSBody(row)
	row.sources = []dnsEvidence{row.evidence}
	return row
}

func canonicalAddresses(addresses []netip.Addr) []netip.Addr {
	owned := slices.Clone(addresses)
	for i := range owned {
		owned[i] = owned[i].Unmap()
	}
	slices.SortFunc(owned, netip.Addr.Compare)
	return slices.Compact(owned)
}

// Reached cleanup stages remain reportable even when a later fresh empty
// fallback makes the aggregate work result a successful no-op.
func wafHasOperation(o wafOutcome) bool {
	return o.result.Code != setter.ResponseNoop || (o.result.Cleanup != nil && len(o.result.Cleanup.Stages) > 0)
}

func notificationKind(a assessment, lifecycle LifecycleContext) notifier.Kind {
	if lifecycle.kind == lifecycleSchedulingFailure {
		return notifier.KindSchedulingFailure
	}
	failed := len(a.problems) > 0
	operation := failed
	cleanup := a.cleanup
	for _, entry := range a.dns {
		failed = failed || entry.outcome.result.Code == setter.ResponseFailed
		operation = operation || entry.outcome.result.Code != setter.ResponseNoop
		cleanup = cleanup || entry.outcome.cleanup
	}
	for _, o := range a.waf {
		failed = failed || o.result.Code == setter.ResponseFailed
		operation = operation || wafHasOperation(o)
		cleanup = cleanup || o.cleanup
	}
	switch {
	case failed && cleanup:
		return notifier.KindCleanupFailure
	case failed:
		return notifier.KindUpdateFailure
	case lifecycle.kind == lifecycleStartup:
		return notifier.KindStartup
	case lifecycle.kind == lifecycleShutdown:
		return notifier.KindShutdown
	case operation && cleanup:
		return notifier.KindCleanup
	case operation:
		return notifier.KindUpdate
	default:
		return notifier.KindStateObservation
	}
}
