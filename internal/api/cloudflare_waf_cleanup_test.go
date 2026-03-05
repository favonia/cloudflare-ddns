package api_test

// vim: nowrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func mockDeleteListResponse(listID api.ID) cloudflare.ListDeleteResponse {
	return cloudflare.ListDeleteResponse{
		Response: mockResponse(),
		Result: struct {
			ID string `json:"id"`
		}{ID: string(listID)},
	}
}

func newDeleteListHandler(t *testing.T, mux *http.ServeMux, listID api.ID) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("DELETE /accounts/%s/rules/lists/%s", mockAccountID, listID),
		func(w http.ResponseWriter, r *http.Request) {
			if !checkRequestLimit(t, &requestLimit) || !checkToken(t, r) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !assert.Empty(t, r.URL.Query()) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(mockDeleteListResponse(listID))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestFinalCleanWAFListWholeListOwnership(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		listRequestLimit    int
		listID              api.ID
		deleteRequestLimit  int
		replaceRequestLimit int
		code                api.WAFListCleanupCode
		prepareMocks        func(*mocks.MockPP)
	}{
		"success": {
			1, mockID("list", 0), 1, 0,
			api.WAFListCleanupUpdated,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiDeletion, "The list %s was deleted", "account456/list")
			},
		},
		"list-fail": {
			0, mockID("list", 0), 0, 0,
			api.WAFListCleanupFailed,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find the list %s", "account456/list"),
				)
			},
		},
		"delete-fail/clear": {
			1, mockID("list", 0), 0, 1,
			api.WAFListCleanupUpdating,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to delete the list %s; clearing it instead: %v", "account456/list", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiClear, "The list %s is being cleared (asynchronously)", "account456/list"),
				)
			},
		},
		"delete-fail/clear-fail": {
			1, mockID("list", 0), 0, 0,
			api.WAFListCleanupFailed,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to delete the list %s; clearing it instead: %v", "account456/list", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to start clearing the list %s: %v", "account456/list", gomock.Any()),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			options := defaultHandleOptions()
			options.DeleteWholeWAFListsOnShutdown = true
			f := newCloudflareHarnessWithOptions(t, options)

			lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			dh := newDeleteListHandler(t, f.serveMux, mockID("list", 0))
			rih := newReplaceListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), nil, "")

			lh.setRequestLimit(tc.listRequestLimit)
			dh.setRequestLimit(tc.deleteRequestLimit)
			rih.setRequestLimit(tc.replaceRequestLimit)
			code := f.cfHandle.FinalCleanWAFList(
				context.Background(),
				f.newPreparedPP(tc.prepareMocks),
				mockWAFList,
				"description",
			)
			require.Equal(t, tc.code, code)
			assertHandlersExhausted(t, lh, dh, rih)
		})
	}
}

func TestFinalCleanWAFListSharedOwnership(t *testing.T) {
	t.Parallel()

	options := defaultHandleOptions()
	options.ManagedWAFListItemsCommentRegex = regexp.MustCompile("^managed$")
	options.DeleteWholeWAFListsOnShutdown = false
	f := newCloudflareHarnessWithOptions(t, options)

	listItems := []listItem{
		{ID: "managed-v4", Prefix: "10.0.0.1/32", Comment: "managed"},
		{ID: "foreign-v4", Prefix: "10.0.0.2/32", Comment: "foreign"},
	}
	listHandler := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 2, kind: cloudflare.ListTypeIP}})
	itemsHandler := newListListItemsHandler(t, f.serveMux, mockID("list", 0), listItems)
	deleteHandler := newDeleteListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), []api.ID{"managed-v4"})

	listHandler.setRequestLimit(1)
	itemsHandler.setRequestLimit(2)
	deleteHandler.setRequestLimit(1)
	code := f.cfHandle.FinalCleanWAFList(
		context.Background(),
		f.newPreparedPP(func(ppfmt *mocks.MockPP) {
			ppfmt.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "10.0.0.1", "account456/list")
		}),
		mockWAFList,
		"description",
	)
	require.Equal(t, api.WAFListCleanupUpdated, code)
	assertHandlersExhausted(t, listHandler, itemsHandler, deleteHandler)
}
