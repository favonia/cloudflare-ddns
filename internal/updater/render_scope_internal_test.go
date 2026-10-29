package updater

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// These direct renderer checks assert scope, chronology and claim eligibility;
// complete message wording awaits maintainer review of integrated captures.
func TestRenderManagedScopeAndCompleteness(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("host.example", setter.ResponseUpdated, reportingSnapshot(""))
	o.filtered = true
	o.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordCreate, setter.DNSOperationCompleted, "new", "192.0.2.2")}
	o.result.Progress.Operations[0].Membership = api.MembershipManaged
	text := renderDNS(makeDNSRow(assessedDNS{outcome: o, firstKnown: true, unexplained: false}))
	require.NotContains(t, strings.Split(text, "\n")[0], "filtered")
	require.Contains(t, text, "only managed address")
	o.result.Code = setter.ResponseNoop
	text = renderDNS(makeDNSRow(assessedDNS{outcome: o, firstKnown: true, unexplained: false}))
	require.Contains(t, text, "Current managed addresses: none")
	o.result.Before = nil
	o.result.Code = setter.ResponseFailed
	o.result.Progress = nil
	text = renderDNS(makeDNSRow(assessedDNS{outcome: o, firstKnown: false, unexplained: false}))
	require.Contains(t, text, "Could not start the update to 192.0.2.2")
	require.NotContains(t, text, "Target:")
}

func TestRenderWAFObservationAndCause(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	added := api.WAFListItem{ID: "new", Prefix: netip.MustParsePrefix("192.0.2.2/32"), Comment: "owned"}
	var o wafOutcome
	o.filtered = true
	o.configured = map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
	o.active = map[ipnet.Family]bool{ipnet.IP4: true}
	o.code = setter.ResponseUpdated
	o.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}
	o.result.Batches = []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: added.Prefix, Comment: "owned"}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{added}}, ReturnedItems: []api.WAFListItem{added}}}}
	text := renderWAF(o)
	require.Contains(t, text, "only managed IPv4 item")
	require.Contains(t, text, "Current managed IPv6 items: none")
	o.result.Before.Items = []api.WAFListItem{old}
	o.result.Batches[0].Result.ReturnedItems = append(o.result.Batches[0].Result.ReturnedItems, api.WAFListItem{ID: old.ID, Prefix: old.Prefix, Comment: "excluded"})
	text = renderWAF(o)
	require.Contains(t, text, "comment unexpectedly changed and no longer matches")
	require.Contains(t, text, "Current managed items: 192.0.2.2/32")
	require.NotContains(t, text, "could not be confirmed")
	require.NotContains(t, text, "only managed")
	o.result.Batches[0].Result.Returned = nil
	text = renderWAF(o)
	require.Contains(t, text, "Could not read the returned list contents")
	require.Contains(t, text, "before the update")
	require.NotContains(t, text, "Current")
	o.result.Batches[0].Result.Status = api.WAFBatchUnconfirmed
	text = renderWAF(o)
	require.NotContains(t, text, "returned list contents")
	require.NotContains(t, text, "could not be confirmed")
	o.result.Batches[0].Result.Status = api.WAFBatchNoEffect
	o.result.Batches[0].Result.PrerequisiteMutation = true
	text = renderWAF(o)
	require.Contains(t, text, "List creation unconfirmed")
	require.Contains(t, text, "Addition blocked")
	require.NotContains(t, text, "Current")
}

func TestRenderWAFIdentityCardinality(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{ID: "one", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: ""}
	duplicate := item
	duplicate.ID = "two"
	var o wafOutcome
	o.filtered = true
	o.configured = map[ipnet.Family]bool{ipnet.IP4: true}
	o.active = map[ipnet.Family]bool{ipnet.IP4: true}
	o.code = setter.ResponseUpdated
	o.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}
	o.result.Batches = []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: item.Prefix, Comment: ""}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item, duplicate}}, ReturnedItems: []api.WAFListItem{item, duplicate}}}}
	text := renderWAF(o)
	require.NotContains(t, text, "only managed item")
	require.Contains(t, text, "(2 items)")
}

func TestRenderWAFAcceptedCleanupScope(t *testing.T) {
	t.Parallel()
	first := api.WAFListItem{ID: "one", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	second := api.WAFListItem{ID: "two", Prefix: netip.MustParsePrefix("192.0.2.2/32"), Comment: "owned"}
	for _, cached := range []bool{false, true} {
		var o wafOutcome
		o.filtered = true
		o.configured = map[ipnet.Family]bool{ipnet.IP4: true}
		o.active = map[ipnet.Family]bool{ipnet.IP4: true}
		o.cleanup = true
		o.code = setter.ResponseUpdating
		o.result.Cleanup = &api.WAFCleanupResult{Code: api.WAFListCleanupUpdating, Before: &api.WAFSnapshot{Exists: true, Cached: cached, Items: []api.WAFListItem{first, second}}, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{first}, Result: api.WAFBatchResult{PrerequisiteMutation: false, Status: api.WAFBatchAccepted, OK: true, Returned: nil, ReturnedItems: nil}}}}
		text := renderWAF(o)
		require.NotContains(t, text, "all managed")
		require.Contains(t, text, "before cleanup")
		require.Contains(t, text, second.Prefix.String())
		require.NotContains(t, text, "Current")
		o.result.Cleanup.Stages[0].Items = append(o.result.Cleanup.Stages[0].Items, second)
		text = renderWAF(o)
		require.Contains(t, text, "all")
		require.Equal(t, 1, strings.Count(text, first.Prefix.String()))
		require.NotContains(t, text, "before cleanup")
		require.NotContains(t, text, "remain")
		if cached {
			require.Contains(t, text, "all the managed items (cached)")
		}
	}
}

