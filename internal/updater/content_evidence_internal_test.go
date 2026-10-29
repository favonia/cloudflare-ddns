package updater

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// Provenance is a per-scope fact even when the legacy renderer uses the same words.
func TestDNSGroupingRetainsProvenance(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	b := a.clone()
	b.result.Before.Cached = true
	require.False(t, makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: a}).sameFacts(makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: b})))
}

// Defensive identity witnesses exercise consumer guarantees, not claims about
// duplicate records returned by Cloudflare.
func TestDNSPairsUseIdentityAndConservativeAmbiguity(t *testing.T) {
	t.Parallel()
	for _, ambiguous := range []bool{false, true} {
		t.Run(strconv.FormatBool(ambiguous), func(t *testing.T) {
			t.Parallel()
			o := reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot("192.0.2.9"))
			o.result.Before.Records = append(o.result.Before.Records, api.RecordFact{ID: "second", IP: netip.MustParseAddr("192.0.2.1")})
			o.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, "second", "192.0.2.8")}
			if ambiguous {
				o.result.Before.Records = append(o.result.Before.Records, api.RecordFact{ID: "second", IP: netip.MustParseAddr("192.0.2.3")})
			}
			evidence := makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o}).evidence
			require.Equal(t, !ambiguous, evidence.operations[0].paired)
			if !ambiguous {
				require.Equal(t, netip.MustParseAddr("192.0.2.1"), evidence.operations[0].old.IP)
			}
			require.False(t, evidence.deltaComplete)
		})
	}
}

func TestDNSRemovalCountsAndVisibleAnchors(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
	o.result.Before.Records = append(o.result.Before.Records, api.RecordFact{ID: "remove", IP: netip.MustParseAddr("192.0.2.1")})
	o.result.Progress.Operations = []setter.DNSOperation{
		reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, "record", "192.0.2.2"),
		reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "remove", "192.0.2.1"),
	}
	e := makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o}).evidence
	plain := e.removalFacts([]int{1}, nil)
	require.Equal(t, 1, plain.count)
	require.False(t, plain.omitOne)
	require.False(t, plain.other)
	require.True(t, e.removalFacts([]int{1}, []int{0}).other)
	// Repeated evidence is one identity, not two records.
	require.Equal(t, 1, e.removalFacts([]int{1, 1}, nil).count)
	o.result.Progress.Operations[1].Record.ID = "record"
	e = makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o}).evidence
	require.False(t, e.removalFacts([]int{1}, []int{0}).other)
	require.False(t, e.removalFacts([]int{1}, nil).countKnown)
}

func TestWAFProjectionRetainsFullCurrentAndActualPrefixes(t *testing.T) {
	t.Parallel()
	v4 := api.WAFListItem{Comment: "", ID: "v4", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	v6 := api.WAFListItem{Comment: "", ID: "v6", Prefix: netip.MustParsePrefix("2001:db8::/32")}
	o := wafOutcome{filtered: false, configured: map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}, list: api.WAFList{AccountID: "", Name: ""}, code: setter.ResponseNoop, cleanup: false, result: setter.WAFResult{Code: 0, Batches: nil, Cleanup: nil, Before: &api.WAFSnapshot{Exists: true, Cached: true, Items: []api.WAFListItem{v4, v6}}}, active: map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: false}}
	content := o.content()
	require.NotNil(t, content.before)
	require.Equal(t, []api.WAFListItem{v4, v6}, content.before.Items)
	require.Equal(t, []api.WAFListItem{v4}, content.unchanged)
	require.True(t, content.before.Cached)
	content.before.Items[0].ID = "mutated"
	require.Equal(t, api.ID("v4"), o.result.Before.Items[0].ID)
}

func TestDNSDisclosureNeedsEveryOldAddressAndExplicitEmpty(t *testing.T) {
	t.Parallel()
	for _, address := range []string{"", "192.0.2.1"} {
		t.Run(address, func(t *testing.T) {
			t.Parallel()
			o := reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot(address))
			row := makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o})
			require.False(t, row.evidence.coveredBy(dnsDisclosure{addresses: nil, knownEmpty: false}))
			require.True(t, row.evidence.coveredBy(row.disclosure))
			state := NewReportingState(true)
			envelope := Compose(state.Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleNone, Schedule: ""})
			require.Empty(t, state.covered)
			state.Submitted(envelope)
			require.True(t, state.covered[o.scope])
		})
	}
	o := reportingOutcome("a.example", setter.ResponseFailed, nil)
	require.False(t, makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o}).evidence.coveredBy(dnsDisclosure{addresses: nil, knownEmpty: true}))
	o.result.Before = reportingSnapshot("192.0.2.1")
	o.result.Before.Records = append(o.result.Before.Records, api.RecordFact{ID: "second", IP: netip.MustParseAddr("192.0.2.2")})
	require.False(t, makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o}).evidence.coveredBy(dnsDisclosure{knownEmpty: false, addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}))
}

