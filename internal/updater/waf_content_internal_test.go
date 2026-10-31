package updater

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// These private-unit tests verify facts consumed by rendering without locking prose.
func TestWAFObservedInventoryAndChronology(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{Comment: "", ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	inactive := api.WAFListItem{Comment: "", ID: "inactive", Prefix: netip.MustParsePrefix("2001:db8::/32")}
	next := api.WAFListItem{Comment: "", ID: "next", Prefix: netip.MustParsePrefix("198.51.100.0/24")}
	for _, cached := range []bool{false, true} {
		o := wafOutcome{result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, list: api.WAFList{AccountID: "", Name: ""}, cleanup: false, scope: wafScope{ipnet.IP4: true, ipnet.IP6: false}}
		o.result.Before = &api.WAFSnapshot{Exists: true, Cached: cached, Items: []api.WAFListItem{old, inactive}}
		c := o.content()
		require.Equal(t, o.result.Before, c.observed)
		require.False(t, c.inputHistorical)
		c.observed.Items[0].ID = "detached"
		require.Equal(t, api.ID("old"), o.result.Before.Items[0].ID)
		delete(o.scope, ipnet.IP6)
		require.Equal(t, []api.WAFListItem{old}, o.content().observed.Items)
		o.scope[ipnet.IP6] = false
		request := []api.WAFListCreateItem{{Comment: "", Prefix: next.Prefix}}
		for _, structural := range []bool{false, true} {
			o.result.Batches = []setter.WAFBatch{{Delete: nil, Create: request, Result: &api.WAFBatchResult{OK: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchNoEffect, PrerequisiteMutation: structural}}}
			c = o.content()
			require.Equal(t, structural, c.inputHistorical)
			require.Equal(t, !structural, c.observed != nil)
		}
		returned := &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{next, inactive}}
		o.result.Batches = []setter.WAFBatch{{Delete: nil, Create: request, Result: &api.WAFBatchResult{ReturnedItems: returned.Items, PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: returned}}, {Create: nil, Delete: nil, Result: &api.WAFBatchResult{Returned: nil, ReturnedItems: nil, PrerequisiteMutation: false, OK: true, Status: api.WAFBatchNoEffect}}}
		c = o.content()
		require.True(t, c.deltaKnown)
		require.True(t, c.inputHistorical)
		require.Equal(t, returned, c.observed)
		require.False(t, c.observed.Cached)
		require.Equal(t, []api.WAFListItem{old}, c.removed)
		require.Equal(t, []api.WAFListItem{next}, c.added)
		o.result.Batches = append(o.result.Batches, setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{old}, Result: &api.WAFBatchResult{OK: false, Returned: nil, ReturnedItems: nil, PrerequisiteMutation: false, Status: api.WAFBatchUnconfirmed}})
		c = o.content()
		require.Nil(t, c.observed)
		require.Empty(t, c.issues)
		require.False(t, c.deltaKnown)
	}
}

func TestWAFContentIssuesAndTerminalRecovery(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{Comment: "", ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	for _, invalid := range []bool{false, true} {
		o := wafOutcome{result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, list: api.WAFList{AccountID: "", Name: ""}, cleanup: false, scope: wafScope{ipnet.IP4: true}}
		o.result.Before = &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{item}}
		result := &api.WAFBatchResult{OK: false, Returned: nil, ReturnedItems: nil, PrerequisiteMutation: false, Status: api.WAFBatchCompleted}
		want := wafReturnedContentsUnavailable
		if invalid {
			result.Returned = &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{item, item}}
			want = wafReturnedIdentityInvalid
		}
		o.result.Batches = []setter.WAFBatch{{Create: nil, Delete: []api.WAFListItem{item}, Result: result}}
		c := o.content()
		require.Nil(t, c.observed)
		require.Equal(t, []wafContentIssue{want}, c.issues)
		require.True(t, c.inputHistorical)
		o.result.Batches = append(o.result.Batches, setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{item}, Result: &api.WAFBatchResult{OK: false, Returned: nil, ReturnedItems: nil, PrerequisiteMutation: false, Status: api.WAFBatchUnconfirmed}})
		require.Equal(t, []wafContentIssue{want}, o.content().issues)
		o.result.Batches = append(o.result.Batches, setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{item}, Result: &api.WAFBatchResult{ReturnedItems: nil, PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Cached: false, Items: nil, Exists: true}}})
		c = o.content()
		require.NotNil(t, c.observed)
		require.Empty(t, c.observed.Items)
		require.Empty(t, c.issues)
		require.False(t, c.deltaKnown)
	}
}

