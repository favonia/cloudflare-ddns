package updater

import (
	"cmp"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// LifecycleKind identifies context supplied by the composition root.
type LifecycleKind uint8

// Lifecycle contexts supported by Compose.
const (
	LifecycleNone LifecycleKind = iota
	LifecycleStartup
	LifecycleShutdown
	LifecycleSchedulingFailure
)

// LifecycleContext describes the service event accompanying an operation.
// Schedule is supplied by the caller; it does not affect work or scheduling.
type LifecycleContext struct {
	Kind     LifecycleKind
	Schedule string
}

// ReportingState owns process-local DNS inference and submitted Before coverage
// under one stable managed ownership scope. Call Assess once per completed round,
// then Compose and Submitted serially. Delivery success does not affect this state.
type ReportingState struct {
	scheduled bool
	inference map[dnsScope]*dnsInference
	covered   map[dnsScope]bool
}

// NewReportingState enables cross-round inference and first-known coverage only
// for scheduled processes. One-shot operations still retain their operation facts.
func NewReportingState(scheduled bool) *ReportingState {
	return &ReportingState{
		scheduled: scheduled,
		inference: make(map[dnsScope]*dnsInference),
		covered:   make(map[dnsScope]bool),
	}
}

type assessedDNS struct {
	outcome     dnsOutcome
	firstKnown  bool
	unexplained bool
}

// Assessment owns an immutable snapshot of one assessed round and its selection
// context. Its zero value can be composed with lifecycle-only context.
type Assessment struct {
	dns      []assessedDNS
	waf      []wafOutcome
	problems []operationProblem
	cleanup  bool
}

// Assess advances inference exactly once for each invoked DNS scope, independently
// of composition or delivery. It snapshots missing coverage without consuming it.
// The caller must not assess the same completed round more than once.
func (s *ReportingState) Assess(message Message) Assessment {
	a := Assessment{
		dns:      make([]assessedDNS, 0, len(message.dns)),
		waf:      slices.Clone(message.waf),
		problems: make([]operationProblem, 0, len(message.problems)),
		cleanup:  message.cleanup,
	}
	for _, o := range message.dns {
		entry := assessedDNS{outcome: o.clone(), firstKnown: false, unexplained: false}
		if s.scheduled {
			model := s.inference[o.scope]
			if model == nil {
				model = &dnsInference{known: false, factors: nil}
				s.inference[o.scope] = model
			}
			entry.unexplained = model.assess(o.result)
			entry.firstKnown = !s.covered[o.scope] && o.result.Before != nil
		}
		a.dns = append(a.dns, entry)
	}
	for _, p := range message.problems {
		p.domains = slices.Clone(p.domains)
		a.problems = append(a.problems, p)
	}
	return a
}

// Envelope owns the selected, grouped notification and the known Before scopes
// actually rendered in it. Reading or composing an envelope never advances coverage.
type Envelope struct {
	notification notifier.Notification
	beforeScopes []dnsScope
}

// Notification returns an independently owned message for existing transports.
func (e Envelope) Notification() notifier.Notification {
	return notifier.NewNotification(e.notification.Kind, slices.Clone(e.notification.Message))
}

// IsEmpty reports whether there is any selected notification content.
func (e Envelope) IsEmpty() bool { return e.notification.IsEmpty() }

// Submitted records only known Before information included in a nonempty envelope.
// Call immediately before Notifier.Send, irrespective of its eventual return value.
func (s *ReportingState) Submitted(e Envelope) {
	if !s.scheduled || e.IsEmpty() {
		return
	}
	for _, scope := range e.beforeScopes {
		s.covered[scope] = true
	}
}

// completionFact preserves supported operation evidence, without implying a
// read-back or exact final managed membership. Create/update share the same
// address-operation claim. IDs, multiplicity and operation order are not rendered.
type completionFact struct {
	status     api.MutationStatus
	removal    bool
	address    netip.Addr
	membership api.ManagedMembership
}

type dnsRow struct {
	domains     []domain.Domain
	family      ipnet.Family
	cleanup     bool
	code        setter.ResponseCode
	beforeKnown bool
	before      []netip.Addr
	target      []netip.Addr
	completion  []completionFact
	unexplained bool
}

// Compose purely selects and groups semantic facts from an owned assessment.
// Startup includes all assessed scopes; ordinary rounds include operation outcomes,
// problems, uncovered Before context and unexplained fresh observations.
func Compose(a Assessment, lifecycle LifecycleContext) Envelope {
	rows := make([]dnsRow, 0, len(a.dns))
	beforeScopes := make([]dnsScope, 0, len(a.dns))
	for _, entry := range a.dns {
		if lifecycle.Kind != LifecycleStartup && entry.outcome.result.Code == setter.ResponseNoop &&
			!entry.firstKnown && !entry.unexplained {
			continue
		}
		row := makeDNSRow(entry)
		if row.beforeKnown {
			beforeScopes = append(beforeScopes, entry.outcome.scope)
		}
		grouped := false
		for i := range rows {
			if rows[i].sameFacts(row) {
				rows[i].domains = append(rows[i].domains, row.domains...)
				grouped = true
				break
			}
		}
		if !grouped {
			rows = append(rows, row)
		}
	}
	message := make(notifier.Message, 0, len(rows)+len(a.waf)+len(a.problems)+1)
	if lifecycle.Kind != LifecycleNone {
		message = append(message, renderLifecycle(lifecycle))
	}
	for _, row := range rows {
		message = append(message, renderDNS(row))
	}
	for _, o := range a.waf {
		if o.code != setter.ResponseNoop || lifecycle.Kind == LifecycleStartup {
			message = append(message, renderWAF(o))
		}
	}
	for _, p := range a.problems {
		message = append(message, renderProblem(p))
	}
	return Envelope{
		notification: notifier.NewNotification(notificationKind(a, lifecycle), message),
		beforeScopes: beforeScopes,
	}
}

func makeDNSRow(entry assessedDNS) dnsRow {
	o := entry.outcome
	row := dnsRow{
		domains:     []domain.Domain{o.scope.domain},
		family:      o.scope.family,
		cleanup:     o.cleanup,
		code:        o.result.Code,
		beforeKnown: o.result.Before != nil,
		before:      nil,
		target:      canonicalAddresses(o.target),
		completion:  nil,
		unexplained: entry.unexplained,
	}
	if o.result.Before != nil {
		for _, r := range o.result.Before.Records {
			row.before = append(row.before, r.IP)
		}
		row.before = canonicalAddresses(row.before)
	}
	for _, effect := range o.result.Effects {
		if effect.Status == api.MutationNoEffect {
			continue
		}
		fact := completionFact{
			status:     effect.Status,
			removal:    effect.Kind == api.RecordDelete,
			address:    effect.Record.IP.Unmap(),
			membership: effect.Membership,
		}
		row.completion = append(row.completion, fact)
	}
	slices.SortFunc(row.completion, compareCompletion)
	row.completion = slices.Compact(row.completion)
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

func compareCompletion(a, b completionFact) int {
	removalOrder := 0
	if a.removal != b.removal {
		if a.removal {
			removalOrder = 1
		} else {
			removalOrder = -1
		}
	}
	return cmp.Or(cmp.Compare(a.status, b.status), removalOrder,
		a.address.Compare(b.address), cmp.Compare(a.membership, b.membership))
}

func (r dnsRow) sameFacts(other dnsRow) bool {
	return r.family == other.family && r.cleanup == other.cleanup && r.code == other.code &&
		r.beforeKnown == other.beforeKnown && slices.Equal(r.before, other.before) &&
		slices.Equal(r.target, other.target) && slices.Equal(r.completion, other.completion) &&
		r.unexplained == other.unexplained
}

func notificationKind(a Assessment, lifecycle LifecycleContext) notifier.Kind {
	if lifecycle.Kind == LifecycleSchedulingFailure {
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
		failed = failed || o.code == setter.ResponseFailed
		operation = operation || o.code != setter.ResponseNoop
		cleanup = cleanup || o.cleanup
	}
	switch {
	case failed && cleanup:
		return notifier.KindCleanupFailure
	case failed:
		return notifier.KindUpdateFailure
	case lifecycle.Kind == LifecycleStartup:
		return notifier.KindStartup
	case lifecycle.Kind == LifecycleShutdown:
		return notifier.KindShutdown
	case operation && cleanup:
		return notifier.KindCleanup
	case operation:
		return notifier.KindUpdate
	default:
		return notifier.KindStateObservation
	}
}
