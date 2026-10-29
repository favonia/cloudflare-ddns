package setter_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// A failed create cannot erase planned deletions or claim per-item progress.
// A later delete cannot overwrite the admitted input or create's returned facts.
func TestWAFResultPreservesBatchBoundariesAndOwnedEvidence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		create   api.WAFBatchResult
		deleteOK bool
		want     setter.ResponseCode
	}{
		{"blocked-create", api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchNoEffect}, false, setter.ResponseFailed},
		{"unconfirmed-create", api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchUnconfirmed}, false, setter.ResponseFailed},
		{"invalid-create-response", api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Returned: nil, ReturnedItems: nil, Status: api.WAFBatchCompleted}, false, setter.ResponseFailed},
		{"create-then-failed-delete", api.WAFBatchResult{PrerequisiteMutation: false, Returned: nil, ReturnedItems: nil, OK: true, Status: api.WAFBatchCompleted}, false, setter.ResponseFailed},
		{"completed-batches", api.WAFBatchResult{PrerequisiteMutation: false, Returned: nil, ReturnedItems: nil, OK: true, Status: api.WAFBatchCompleted}, true, setter.ResponseUpdated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, h := newSetterHarness(t)
			list := api.WAFList{AccountID: "account", Name: "list"}
			old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "managed"}
			unavailable := api.WAFListItem{ID: "v6", Prefix: netip.MustParsePrefix("2001:db8::/64"), Comment: "managed"}
			input := []api.WAFListItem{old, unavailable}
			target := netip.MustParsePrefix("198.51.100.1/32")
			createItems := []api.WAFListCreateItem{{Prefix: target, Comment: "managed"}}
			returnedItem := api.WAFListItem{ID: "new", Prefix: target, Comment: "outside-selector"}
			returnedItems := []api.WAFListItem{returnedItem}
			selected := []api.WAFListItem{old, unavailable}
			create := tc.create
			create.PrerequisiteMutation = true
			if create.OK {
				create.Returned = &api.WAFSnapshot{Cached: false, Exists: true, Items: selected}
				create.ReturnedItems = returnedItems
			}
			p := pp.NewSilent()
			s := setter.New(p, h.mockHandle)
			readCall := h.mockHandle.EXPECT().ListWAFListItems(ctx, p, list, "description", "managed").Return(input, true, true, true)
			createCall := h.mockHandle.EXPECT().CreateWAFListItems(ctx, p, list, "description", createItems).
				DoAndReturn(func(context.Context, pp.PP, api.WAFList, string, []api.WAFListCreateItem) api.WAFBatchResult {
					// Model a dependency mutating its cached backing array during the call.
					input[0].ID = "cache-changed"
					return create
				}).After(readCall.Call)
			if create.OK {
				h.mockHandle.EXPECT().DeleteWAFListItems(ctx, p, list, "description", []api.ID{"old"}).
					DoAndReturn(func(context.Context, pp.PP, api.WAFList, string, []api.ID) api.WAFBatchResult {
						selected[0].ID = "response-changed"
						returnedItems[0].Comment = "response-changed"
						return wafBatchResultForTest(tc.deleteOK)
					}).After(createCall)
			}
			result := s.SetWAFList(ctx, p, list, "description", map[ipnet.Family]setter.WAFTargets{
				ipnet.IP4: setter.NewAvailableWAFTargets([]netip.Prefix{target}),
				ipnet.IP6: setter.NewUnavailableWAFTargets(),
			}, "managed")
			require.Equal(t, tc.want, result.Code)
			require.Equal(t, &api.WAFSnapshot{Exists: true, Cached: true, Items: []api.WAFListItem{old, unavailable}}, result.Before)
			require.Len(t, result.Batches, 2)
			require.Equal(t, createItems, result.Batches[0].Create)
			require.Empty(t, result.Batches[0].Delete)
			require.Equal(t, []api.WAFListItem{old}, result.Batches[1].Delete)
			require.Empty(t, result.Batches[1].Create)
			require.NotNil(t, result.Batches[0].Result)
			require.Equal(t, tc.create.Status, result.Batches[0].Result.Status)
			require.True(t, result.Batches[0].Result.PrerequisiteMutation)
			require.Equal(t, tc.create.OK, result.Batches[0].Result.OK)
			if create.OK {
				require.NotNil(t, result.Batches[1].Result)
				require.Equal(t, tc.deleteOK, result.Batches[1].Result.OK)
				require.Equal(t, []api.WAFListItem{old, unavailable}, result.Batches[0].Result.Returned.Items)
				require.Equal(t, []api.WAFListItem{returnedItem}, result.Batches[0].Result.ReturnedItems)
			} else {
				require.Nil(t, result.Batches[1].Result)
			}
		})
	}
}

