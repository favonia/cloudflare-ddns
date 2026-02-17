package api_test

// vim: nowrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func TestFinalClearWAFListAsync(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		listRequestLimit    int
		listID              api.ID
		deleteRequestLimit  int
		replaceRequestLimit int
		deleted             bool
		ok                  bool
		prepareMocks        func(*mocks.MockPP)
	}{
		"success": {
			1, mockID("list", 0), 1, 0,
			true, true,
			nil,
		},
		"list-fail": {
			0, mockID("list", 0), 0, 0,
			false, false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find the list %s", "account456/list"),
				)
			},
		},
		"delete-fail/clear": {
			1, mockID("list", 0), 0, 1,
			false, true,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to delete the list %s; clearing it instead: %v", "account456/list", gomock.Any())
			},
		},
		"delete-fail/clear-fail": {
			1, mockID("list", 0), 0, 0,
			false, false,
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

			f := newCloudflareFixture(t)
			lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			dh := newDeleteListHandler(t, f.serveMux, mockID("list", 0))
			rih := newReplaceListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0))

			lh.setRequestLimit(tc.listRequestLimit)
			dh.setRequestLimit(tc.deleteRequestLimit)
			rih.setRequestLimit(tc.replaceRequestLimit)
			deleted, ok := f.cfHandle.FinalClearWAFListAsync(context.Background(), f.newPreparedPP(tc.prepareMocks), mockWAFList, "description")
			require.Equal(t, tc.deleted, deleted)
			require.Equal(t, tc.ok, ok)
			assertHandlersExhausted(t, lh, dh, rih)
		})
	}
}
