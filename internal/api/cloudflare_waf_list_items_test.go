package api_test

// vim: nowrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

const listItemPageSize = 100

type listItem struct {
	IP      string
	Comment string
}

func mockListItem(listItem listItem) cloudflare.ListItem {
	var ip *string
	if listItem.IP != "" {
		ip = &listItem.IP
	}

	return cloudflare.ListItem{
		ID:         string(mockID(listItem.IP, 0)),
		IP:         ip,
		Redirect:   nil,
		Hostname:   nil,
		ASN:        nil,
		Comment:    listItem.Comment,
		CreatedOn:  nil,
		ModifiedOn: nil,
	}
}

func mockListListItemsResponse(listItems []listItem) cloudflare.ListItemsListResponse {
	// Pagination is intentionally delegated to cloudflare-go (ListListItems).
	// These tests mock a single page only to focus on this package's logic.
	if len(listItems) > listItemPageSize {
		panic("mockListItemsResponse got too many items")
	}

	items := make([]cloudflare.ListItem, 0, len(listItems))
	for _, meta := range listItems {
		items = append(items, mockListItem(meta))
	}

	return cloudflare.ListItemsListResponse{
		Result:     items,
		ResultInfo: mockResultInfo(len(listItems), listItemPageSize),
		Response:   mockResponse(),
	}
}

func newListListItemsHandler(t *testing.T, mux *http.ServeMux, listID api.ID, listItems []listItem) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists/%s/items", mockAccountID, listID),
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
			err := json.NewEncoder(w).Encode(mockListListItemsResponse(listItems))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestListWAFListItems(t *testing.T) {
	t.Parallel()

	emptyListMeta := listMeta{} //nolint:exhaustruct

	for name, tc := range map[string]struct {
		lists                 []listMeta
		listRequestLimit      int
		newList               listMeta
		createRequestLimit    int
		items                 []listItem
		listItemsRequestLimit int
		ok                    bool
		alreadyExisting       bool
		output                []api.WAFListItem
		prepareMocks          func(*mocks.MockPP)
	}{
		"existing": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{"10.0.0.1", ""}, {"2001:db8::/32", ""}, {"10.0.0.0/20", ""}},
			1,
			true, true,
			[]api.WAFListItem{
				{ID: (mockID("10.0.0.1", 0)), Prefix: netip.MustParsePrefix("10.0.0.1/32")},
				{ID: (mockID("2001:db8::/32", 0)), Prefix: netip.MustParsePrefix("2001:db8::/32")},
				{ID: (mockID("10.0.0.0/20", 0)), Prefix: netip.MustParsePrefix("10.0.0.0/20")},
			},
			nil,
		},
		"create": {
			[]listMeta{},
			1,
			listMeta{name: "list", size: 5, kind: cloudflare.ListTypeIP},
			1,
			nil,
			0,
			true, false, nil,
			nil,
		},
		"create-fail": {
			[]listMeta{},
			1,
			emptyListMeta,
			0,
			nil,
			0,
			false, false, nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to create the list %s: %v", "account456/list", gomock.Any())
			},
		},
		"list-fail": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			0,
			emptyListMeta,
			0, nil, 0,
			false, false, nil,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of the list %s", "account456/list"),
				)
			},
		},
		"list-item-fail": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{"10.0.0.1", ""}},
			0,
			false, false, nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to retrieve items in the list %s: %v", "account456/list", gomock.Any())
			},
		},
		"invalid": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{"invalid item", ""}},
			1,
			false, false, nil,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP range: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP address as well: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Found an invalid IP range/address %q in the list %s", "invalid item", "account456/list"),
				)
			},
		},
		"nil": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{"", ""}},
			1,
			false, false, nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
					"Found a non-IP in the list %s",
					"account456/list")
			},
		},
		"comment": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{"10.0.0.1", "hello"}},
			1,
			true, true,
			[]api.WAFListItem{
				{ID: (mockID("10.0.0.1", 0)), Prefix: netip.MustParsePrefix("10.0.0.1/32")},
			},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiWarning,
					"The IP range/address %q in the list %s has a non-empty comment %q. The comment might be lost during an IP update.",
					"10.0.0.1", "account456/list", "hello")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newCloudflareFixture(t)
			lh := newListListsHandler(t, f.serveMux, tc.lists)
			clh := newCreateListHandler(t, f.serveMux, tc.newList)
			lih := newListListItemsHandler(t, f.serveMux, mockID("list", 0), tc.items)

			lh.setRequestLimit(tc.listRequestLimit)
			clh.setRequestLimit(tc.createRequestLimit)
			lih.setRequestLimit(tc.listItemsRequestLimit)
			output, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(context.Background(), f.newPreparedPP(tc.prepareMocks), mockWAFList, "description")
			require.Equal(t, tc.ok, ok)
			require.False(t, cached)
			require.Equal(t, tc.alreadyExisting, alreadyExisting)
			require.Equal(t, tc.output, output)
			assertHandlersExhausted(t, lh, clh, lih)
		})
	}
}