func TestWAFExclusionsTrackIdentityTransitions(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "owned"}
	outside := old
	outside.Comment = "foreign"
	other := outside
	other.ID = "other"
	o := wafOutcome{result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, list: api.WAFList{AccountID: "", Name: ""}, cleanup: false, scope: wafScope{ipnet.IP4: true}}
	o.result.Before = &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{old}}
	response := func(selected, all []api.WAFListItem) setter.WAFBatch {
		return setter.WAFBatch{Delete: nil, Create: []api.WAFListCreateItem{{Comment: "", Prefix: old.Prefix}}, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Cached: false, Exists: true, Items: selected}, ReturnedItems: all}}
	}
	o.result.Batches = []setter.WAFBatch{response(nil, []api.WAFListItem{outside, other}), response(nil, []api.WAFListItem{outside, other})}
	c := o.content()
	require.False(t, c.deltaKnown)
	require.NotNil(t, c.observed)
	require.Empty(t, c.observed.Items)
	require.Equal(t, []wafExclusion{{batchIndex: 0, item: outside}}, c.exclusions)
	o.result.Batches = append(o.result.Batches, response([]api.WAFListItem{old}, []api.WAFListItem{old}), response(nil, []api.WAFListItem{outside}))
	c = o.content()
	require.Equal(t, []wafExclusion{{batchIndex: 0, item: outside}, {batchIndex: 3, item: outside}}, c.exclusions)
	o.result.Batches = append(o.result.Batches, setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{old}, Result: &api.WAFBatchResult{OK: false, Returned: nil, ReturnedItems: nil, PrerequisiteMutation: false, Status: api.WAFBatchUnconfirmed}})
	require.Nil(t, o.content().observed)
	require.Len(t, o.content().exclusions, 2)
}

func TestWAFReturnedIdentityMustAgreeAcrossViews(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{Comment: "", ID: "same", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	conflicting := item
	conflicting.Prefix = netip.MustParsePrefix("198.51.100.0/24")
	o := wafOutcome{result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, list: api.WAFList{AccountID: "", Name: ""}, cleanup: false, scope: wafScope{ipnet.IP4: true}}
	o.result.Before = &api.WAFSnapshot{Cached: false, Items: nil, Exists: true}
	o.result.Batches = []setter.WAFBatch{{Delete: nil, Create: []api.WAFListCreateItem{{Comment: "", Prefix: item.Prefix}}, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{item}}, ReturnedItems: []api.WAFListItem{conflicting}}}}
	c := o.content()
	require.Nil(t, c.observed)
	require.False(t, c.deltaKnown)
	require.Equal(t, []wafContentIssue{wafReturnedIdentityInvalid}, c.issues)
}

