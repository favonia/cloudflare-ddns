package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// A parsed selected view must be contained in the full returned view. Nil full
// items represent an empty list, not a legacy permission to omit membership
// evidence. Otherwise an omitted formerly owned ID could mean exclusion rather
// than removal, making complete-delta claims unsafe.
func TestWAFReturnedViewsRequireCompleteMembershipEvidence(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{ID: "owned", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	result := api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item}}, ReturnedItems: nil}
	require.False(t, trustworthyWAFResponse(result))
	result.ReturnedItems = []api.WAFListItem{item}
	require.True(t, trustworthyWAFResponse(result))
	result.Returned.Items = nil
	result.ReturnedItems = nil
	require.True(t, trustworthyWAFResponse(result), "nil full items validly represent a parsed empty list")
}

// A structurally incomplete response must not turn a missing owned identity
// into a confirmed removal, even if the work Boolean reports success.
func TestWAFIncompleteReturnedViewCannotEstablishDelta(t *testing.T) {
	t.Parallel()
	old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	next := api.WAFListItem{ID: "new", Prefix: netip.MustParsePrefix("192.0.2.2/32"), Comment: "owned"}
	var o wafOutcome
	o.scope = wafScope{ipnet.IP4: true}
	o.result.Code = setter.ResponseUpdated
	o.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old}}
	o.result.Batches = []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: next.Prefix, Comment: next.Comment}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, Status: api.WAFBatchCompleted, OK: true, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{next}}, ReturnedItems: nil}}}
	content := o.content()
	require.False(t, content.deltaKnown)
	require.Empty(t, content.removed)
	require.Empty(t, content.added)
	require.True(t, content.inputHistorical)
	require.Equal(t, []wafContentIssue{wafReturnedIdentityInvalid}, content.issues)
}
