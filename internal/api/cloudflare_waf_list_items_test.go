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
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

const listItemPageSize = 100

type listItem struct {
	ID      api.ID
	Prefix  string
	Comment string
}

func mockListItem(listItem listItem) cloudflare.ListItem {
	var ip *string
	if listItem.Prefix != "" {
		ip = &listItem.Prefix
	}

	id := listItem.ID
	if id == "" {
		id = mockID(listItem.Prefix, 0)
	}

	return cloudflare.ListItem{
		ID:         id.String(),
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

// checkListItemCreateRequestPayload validates the request body format shared by
// both create (POST) and replace (PUT) list-item APIs in cloudflare-go.
// The operation differs, but the payload is the same: []ListItemCreateRequest.
//
// This helper runs inside HTTP handlers; require is unsafe in HTTP handler
// goroutines.
func checkListItemCreateRequestPayload(t *testing.T, r *http.Request, expectedItems []netip.Prefix, expectedComment string) bool {
	t.Helper()

	var createRequests []cloudflare.ListItemCreateRequest
	if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&createRequests)) { //nolint:testifylint // require is unsafe in HTTP handler goroutines.
		return false
	}

	actualItems := make([]string, 0, len(createRequests))
	for _, item := range createRequests {
		if !assert.NotNil(t, item.IP) || !assert.Equal(t, expectedComment, item.Comment) {
			return false
		}
		actualItems = append(actualItems, *item.IP)
	}

	expectedRawItems := make([]string, 0, len(expectedItems))
	for _, item := range expectedItems {
		expectedRawItems = append(expectedRawItems, ipnet.DescribePrefixOrIP(item))
	}

	return assert.ElementsMatch(t, expectedRawItems, actualItems)
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
			[]listItem{{ID: "", Prefix: "10.0.0.1", Comment: ""}, {ID: "", Prefix: "2001:db8::/32", Comment: ""}, {ID: "", Prefix: "10.0.0.0/20", Comment: ""}},
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
			[]listItem{{ID: "", Prefix: "10.0.0.1", Comment: ""}},
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
			[]listItem{{ID: "", Prefix: "invalid item", Comment: ""}},
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
			[]listItem{{ID: "", Prefix: "", Comment: ""}},
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
			[]listItem{{ID: "item-with-comment", Prefix: "10.0.0.1", Comment: "hello"}},
			1,
			true, true,
			[]api.WAFListItem{
				{ID: "item-with-comment", Prefix: netip.MustParsePrefix("10.0.0.1/32")},
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

			f := newCloudflareHarness(t)
			lh := newListListsHandler(t, f.serveMux, tc.lists)
			clh := newCreateListHandler(t, f.serveMux,
				cloudflare.ListCreateRequest{
					Name:        mockWAFList.Name,
					Description: "description",
					Kind:        cloudflare.ListTypeIP,
				},
				tc.newList,
			)
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

	f := newCloudflareHarness(t)
	lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
	lih := newListListItemsHandler(t, f.serveMux, mockID("list", 0), []listItem{
		{ID: "", Prefix: "10.0.0.1", Comment: ""},
		{ID: "", Prefix: "2001:db8::/32", Comment: ""},
		{ID: "", Prefix: "10.0.0.0/20", Comment: ""},
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
			OperationID string `json:"operation_id"` //nolint:tagliatelle // Cloudflare uses snake_case field names.
		}{OperationID: string(id)},
		Response: mockResponse(),
	}
}

func newDeleteListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID api.ID, expectedIDs []api.ID) httpHandler {
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

			var deleteRequest cloudflare.ListItemDeleteRequest
			if err := json.NewDecoder(r.Body).Decode(&deleteRequest); !assert.NoError(t, err) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			actualIDs := make([]api.ID, 0, len(deleteRequest.Items))
			for _, item := range deleteRequest.Items {
				actualIDs = append(actualIDs, api.ID(item.ID))
			}

			if !assert.ElementsMatch(t, expectedIDs, actualIDs) {
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
			[]listItem{{ID: "", Prefix: "10.0.0.1/32", Comment: ""}, {ID: "", Prefix: "2001:db8::/32", Comment: ""}, {ID: "", Prefix: "10.0.0.0/20", Comment: ""}},
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
			[]listItem{{ID: "", Prefix: "10.0.0.1/32", Comment: ""}, {ID: "", Prefix: "2001:db8::/32", Comment: ""}, {ID: "", Prefix: "invalid item", Comment: ""}},
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

			f := newCloudflareHarness(t)
			lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			dih := newDeleteListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), tc.idsToDelete)
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
			OperationID string `json:"operation_id"` //nolint:tagliatelle // Cloudflare uses snake_case field names.
		}{OperationID: string(id)},
		Response: mockResponse(),
	}
}

func newReplaceListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID api.ID,
	expectedItems []netip.Prefix, expectedComment string,
) httpHandler {
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

			if !checkListItemCreateRequestPayload(t, r, expectedItems, expectedComment) {
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

func newCreateListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID api.ID,
	expectedItems []netip.Prefix, expectedComment string,
) httpHandler {
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

			if !checkListItemCreateRequestPayload(t, r, expectedItems, expectedComment) {
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
			[]listItem{{ID: "", Prefix: "10.0.0.1/32", Comment: ""}, {ID: "", Prefix: "2001:db8::/32", Comment: ""}, {ID: "", Prefix: "10.0.0.0/20", Comment: ""}},
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
			[]listItem{{ID: "", Prefix: "10.0.0.1/32", Comment: ""}, {ID: "", Prefix: "2001:db8::/32", Comment: ""}, {ID: "", Prefix: "invalid item", Comment: ""}},
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

			f := newCloudflareHarness(t)
			lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			cih := newCreateListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), tc.itemsToCreate, itemComment)
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