func TestWAFObservedUnknownMissingEmpty(t *testing.T) {
	t.Parallel()
	for _, input := range []*api.WAFSnapshot{nil, {Cached: false, Items: nil, Exists: false}, {Cached: false, Items: nil, Exists: true}, {Items: nil, Exists: true, Cached: true}} {
		o := wafOutcome{result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, list: api.WAFList{AccountID: "", Name: ""}, cleanup: false, scope: wafScope{ipnet.IP4: false}}
		o.result.Before = input
		require.Equal(t, input, o.content().observed)
	}
	item := api.WAFListItem{Comment: "", ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	o := wafOutcome{result: setter.WAFResult{Code: 0, Before: nil, Batches: nil, Cleanup: nil}, filtered: false, list: api.WAFList{AccountID: "", Name: ""}, cleanup: false, scope: wafScope{ipnet.IP4: true}}
	o.result.Cleanup = &api.WAFCleanupResult{Code: 0, Before: &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{item}}, Stages: []api.WAFCleanupStage{{Items: nil, WholeList: true, Result: api.WAFBatchResult{OK: false, Returned: nil, ReturnedItems: nil, PrerequisiteMutation: false, Status: api.WAFBatchUnconfirmed}}}}
	// Fallback input follows the failed whole-list operation.
	require.False(t, o.content().inputHistorical)
	require.NotNil(t, o.content().observed)
	o.result.Cleanup.Stages = append(o.result.Cleanup.Stages, api.WAFCleanupStage{WholeList: false, Items: []api.WAFListItem{item}, Result: api.WAFBatchResult{Returned: nil, ReturnedItems: nil, PrerequisiteMutation: false, OK: true, Status: api.WAFBatchAccepted}})
	require.True(t, o.content().inputHistorical)
	require.Nil(t, o.content().observed)
	require.False(t, o.content().deltaKnown)
}

func TestWAFExclusionsRespectConfiguredOwnership(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{ID: "six", Prefix: netip.MustParsePrefix("2001:db8::/32"), Comment: "owned"}
	outside := old
	outside.Comment = "foreign"
	for _, configured := range []bool{false, true} {
		var o wafOutcome
		o.scope = wafScope{ipnet.IP4: true}
		if configured {
			o.scope[ipnet.IP6] = false
		}
		o.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old}}
		o.result.Batches = []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}, ReturnedItems: []api.WAFListItem{outside}}}}
		if configured {
			require.Len(t, o.content().exclusions, 1)
		} else {
			require.Empty(t, o.content().exclusions)
		}
	}
}

// Membership guards cover configured ownership, including unavailable families,
// while omitted families cannot suppress a supported active-family delta.
func TestWAFDeltaMembershipUsesConfiguredOwnership(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "owned"}
	added := api.WAFListItem{ID: "added", Prefix: netip.MustParsePrefix("198.51.100.0/24"), Comment: "owned"}
	six := api.WAFListItem{ID: "six", Prefix: netip.MustParsePrefix("2001:db8::/32"), Comment: "owned"}
	outside := six
	outside.Comment = "foreign"
	for _, configured := range []bool{false, true} {
		for _, laterResponse := range []bool{false, true} {
			var o wafOutcome
			o.scope = wafScope{ipnet.IP4: true}
			if configured {
				o.scope[ipnet.IP6] = false
			}
			o.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old, six}}
			returned := []api.WAFListItem{old, added}
			full := []api.WAFListItem{old, added, outside}
			if laterResponse {
				returned = append(returned, six)
				full = []api.WAFListItem{old, added, six}
			}
			o.result.Batches = []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: added.Prefix, Comment: "owned"}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: returned}, ReturnedItems: full}}}
			wantCurrent := []api.WAFListItem{old, added}
			if laterResponse {
				o.result.Batches = append(o.result.Batches, setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{old}, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{added}}, ReturnedItems: []api.WAFListItem{added, outside}}})
				wantCurrent = []api.WAFListItem{added}
			}
			c := o.content()
			require.Equal(t, &api.WAFSnapshot{Exists: true, Cached: false, Items: wantCurrent}, c.observed)
			require.Equal(t, !configured, c.deltaKnown)
			require.Empty(t, c.issues)
			if configured {
				require.Empty(t, c.added)
				require.Empty(t, c.removed)
				require.Empty(t, c.unchanged)
				require.Len(t, c.exclusions, 1)
			} else {
				require.Empty(t, c.exclusions)
				require.Equal(t, []api.WAFListItem{added}, c.added)
				if laterResponse {
					require.Equal(t, []api.WAFListItem{old}, c.removed)
					require.Empty(t, c.unchanged)
				} else {
					require.Empty(t, c.removed)
					require.Equal(t, []api.WAFListItem{old}, c.unchanged)
				}
			}
		}
	}
}