func TestDNSRemovalSingularRequiresCompleteIdentity(t *testing.T) {
	t.Parallel()
	for _, before := range []*setter.DNSSnapshot{nil, reportingSnapshot(""), reportingSnapshot("192.0.2.1")} {
		o := reportingOutcome("a.example", setter.ResponseFailed, before)
		o.result.Progress = &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{reportingOperation(api.RecordDelete, setter.DNSOperationUnconfirmed, "record", "192.0.2.1")}}
		facts := makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o}).evidence.removalFacts([]int{0}, nil)
		require.True(t, facts.countKnown)
		require.Equal(t, before != nil && len(before.Records) > 0, facts.omitOne)
	}
}

func TestWAFContentUnknownMissingEmptyAndFailedBatches(t *testing.T) {
	t.Parallel()
	for _, snapshot := range []*api.WAFSnapshot{nil, {Cached: false, Items: nil, Exists: false}, {Cached: false, Items: nil, Exists: true}, {Items: nil, Exists: true, Cached: true}} {
		o := wafOutcome{filtered: false, configured: map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}, active: nil, list: api.WAFList{AccountID: "", Name: ""}, code: setter.ResponseNoop, cleanup: false, result: setter.WAFResult{Code: 0, Batches: nil, Cleanup: nil, Before: snapshot}}
		require.Equal(t, snapshot, o.content().before)
	}
	old := api.WAFListItem{Comment: "", ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	next := api.WAFListItem{Comment: "", ID: "new", Prefix: netip.MustParsePrefix("192.0.2.1/32")}
	for _, result := range []*api.WAFBatchResult{nil, {PrerequisiteMutation: false, OK: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchNoEffect}, {PrerequisiteMutation: false, OK: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchUnconfirmed}, {PrerequisiteMutation: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchCompleted, OK: false}, {PrerequisiteMutation: false, ReturnedItems: nil, Status: api.WAFBatchCompleted, OK: true, Returned: &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{next}}}} {
		o := wafOutcome{filtered: false, configured: map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}, list: api.WAFList{AccountID: "", Name: ""}, code: setter.ResponseNoop, cleanup: false, active: map[ipnet.Family]bool{ipnet.IP4: true}, result: setter.WAFResult{Code: 0, Cleanup: nil, Before: &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{old}}, Batches: []setter.WAFBatch{{Create: nil, Delete: []api.WAFListItem{old}, Result: result}}}}
		content := o.content()
		known := result != nil && result.OK
		require.Equal(t, known, content.deltaKnown)
		if known {
			require.Equal(t, []api.WAFListItem{old}, content.removed)
			require.Equal(t, []api.WAFListItem{next}, content.added)
		} else {
			require.Empty(t, content.removed)
			require.Empty(t, content.added)
		}
	}
}

func TestWAFCleanupSnapshotFollowsWholeListFailure(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{Comment: "", ID: "post-failure", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	o := wafOutcome{filtered: false, configured: map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}, list: api.WAFList{AccountID: "", Name: ""}, code: setter.ResponseNoop, cleanup: false, active: map[ipnet.Family]bool{ipnet.IP4: true}, result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: 0, Before: &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{item}}, Stages: []api.WAFCleanupStage{
		{Items: nil, WholeList: true, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchUnconfirmed}},
		{WholeList: false, Items: []api.WAFListItem{item}, Result: api.WAFBatchResult{PrerequisiteMutation: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchCompleted, OK: true}},
	}}}}
	content := o.content()
	require.True(t, content.deltaKnown)
	require.Equal(t, []api.WAFListItem{item}, content.removed)
	require.Equal(t, []api.WAFListItem{item}, content.before.Items)
	require.False(t, o.result.Cleanup.Stages[0].Result.OK)
}

func TestDNSGroupingChecksOriginalScopeRemovalEligibility(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
	a.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "record", "192.0.2.1")}
	b := a.clone()
	b.result.Before.Records = append(b.result.Before.Records, api.RecordFact{ID: "another", IP: netip.MustParseAddr("192.0.2.1")})
	require.False(t, makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: a}).sameFacts(makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: b})))
}

func TestDNSCompleteDeltaRejectsMembershipAnomaly(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	o.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, "record", "192.0.2.2")}
	o.result.Progress.Operations[0].Membership = api.MembershipManaged
	require.True(t, makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o}).evidence.deltaComplete)
	o.result.Progress.Operations[0].Membership = api.MembershipUnmanaged
	row := makeDNSRow(assessedDNS{firstKnown: false, unexplained: false, outcome: o})
	require.True(t, row.evidence.operations[0].paired)
	require.False(t, row.evidence.deltaComplete)
	require.True(t, row.evidence.coveredBy(row.disclosure))
}

