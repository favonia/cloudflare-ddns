package api_test

import (
	"context"
	"net/netip"
	"regexp"
	"testing"
	"testing/synctest"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Returned membership is established by the response, not requested comments or
// prefixes. Result ownership must also survive reads exposing cache storage.
func TestWAFBatchResultOwnsReturnedMembership(t *testing.T) {
	t.Parallel()
	for _, create := range []bool{false, true} {
		name := "delete"
		if create {
			name = "create"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				options := defaultHandleOptions()
				options.ManagedWAFListItemsCommentRegex = regexp.MustCompile("^managed$")
				options.AllowWholeWAFListDeleteOnShutdown = false
				f := newCloudflareHarnessWithOptions(t, options)
				listID, operationID := mockID("list", 0), mockID("op", 0)
				lists := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 2, kind: cloudflare.ListTypeIP}})
				response := newListListItemsHandler(t, f.serveMux, listID, []listItem{
					{ID: "retained", Prefix: "192.0.2.0/24", Comment: "managed"},
					{ID: "outside", Prefix: "198.51.100.1/32", Comment: "foreign"},
				})
				requested := []api.WAFListCreateItem{{Prefix: netip.MustParsePrefix("198.51.100.1/32"), Comment: "managed"}}
				var mutation httpHandler
				if create {
					mutation = newCreateListItemsHandler(t, f.serveMux, listID, operationID, requested)
				} else {
					mutation = newDeleteListItemsHandler(t, f.serveMux, listID, operationID, []api.ID{"retained"})
				}
				lists.setRequestLimit(1)
				response.setRequestLimit(1)
				mutation.setRequestLimit(1)
				var result api.WAFBatchResult
				if create {
					result = f.cfHandle.CreateWAFListItems(context.Background(), pp.NewSilent(), mockWAFList, "description", requested)
				} else {
					result = f.cfHandle.DeleteWAFListItems(context.Background(), pp.NewSilent(), mockWAFList, "description", []api.ID{"retained"})
				}
				retained := api.WAFListItem{ID: "retained", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "managed"}
				outside := api.WAFListItem{ID: "outside", Prefix: netip.MustParsePrefix("198.51.100.1/32"), Comment: "foreign"}
				require.True(t, result.OK)
				require.Equal(t, api.WAFBatchCompleted, result.Status)
				require.Equal(t, &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{retained}}, result.Returned)
				require.Equal(t, []api.WAFListItem{retained, outside}, result.ReturnedItems)
				// The returned managed view must neither alias the cache nor full response.
				result.Returned.Items[0].Comment = "result-changed"
				require.Equal(t, retained, result.ReturnedItems[0])
				cached, exists, wasCached, ok := f.cfHandle.ListWAFListItems(context.Background(), pp.NewSilent(), mockWAFList, "description", "managed")
				require.True(t, ok)
				require.True(t, exists)
				require.True(t, wasCached)
				require.Equal(t, []api.WAFListItem{retained}, cached)
				cached[0].ID = "cache-changed"
				require.Equal(t, api.ID("retained"), result.Returned.Items[0].ID)
				require.Equal(t, retained, result.ReturnedItems[0])
				assertHandlersExhausted(t, lists, response, mutation)
			})
		})
	}
}

// Prerequisite evidence separates lookup failure from dispatched structural creation.
func TestWAFBatchPrerequisiteDispatch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		lookup, create, items bool
		want                  bool
		status                api.WAFBatchStatus
	}{
		{"lookup-failed", false, false, false, false, api.WAFBatchNoEffect},
		{"structural-unconfirmed", true, false, false, true, api.WAFBatchNoEffect},
		{"structural-created-items-unconfirmed", true, true, false, true, api.WAFBatchUnconfirmed},
		{"structural-and-items-completed", true, true, true, true, api.WAFBatchCompleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				f := newCloudflareHarness(t)
				lists := newListListsHandler(t, f.serveMux, nil)
				if tc.lookup {
					lists.setRequestLimit(1)
				}
				create := newCreateListHandler(t, f.serveMux, cloudflare.ListCreateRequest{Name: "list", Description: "description", Kind: cloudflare.ListTypeIP}, listMeta{name: "list", size: 0, kind: cloudflare.ListTypeIP})
				if tc.create {
					create.setRequestLimit(1)
				}
				requested := []api.WAFListCreateItem{{Comment: "", Prefix: netip.MustParsePrefix("192.0.2.1/32")}}
				mutation := newCreateListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), requested)
				if tc.items {
					mutation.setRequestLimit(1)
				}
				returned := newListListItemsHandler(t, f.serveMux, mockID("list", 0), nil)
				if tc.items {
					returned.setRequestLimit(1)
				}
				result := f.cfHandle.CreateWAFListItems(context.Background(), pp.NewSilent(), mockWAFList, "description", requested)
				require.Equal(t, tc.want, result.PrerequisiteMutation)
				require.Equal(t, tc.status, result.Status)
				assertHandlersExhausted(t, lists, create, mutation, returned)
			})
		})
	}
}