func TestRenderWAFBlockedInputAndResponseOrdering(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{ID: "one", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	var o wafOutcome
	o.filtered = true
	o.configured = map[ipnet.Family]bool{ipnet.IP4: true}
	o.active = map[ipnet.Family]bool{ipnet.IP4: true}
	o.code = setter.ResponseFailed
	o.result.Before = &api.WAFSnapshot{Exists: true, Cached: true, Items: []api.WAFListItem{old}}
	o.result.Batches = []setter.WAFBatch{
		{Create: []api.WAFListCreateItem{{Prefix: netip.MustParsePrefix("192.0.2.2/32"), Comment: "owned"}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchNoEffect, Returned: nil, ReturnedItems: nil}},
		{Create: nil, Delete: []api.WAFListItem{old}, Result: nil},
	}
	text := renderWAF(o)
	require.Contains(t, text, "Current managed items (cached)")
	require.NotContains(t, text, "before the update")
	require.NotContains(t, text, "List creation")
	o.result.Batches[0].Result.Status = api.WAFBatchCompleted
	text = renderWAF(o)
	require.Less(t, strings.Index(text, "Addition completed"), strings.Index(text, "returned list contents"))
	require.Less(t, strings.Index(text, "returned list contents"), strings.Index(text, "Removal not attempted"))
	require.Contains(t, text, "Managed items before the update (cached)")
	require.NotContains(t, text, "Current")
	o.result.Batches[0].Result.Returned = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old, old}}
	text = renderWAF(o)
	require.Contains(t, text, "identified reliably")
	require.NotContains(t, text, "returned list contents")
	require.NotContains(t, text, "Current")
}

func TestRenderWAFEmptyInventoryUsesItems(t *testing.T) {
	t.Parallel()
	for _, filtered := range []bool{false, true} {
		for _, cached := range []bool{false, true} {
			var o wafOutcome
			o.filtered = filtered
			o.configured = map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
			o.active = map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
			o.code = setter.ResponseNoop
			o.result.Before = &api.WAFSnapshot{Exists: true, Cached: cached, Items: nil}
			text := renderWAF(o)
			require.NotContains(t, text, "prefix")
			if filtered {
				require.Contains(t, text, "managed items")
				require.Contains(t, text, ": none")
			} else {
				require.Contains(t, text, ": no items")
			}
			require.Equal(t, cached, strings.Contains(text, "(cached)"))
			require.Contains(t, text, "Current")
			require.NotContains(t, text, "before")
		}
	}
}

// An unselected response item with a requested prefix leaves creation identity
// unresolved. Keep the completed operation and trustworthy inventory visible.
func TestRenderWAFPrefixOnlyReturnRetainsCompletedAddition(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	foreign := api.WAFListItem{ID: "unrelated", Prefix: netip.MustParsePrefix("192.0.2.2/32"), Comment: "foreign"}
	for _, tc := range []struct {
		name   string
		before []api.WAFListItem
	}{
		{"empty-input", nil}, {"nonempty-input", []api.WAFListItem{old}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var o wafOutcome
			o.filtered = true
			o.configured = map[ipnet.Family]bool{ipnet.IP4: true}
			o.active = map[ipnet.Family]bool{ipnet.IP4: true}
			o.code = setter.ResponseUpdated
			o.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: tc.before}
			full := append([]api.WAFListItem{foreign}, tc.before...)
			o.result.Batches = []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: foreign.Prefix, Comment: "owned"}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: tc.before}, ReturnedItems: full}}}
			text := renderWAF(o)
			require.Contains(t, text, "Addition completed: "+foreign.Prefix.String())
			require.Contains(t, text, "Current managed items")
			if len(tc.before) == 0 {
				require.Contains(t, text, ": none")
			} else {
				require.Contains(t, text, "Current managed items: "+old.Prefix.String())
			}
			require.NotContains(t, text, "excluded")
			require.NotContains(t, text, "comment unexpectedly changed")
			require.NotContains(t, text, "could not be confirmed")
			require.NotContains(t, text, "unknown")
			require.NotContains(t, text, "before the update")
			require.Less(t, strings.Index(text, "Addition completed"), strings.Index(text, "Current managed items"))
		})
	}
}