func TestWAFCleanupDoesNotPromoteConflictingIdentity(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{ID: "same", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: ""}
	conflicting := item
	conflicting.Prefix = netip.MustParsePrefix("192.0.3.0/24")
	var outcome wafOutcome
	outcome.configured = map[ipnet.Family]bool{ipnet.IP4: true}
	outcome.active = map[ipnet.Family]bool{ipnet.IP4: true}
	outcome.result.Cleanup = &api.WAFCleanupResult{Code: api.WAFListCleanupUpdated, Before: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item, conflicting}}, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{item}, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: nil, ReturnedItems: nil}}}}
	content := outcome.content()
	require.False(t, content.deltaKnown)
	require.Empty(t, content.removed)
}

func TestWAFMembershipAnomalyRetainsOperationWithoutContentDelta(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{ID: "original", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "owned"}
	changed := item
	changed.Comment = "outside-selector"
	var outcome wafOutcome
	outcome.configured = map[ipnet.Family]bool{ipnet.IP4: true}
	outcome.active = map[ipnet.Family]bool{ipnet.IP4: true}
	outcome.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item}}
	outcome.result.Batches = []setter.WAFBatch{{Create: nil, Delete: []api.WAFListItem{item}, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}, ReturnedItems: []api.WAFListItem{changed}}}}
	content := outcome.content()
	require.False(t, content.deltaKnown)
	require.Empty(t, content.removed)
	require.Equal(t, api.WAFBatchCompleted, outcome.result.Batches[0].Result.Status)
}

func TestDNSKnownEmptyRetainsMutationChronology(t *testing.T) {
	t.Parallel()
	for _, status := range []setter.DNSOperationStatus{setter.DNSOperationNotAttempted, setter.DNSOperationBlocked, setter.DNSOperationUnconfirmed, setter.DNSOperationCompleted} {
		o := reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot(""))
		o.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordCreate, status, "new", "192.0.2.1")}
		row := makeDNSRow(assessedDNS{outcome: o, firstKnown: false, unexplained: false})
		require.True(t, row.disclosure.knownEmpty)
		require.Equal(t, status == setter.DNSOperationUnconfirmed || status == setter.DNSOperationCompleted, row.evidence.inputHistorical)
	}
}

func TestDNSGroupedSourcesRemainIndependent(t *testing.T) {
	t.Parallel()
	a := reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
	b := a.clone()
	b.result.Before.Records[0].ID = "other"
	b.result.Progress.Operations[0].Record.ID = "other"
	rows := appendDNSRow(nil, makeDNSRow(assessedDNS{outcome: a, firstKnown: false, unexplained: false}))
	rows = appendDNSRow(rows, makeDNSRow(assessedDNS{outcome: b, firstKnown: false, unexplained: false}))
	require.Len(t, rows, 1)
	require.Len(t, rows[0].sources, 2)
	require.Equal(t, api.ID("record"), rows[0].sources[0].records[0].ID)
	require.Equal(t, api.ID("other"), rows[0].sources[1].records[0].ID)
}

func TestDNSContradictoryDeletionDoesNotEstablishCountOrDelta(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("a.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	o.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordDelete, setter.DNSOperationCompleted, "record", "192.0.2.9")}
	e := makeDNSRow(assessedDNS{outcome: o, firstKnown: false, unexplained: false}).evidence
	require.False(t, e.deltaComplete)
	require.False(t, e.removalFacts([]int{0}, nil).countKnown)
	o.result.Before = reportingSnapshot("")
	e = makeDNSRow(assessedDNS{outcome: o, firstKnown: false, unexplained: false}).evidence
	require.False(t, e.deltaComplete)
}

func TestDNSMissingIdentityDoesNotPairSortedAddresses(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"", "not-in-snapshot"} {
		o := reportingOutcome("a.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
		o.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, id, "192.0.2.2")}
		e := makeDNSRow(assessedDNS{outcome: o, firstKnown: false, unexplained: false}).evidence
		require.False(t, e.operations[0].paired)
		require.False(t, e.deltaComplete)
	}
}

