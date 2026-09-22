package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Completion must not be reported merely because an item-deletion request was
// accepted. Exercise the real adapter against a local API fixture.
func TestFinalCleanWAFListWaitsForItems(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		options := defaultHandleOptions()
		options.AllowWholeWAFListDeleteOnShutdown = false
		f := newCloudflareHarnessWithOptions(t, options)
		listID, operationID := mockID("list", 0), mockID("op", 0)
		lists := newListListsHandler(t, f.serveMux,
			[]listMeta{{name: "list", size: 1, kind: cloudflare.ListTypeIP}})
		items := newListListItemsHandler(t, f.serveMux, listID, []listItem{
			{ID: "v4", Prefix: "192.0.2.1/32", Comment: "managed"},
		})
		deletion := newDeleteListItemsHandler(t, f.serveMux, listID, operationID, []api.ID{"v4"})
		lists.setRequestLimit(1)
		items.setRequestLimit(1)
		deletion.setRequestLimit(1)

		result := f.cfHandle.FinalCleanWAFList(context.Background(), pp.NewSilent(),
			mockWAFList, "description", cleanupFamilies(ipnet.IP4, ipnet.IP6), api.CleanupWait)

		require.Equal(t, api.WAFListCleanupUpdated, result)
		assertHandlersExhausted(t, lists, items, deletion)
	})
}

func TestFinalCleanWAFListCompletionOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		status                string
		wholeList             bool
		cancelAfterAcceptance bool
		mode                  api.CleanupMode
		want                  api.WAFListCleanupCode
	}{
		{"fallback-completed", "completed", true, false, api.CleanupWait, api.WAFListCleanupUpdated},
		{"fallback-failed", "failed", true, false, api.CleanupWait, api.WAFListCleanupFailed},
		{"items-failed", "failed", false, false, api.CleanupWait, api.WAFListCleanupFailed},
		{"items-timeout", "pending", false, false, api.CleanupWait, api.WAFListCleanupFailed},
		{"items-canceled", "pending", false, true, api.CleanupWait, api.WAFListCleanupFailed},
		{"items-accepted", "pending", false, false, api.CleanupAllowAsync, api.WAFListCleanupUpdating},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				options := defaultHandleOptions()
				options.AllowWholeWAFListDeleteOnShutdown = tc.wholeList
				f := newCloudflareHarnessWithOptions(t, options)
				listID, operationID := mockID("list", 0), mockID("op", 0)
				lists := newListListsHandler(t, f.serveMux,
					[]listMeta{{name: "list", size: 2, kind: cloudflare.ListTypeIP}})
				items := newListListItemsHandler(t, f.serveMux, listID, []listItem{
					{ID: "v4", Prefix: "192.0.2.1/32", Comment: "managed"},
					{ID: "v6", Prefix: "2001:db8::1/128", Comment: "managed"},
				})
				families := cleanupFamilies(ipnet.IP4)
				expectedIDs := []api.ID{"v4"}
				if tc.wholeList {
					families = cleanupFamilies(ipnet.IP4, ipnet.IP6)
					expectedIDs = append(expectedIDs, "v6")
					f.serveMux.HandleFunc(fmt.Sprintf("DELETE /accounts/%s/rules/lists/%s", mockAccountID, listID),
						func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				deletion := newDeleteListItemsRequestHandler(t, f.serveMux, listID, operationID, expectedIDs)
				polls := 0
				f.serveMux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists/bulk_operations/%s", mockAccountID, operationID),
					func(w http.ResponseWriter, _ *http.Request) {
						polls++
						response := mockListBulkOperationResponse(operationID)
						response.Result.Status = tc.status
						response.Result.Error = "fixture failure"
						assert.NoError(t, json.NewEncoder(w).Encode(response))
						if tc.cancelAfterAcceptance {
							cancel()
						}
					})
				lists.setRequestLimit(1)
				items.setRequestLimit(1)
				deletion.setRequestLimit(1)
				result := f.cfHandle.FinalCleanWAFList(ctx, pp.NewSilent(), mockWAFList, "description", families, tc.mode)
				require.Equal(t, tc.want, result)
				if tc.mode == api.CleanupAllowAsync {
					require.Zero(t, polls)
				} else {
					require.Positive(t, polls)
				}
				assertHandlersExhausted(t, lists, items, deletion)
			})
		})
	}
}