// These are defensive evidence-contract cases, not claims that normal cleanup
// produces mismatched IDs. Equal prefix values and equal counts cannot prove
// that the admitted managed identities were all targeted or removed.
func TestWAFCleanupCompletenessRequiresAdmittedIdentities(t *testing.T) {
	t.Parallel()
	original := api.WAFListItem{ID: "original", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	for _, status := range []api.WAFBatchStatus{api.WAFBatchAccepted, api.WAFBatchCompleted} {
		for _, change := range []string{"identity", "metadata", "repeated target"} {
			t.Run(fmt.Sprintf("%d/%s", status, change), func(t *testing.T) {
				t.Parallel()
				target := original
				switch change {
				case "identity":
					target.ID = "different"
				case "metadata":
					target.Comment = "different"
				}
				targets := []api.WAFListItem{target}
				if change == "repeated target" {
					targets = append(targets, target)
				}
				var o wafOutcome
				o.scope = wafScope{ipnet.IP4: true}
				o.filtered, o.cleanup = true, true
				o.result.Code = setter.ResponseUpdating
				if status == api.WAFBatchCompleted {
					o.result.Code = setter.ResponseUpdated
				}
				before := &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{original}}
				o.result.Cleanup = &api.WAFCleanupResult{Before: before, Code: api.WAFListCleanupUpdating, Stages: []api.WAFCleanupStage{
					{WholeList: false, Items: targets, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: status, Returned: nil, ReturnedItems: nil}},
				}}
				if status == api.WAFBatchCompleted {
					o.result.Cleanup.Code = api.WAFListCleanupUpdated
				}
				content := o.content()
				require.False(t, content.deltaKnown)
				require.Empty(t, content.removed)
				text := renderWAF(o)
				require.NotContains(t, text, "all managed items")
				require.NotContains(t, text, "No managed items remain")
				require.Contains(t, text, "before cleanup")
				require.Contains(t, text, original.Prefix.String())
			})
		}
	}
}

// Work success and trustworthy inventory are separate evidence. The current
// parser reports nil inventory on failure, but WAFBatchResult does not promise
// that every usable observation implies OK. Preserve that boundary without
// interpreting an observed target as proof that all requested work succeeded.
func TestWAFObservedInventoryDoesNotImplyWorkSuccess(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "owned"}
	next := api.WAFListItem{ID: "next", Prefix: netip.MustParsePrefix("198.51.100.0/24"), Comment: "owned"}
	for _, ok := range []bool{false, true} {
		var o wafOutcome
		o.scope = wafScope{ipnet.IP4: true}
		o.result.Before = &api.WAFSnapshot{Exists: true, Cached: true, Items: []api.WAFListItem{old}}
		returned := &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old, next}}
		o.result.Batches = []setter.WAFBatch{{
			Create: []api.WAFListCreateItem{{Prefix: next.Prefix, Comment: next.Comment}},
			Delete: nil,
			Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: ok, Status: api.WAFBatchCompleted, Returned: returned, ReturnedItems: returned.Items},
		}}
		content := o.content()
		require.Equal(t, returned, content.observed)
		require.True(t, content.inputHistorical)
		require.Empty(t, content.issues)
		require.Equal(t, ok, content.deltaKnown)
		if ok {
			require.Empty(t, content.removed)
			require.Equal(t, []api.WAFListItem{old}, content.unchanged)
			require.Equal(t, []api.WAFListItem{next}, content.added)
		} else {
			require.Empty(t, content.removed)
			require.Empty(t, content.added)
			require.Empty(t, content.unchanged)
		}
	}
}