func TestWAFResultReadKnowledge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		exists, cached, ok bool
	}{
		{"missing", false, false, true},
		{"fresh-selected-empty", true, false, true},
		{"cached-selected-empty", true, true, true},
		{"unknown", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, h := newSetterHarness(t)
			p := pp.NewSilent()
			s := setter.New(p, h.mockHandle)
			list := api.WAFList{AccountID: "account", Name: "list"}
			h.mockHandle.EXPECT().ListWAFListItems(ctx, p, list, "", "").Return(nil, tc.exists, tc.cached, tc.ok)
			result := s.SetWAFList(ctx, p, list, "", map[ipnet.Family]setter.WAFTargets{
				ipnet.IP4: setter.NewAvailableWAFTargets(nil),
			}, "")
			require.Empty(t, result.Batches)
			if tc.ok {
				require.Equal(t, setter.ResponseNoop, result.Code)
				require.Equal(t, &api.WAFSnapshot{Items: nil, Exists: tc.exists, Cached: tc.cached}, result.Before)
			} else {
				require.Equal(t, setter.ResponseFailed, result.Code)
				require.Nil(t, result.Before)
			}
		})
	}
}

func TestFinalWAFResultOwnsCleanupEvidence(t *testing.T) {
	t.Parallel()
	ctx, h := newSetterHarness(t)
	p := pp.NewSilent()
	s := setter.New(p, h.mockHandle)
	list := api.WAFList{AccountID: "account", Name: "list"}
	item := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "managed"}
	input := []api.WAFListItem{item}
	cleanup := api.WAFCleanupResult{
		Code:   api.WAFListCleanupUpdating,
		Before: &api.WAFSnapshot{Exists: true, Cached: true, Items: input},
		Stages: []api.WAFCleanupStage{{WholeList: false, Items: input, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchAccepted, Returned: &api.WAFSnapshot{Cached: false, Exists: true, Items: input}, ReturnedItems: input}}},
	}
	h.mockHandle.EXPECT().FinalCleanWAFList(ctx, p, list, "", gomock.Any(), api.CleanupAllowAsync).Return(cleanup)
	result := s.FinalClearWAFList(ctx, p, list, "", map[ipnet.Family]bool{ipnet.IP4: true}, api.CleanupAllowAsync)
	input[0].ID = "changed"
	cleanup.Stages[0].Result.Status = api.WAFBatchUnconfirmed
	require.Equal(t, setter.ResponseUpdating, result.Code)
	require.Equal(t, item, result.Before.Items[0])
	require.Equal(t, item, result.Cleanup.Before.Items[0])
	require.Equal(t, item, result.Cleanup.Stages[0].Items[0])
	require.Equal(t, item, result.Cleanup.Stages[0].Result.Returned.Items[0])
	require.Equal(t, item, result.Cleanup.Stages[0].Result.ReturnedItems[0])
	require.Equal(t, api.WAFBatchAccepted, result.Cleanup.Stages[0].Result.Status)
	require.Empty(t, result.Batches)
}

// Only required deletions belong to an unvisited plan. The existing empty
// delete call is retained as evidence only when execution actually reaches it.
func TestWAFResultEmptyDeleteBatch(t *testing.T) {
	t.Parallel()
	for _, createOK := range []bool{false, true} {
		name := "failed-create"
		if createOK {
			name = "completed-create"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, h := newSetterHarness(t)
			p := pp.NewSilent()
			s := setter.New(p, h.mockHandle)
			list := api.WAFList{AccountID: "account", Name: "list"}
			target := netip.MustParsePrefix("192.0.2.1/32")
			read := h.mockHandle.EXPECT().ListWAFListItems(ctx, p, list, "", "").Return(nil, false, false, true)
			create := h.mockHandle.EXPECT().CreateWAFListItems(ctx, p, list, "", []api.WAFListCreateItem{{Prefix: target, Comment: ""}}).
				Return(wafBatchResultForTest(createOK)).After(read.Call)
			if createOK {
				h.mockHandle.EXPECT().DeleteWAFListItems(ctx, p, list, "", []api.ID{}).
					Return(api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchNoEffect, Returned: nil, ReturnedItems: nil}).After(create)
			}
			result := s.SetWAFList(ctx, p, list, "", map[ipnet.Family]setter.WAFTargets{ipnet.IP4: setter.NewAvailableWAFTargets([]netip.Prefix{target})}, "")
			require.NotNil(t, result.Before)
			require.False(t, result.Before.Exists)
			if createOK {
				require.Equal(t, setter.ResponseUpdated, result.Code)
				require.Len(t, result.Batches, 2)
				require.NotNil(t, result.Batches[1].Result)
				require.Equal(t, api.WAFBatchNoEffect, result.Batches[1].Result.Status)
				require.True(t, result.Batches[1].Result.OK)
				require.Empty(t, result.Batches[1].Create)
				require.Empty(t, result.Batches[1].Delete)
			} else {
				require.Equal(t, setter.ResponseFailed, result.Code)
				require.Len(t, result.Batches, 1)
			}
		})
	}
}