func TestListWAFListItemsCache(t *testing.T) {
	t.Parallel()

	f := newCloudflareFixture(t)
	lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
	lih := newListListItemsHandler(t, f.serveMux, mockID("list", 0), []listItem{
		{"10.0.0.1", ""},
		{"2001:db8::/32", ""},
		{"10.0.0.0/20", ""},
	})

	lh.setRequestLimit(1)
	lih.setRequestLimit(1)
	output, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(context.Background(), f.newPP(), mockWAFList, "description")
	require.True(t, ok)
	require.False(t, cached)
	require.True(t, alreadyExisting)
	require.Equal(t, []api.WAFListItem{
		{ID: mockID("10.0.0.1", 0), Prefix: netip.MustParsePrefix("10.0.0.1/32")},
		{ID: mockID("2001:db8::/32", 0), Prefix: netip.MustParsePrefix("2001:db8::/32")},
		{ID: mockID("10.0.0.0/20", 0), Prefix: netip.MustParsePrefix("10.0.0.0/20")},
	}, output)
	assertHandlersExhausted(t, lh, lih)

	lh.setRequestLimit(0)
	lih.setRequestLimit(0)
	output, alreadyExisting, cached, ok = f.cfHandle.ListWAFListItems(context.Background(), f.newPP(), mockWAFList, "description")
	require.True(t, ok)
	require.True(t, cached)
	require.True(t, alreadyExisting)
	require.Equal(t, []api.WAFListItem{
		{ID: mockID("10.0.0.1", 0), Prefix: netip.MustParsePrefix("10.0.0.1/32")},
		{ID: mockID("2001:db8::/32", 0), Prefix: netip.MustParsePrefix("2001:db8::/32")},
		{ID: mockID("10.0.0.0/20", 0), Prefix: netip.MustParsePrefix("10.0.0.0/20")},
	}, output)
	assertHandlersExhausted(t, lh, lih)
}

func mockListBulkOperationResponse(id api.ID) cloudflare.ListBulkOperationResponse {
	t := time.Now()
	return cloudflare.ListBulkOperationResponse{
		Response: mockResponse(),
		Result: cloudflare.ListBulkOperation{
			ID:        string(id),
			Status:    "completed",
			Error:     "",
			Completed: &t,
		},
	}
}

func handleListBulkOperation(t *testing.T, operationID api.ID, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if !checkToken(t, r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !assert.Empty(t, r.URL.Query()) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockListBulkOperationResponse(operationID))
	assert.NoError(t, err)
}

func mockListItemDeleteResponse(id api.ID) cloudflare.ListItemDeleteResponse {
	return cloudflare.ListItemDeleteResponse{
		Result: struct {
			OperationID string `json:"operation_id"` //nolint:tagliatelle
		}{OperationID: string(id)},
		Response: mockResponse(),
	}
}

func newDeleteListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID api.ID) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("DELETE /accounts/%s/rules/lists/%s/items", mockAccountID, listID),
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
			err := json.NewEncoder(w).Encode(mockListItemDeleteResponse(operationID))
			assert.NoError(t, err)
		})

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists/bulk_operations/%s", mockAccountID, operationID),
		func(w http.ResponseWriter, r *http.Request) {
			handleListBulkOperation(t, operationID, w, r)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestDeleteWAFListItems(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		listRequestLimit      int
		idsToDelete           []api.ID
		deleteRequestLimit    int
		listItemsResponse     []listItem
		listItemsRequestLimit int
		ok                    bool
		prepareMocks          func(*mocks.MockPP)
	}{
		"success": {
			1,
			[]api.ID{"id1", "id2", "id3"},
			1,
			[]listItem{{"10.0.0.1/32", ""}, {"2001:db8::/32", ""}, {"10.0.0.0/20", ""}},
			1, true,
			nil,
		},
		"empty": {0, nil, 0, nil, 0, true, nil},
		"list-fail": {
			0,
			[]api.ID{"id1", "id2", "id3"},
			0, nil, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find the list %s", "account456/list"),
				)
			},
		},
		"delete-fail": {
			1,
			[]api.ID{"id1", "id2", "id3"},
			0, nil, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to finish deleting items from the list %s: %v", "account456/list", gomock.Any())
			},
		},
		"list-items-invalid": {
			1,
			[]api.ID{"id1", "id2", "id3"},
			1,
			[]listItem{{"10.0.0.1/32", ""}, {"2001:db8::/32", ""}, {"invalid item", ""}},
			1,
			false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP range: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP address as well: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Found an invalid IP range/address %q in the list %s", "invalid item", "account456/list"),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newCloudflareFixture(t)
			lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			dih := newDeleteListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0))
			lih := newListListItemsHandler(t, f.serveMux, mockID("list", 0), tc.listItemsResponse)

			lh.setRequestLimit(tc.listRequestLimit)
			dih.setRequestLimit(tc.deleteRequestLimit)
			lih.setRequestLimit(tc.listItemsRequestLimit)
			ok := f.cfHandle.DeleteWAFListItems(context.Background(), f.newPreparedPP(tc.prepareMocks), mockWAFList, "description", tc.idsToDelete)
			require.Equal(t, tc.ok, ok)
			assertHandlersExhausted(t, lh, dih, lih)

			if tc.ok {
				dih.setRequestLimit(tc.deleteRequestLimit)
				lih.setRequestLimit(tc.listItemsRequestLimit)
				ok = f.cfHandle.DeleteWAFListItems(context.Background(), f.newPP(), mockWAFList, "description", tc.idsToDelete)
				require.Equal(t, tc.ok, ok)
				assertHandlersExhausted(t, lh, dih, lih)
			}
		})
	}
}

