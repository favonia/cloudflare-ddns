package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// Empty inventory must not collapse managed-but-unselected ownership into an
// unmanaged family: successful IPv4 work still needs the empty IPv6 observation.
func TestWAFScopeRetainsEmptyUnselectedFamily(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{ID: "new", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	for _, managed := range []bool{false, true} {
		var o wafOutcome
		o.scope = wafScope{ipnet.IP4: true}
		o.filtered = !managed
		if managed {
			o.scope[ipnet.IP6] = false
		}
		o.result.Code = setter.ResponseUpdated
		o.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}
		o.result.Batches = []setter.WAFBatch{{
			Create: []api.WAFListCreateItem{{Prefix: item.Prefix, Comment: item.Comment}}, Delete: nil,
			Result: &api.WAFBatchResult{
				Status: api.WAFBatchCompleted, OK: true, PrerequisiteMutation: false,
				Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item}}, ReturnedItems: []api.WAFListItem{item},
			},
		}}
		content := o.content()
		require.True(t, content.deltaKnown)
		require.Equal(t, []api.WAFListItem{item}, content.added)
		text := renderWAF(o)
		require.Contains(t, text, "Added: "+item.Prefix.String())
		if managed {
			require.Contains(t, text, "Current managed IPv6 items: none")
			require.Contains(t, text, "IPv4 item")
		} else {
			require.NotContains(t, text, "IPv6")
			require.NotContains(t, text, "IPv4 item")
		}
	}
}

// The renderer accepts partial selection even though shutdown currently selects
// every managed family. Neither completion nor acceptance may claim all managed
// contents were removed when another managed family was not selected.
func TestWAFScopePartialCleanupPreservesUnselectedInventory(t *testing.T) {
	t.Parallel()
	four := api.WAFListItem{ID: "four", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	six := api.WAFListItem{ID: "six", Prefix: netip.MustParsePrefix("2001:db8::/64"), Comment: "owned"}
	for _, status := range []api.WAFBatchStatus{api.WAFBatchCompleted, api.WAFBatchAccepted} {
		var o wafOutcome
		o.scope = wafScope{ipnet.IP4: true, ipnet.IP6: false}
		o.cleanup, o.filtered = true, true
		o.result.Code = setter.ResponseUpdated
		cleanupCode := api.WAFListCleanupUpdated
		if status == api.WAFBatchAccepted {
			o.result.Code = setter.ResponseUpdating
			cleanupCode = api.WAFListCleanupUpdating
		}
		before := &api.WAFSnapshot{Exists: true, Cached: true, Items: []api.WAFListItem{four, six}}
		o.result.Before = before
		o.result.Cleanup = &api.WAFCleanupResult{Code: cleanupCode, Before: before, Stages: []api.WAFCleanupStage{{
			WholeList: false, Items: []api.WAFListItem{four}, Result: api.WAFBatchResult{Status: status, OK: true, PrerequisiteMutation: false, Returned: nil, ReturnedItems: nil},
		}}}
		content := o.content()
		require.Equal(t, status == api.WAFBatchCompleted, content.deltaKnown)
		require.False(t, o.acceptedFullCleanup(o.configuredSnapshot(before)))
		text := renderWAF(o)
		require.Contains(t, text, six.Prefix.String())
		require.Contains(t, text, "before cleanup (cached)")
		require.NotContains(t, text, "No managed items remain")
		require.NotContains(t, text, "all managed items")
		if status == api.WAFBatchCompleted {
			require.Equal(t, []api.WAFListItem{four}, content.removed)
			require.Contains(t, text, "No managed IPv4 items remain")
		} else {
			require.Empty(t, content.removed)
			require.Contains(t, text, "Cleanup started")
		}
	}
}