// This two-batch consumer-contract witness does not assert a live remote race.
func TestWAFContentTracksOwnershipAcrossBatches(t *testing.T) {
	t.Parallel()
	for _, leavesOwnership := range []bool{false, true} {
		t.Run(strconv.FormatBool(leavesOwnership), func(t *testing.T) {
			t.Parallel()
			old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "owned"}
			created := api.WAFListItem{ID: "created", Prefix: netip.MustParsePrefix("198.51.100.0/24"), Comment: "owned"}
			returned := created
			selected := []api.WAFListItem{created}
			if leavesOwnership {
				returned.Comment = "outside-selector"
				selected = nil
			}
			var outcome wafOutcome
			outcome.configured = map[ipnet.Family]bool{ipnet.IP4: true}
			outcome.active = map[ipnet.Family]bool{ipnet.IP4: true}
			outcome.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old}}
			outcome.result.Batches = []setter.WAFBatch{
				{Create: []api.WAFListCreateItem{{Prefix: created.Prefix, Comment: created.Comment}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old, created}}, ReturnedItems: []api.WAFListItem{old, created}}},
				{Create: nil, Delete: []api.WAFListItem{old}, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: selected}, ReturnedItems: []api.WAFListItem{returned}}},
			}
			content := outcome.content()
			require.Equal(t, !leavesOwnership, content.deltaKnown)
			if leavesOwnership {
				require.Empty(t, content.added)
				require.Empty(t, content.removed)
			} else {
				require.Equal(t, []api.WAFListItem{old}, content.removed)
				require.Equal(t, []api.WAFListItem{created}, content.added)
			}
			require.Equal(t, []api.WAFListItem{old}, content.before.Items)
			require.Equal(t, []api.WAFListItem{old}, outcome.result.Before.Items)
			for _, batch := range outcome.result.Batches {
				require.Equal(t, api.WAFBatchCompleted, batch.Result.Status)
				require.True(t, batch.Result.OK)
			}
			require.Equal(t, []api.WAFListItem{returned}, outcome.result.Batches[1].Result.ReturnedItems)
		})
	}
}

func TestWAFContentComparesRetainedPrefixes(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"comment", "identity", "both"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			old := api.WAFListItem{ID: "retained", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "owned-old"}
			retained := old
			if change != "identity" {
				retained.Comment = "owned-new"
			}
			if change != "comment" {
				retained.ID = "replacement"
			}
			added := api.WAFListItem{ID: "added", Prefix: netip.MustParsePrefix("198.51.100.0/24"), Comment: "owned-new"}
			var outcome wafOutcome
			outcome.configured = map[ipnet.Family]bool{ipnet.IP4: true}
			outcome.active = map[ipnet.Family]bool{ipnet.IP4: true}
			outcome.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old}}
			outcome.result.Batches = []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: added.Prefix, Comment: added.Comment}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{retained, added}}, ReturnedItems: []api.WAFListItem{retained, added}}}}
			content := outcome.content()
			require.True(t, content.deltaKnown)
			require.Empty(t, content.removed)
			require.Equal(t, []api.WAFListItem{added}, content.added)
			require.Equal(t, []api.WAFListItem{old}, content.unchanged)
			require.Equal(t, []api.WAFListItem{old}, content.before.Items)
			require.Equal(t, []api.WAFListItem{retained, added}, outcome.result.Batches[0].Result.Returned.Items)
		})
	}
}

// Equal prefixes cannot attribute an unowned item to a create request.
func TestWAFPrefixOnlyDoesNotAttributeExclusion(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{Comment: "", ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	foreign := api.WAFListItem{ID: "foreign", Prefix: netip.MustParsePrefix("198.51.100.0/24"), Comment: "foreign"}
	o := wafOutcome{result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, list: api.WAFList{AccountID: "", Name: ""}, code: 0, cleanup: false, configured: map[ipnet.Family]bool{ipnet.IP4: true}, active: map[ipnet.Family]bool{ipnet.IP4: true}}
	o.result.Before = &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{old}}
	o.result.Batches = []setter.WAFBatch{{Delete: nil, Create: []api.WAFListCreateItem{{Comment: "", Prefix: foreign.Prefix}}, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{old}}, ReturnedItems: []api.WAFListItem{old, foreign}}}}
	content := o.content()
	require.False(t, content.deltaKnown)
	require.Equal(t, o.result.Batches[0].Result.Returned, content.observed)
	require.Empty(t, content.exclusions)
	require.Empty(t, content.issues)
}

// A deleted identity does not remove a prefix retained by another identity.
func TestWAFCleanupSubtractsIdentityBeforePrefixProjection(t *testing.T) {
	t.Parallel()
	first := api.WAFListItem{Comment: "", ID: "first", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	second := first
	second.ID = "second"
	o := wafOutcome{result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, list: api.WAFList{AccountID: "", Name: ""}, code: 0, cleanup: false, configured: map[ipnet.Family]bool{ipnet.IP4: true}, active: map[ipnet.Family]bool{ipnet.IP4: true}}
	o.result.Cleanup = &api.WAFCleanupResult{Code: 0, Before: &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{first, second}}, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{first}, Result: api.WAFBatchResult{Returned: nil, ReturnedItems: nil, PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted}}}}
	c := o.content()
	require.True(t, c.deltaKnown)
	require.Empty(t, c.removed)
	require.Equal(t, []api.WAFListItem{first}, c.unchanged)
}