func mockListItemCreateResponse(id api.ID) cloudflare.ListItemCreateResponse {
	return cloudflare.ListItemCreateResponse{
		Result: struct {
			OperationID string `json:"operation_id"` //nolint:tagliatelle
		}{OperationID: string(id)},
		Response: mockResponse(),
	}
}

func newReplaceListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID api.ID) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("PUT /accounts/%s/rules/lists/%s/items", mockAccountID, listID),
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
			err := json.NewEncoder(w).Encode(mockListItemCreateResponse(operationID))
			assert.NoError(t, err)
		})

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists/bulk_operations/%s", mockAccountID, operationID),
		func(w http.ResponseWriter, r *http.Request) {
			handleListBulkOperation(t, operationID, w, r)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func newCreateListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID api.ID) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("POST /accounts/%s/rules/lists/%s/items", mockAccountID, listID),
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
			err := json.NewEncoder(w).Encode(mockListItemCreateResponse(operationID))
			assert.NoError(t, err)
		})

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists/bulk_operations/%s", mockAccountID, operationID),
		func(w http.ResponseWriter, r *http.Request) {
			handleListBulkOperation(t, operationID, w, r)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestCreateWAFListItems(t *testing.T) {
	t.Parallel()

	itemComment := "item comment"

	for name, tc := range map[string]struct {
		listRequestLimit      int
		itemsToCreate         []netip.Prefix
		createRequestLimit    int
		listItemsResponse     []listItem
		listItemsRequestLimit int
		ok                    bool
		prepareMocks          func(*mocks.MockPP)
	}{
		"success": {
			1,
			[]netip.Prefix{netip.MustParsePrefix("10.0.0.1/16"), netip.MustParsePrefix("2001:db8::/50")},
			1,
			[]listItem{{"10.0.0.1/32", ""}, {"2001:db8::/32", ""}, {"10.0.0.0/20", ""}},
			1,
			true,
			nil,
		},
		"empty": {0, nil, 0, nil, 0, true, nil},
		"list-fail": {
			0,
			[]netip.Prefix{netip.MustParsePrefix("10.0.0.1/16"), netip.MustParsePrefix("2001:db8::/50")},
			0, nil, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find the list %s", "account456/list"),
				)
			},
		},
		"create-fail": {
			1,
			[]netip.Prefix{netip.MustParsePrefix("10.0.0.1/16"), netip.MustParsePrefix("2001:db8::/50")},
			0, nil, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to finish adding items to the list %s: %v", "account456/list", gomock.Any())
			},
		},
		"list-items-invalid": {
			1,
			[]netip.Prefix{netip.MustParsePrefix("10.0.0.1/16"), netip.MustParsePrefix("2001:db8::/50")},
			1,
			[]listItem{{"10.0.0.1/32", ""}, {"2001:db8::/32", ""}, {"invalid item", ""}},
			1,
			false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP range: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP address as well: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Found an invalid IP range/address %q in the list %s", "invalid item", "account456/list"),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newCloudflareFixture(t)
			lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			cih := newCreateListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0))
			lih := newListListItemsHandler(t, f.serveMux, mockID("list", 0), tc.listItemsResponse)

			lh.setRequestLimit(tc.listRequestLimit)
			cih.setRequestLimit(tc.createRequestLimit)
			lih.setRequestLimit(tc.listItemsRequestLimit)
			ok := f.cfHandle.CreateWAFListItems(context.Background(), f.newPreparedPP(tc.prepareMocks), mockWAFList, "description", tc.itemsToCreate, itemComment)
			require.Equal(t, tc.ok, ok)
			assertHandlersExhausted(t, lh, cih, lih)

			if tc.ok {
				cih.setRequestLimit(tc.createRequestLimit)
				lih.setRequestLimit(tc.listItemsRequestLimit)
				ok = f.cfHandle.CreateWAFListItems(context.Background(), f.newPP(), mockWAFList, "description", tc.itemsToCreate, itemComment)
				require.Equal(t, tc.ok, ok)
				assertHandlersExhausted(t, lh, cih, lih)
			}
		})
	}
}
