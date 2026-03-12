package api_test

// vim: nowrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
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
	ID      ID
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

func newListListItemsHandler(t *testing.T, mux *http.ServeMux, listID ID, listItems []listItem) httpHandler {
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

func newListListItemsHandlerSequence(t *testing.T, mux *http.ServeMux, listID ID, sequence [][]listItem) httpHandler {
	t.Helper()

	var requestLimit int
	next := 0

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
			if !assert.Less(t, next, len(sequence)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(mockListListItemsResponse(sequence[next]))
			assert.NoError(t, err)
			next++
		})

	return httpHandler{requestLimit: &requestLimit}
}

// checkListItemCreateRequestPayload validates the request body format shared by
// both create (POST) and replace (PUT) list-item APIs in cloudflare-go.
// The operation differs, but the payload is the same: []ListItemCreateRequest.
//
// This helper runs inside HTTP handlers; require is unsafe in HTTP handler
// goroutines.
func checkListItemCreateRequestPayload(t *testing.T, r *http.Request, expectedItems []api.WAFListCreateItem) bool {
	t.Helper()

	var createRequests []cloudflare.ListItemCreateRequest
	if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&createRequests)) { //nolint:testifylint // require is unsafe in HTTP handler goroutines.
		return false
	}

	actualItems := make([]api.WAFListCreateItem, 0, len(createRequests))
	for _, item := range createRequests {
		if !assert.NotNil(t, item.IP) {
			return false
		}
		rawPrefix := *item.IP
		prefix, err := netip.ParsePrefix(rawPrefix)
		if err != nil {
			if strings.Contains(rawPrefix, "/") {
				assert.NoError(t, err)
				return false
			}
			addr, addrErr := netip.ParseAddr(rawPrefix)
			require.NoError(t, addrErr)
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		actualItems = append(actualItems, api.WAFListCreateItem{
			Prefix:  prefix.Masked(),
			Comment: item.Comment,
		})
	}

	expectedNormalizedItems := make([]api.WAFListCreateItem, 0, len(expectedItems))
	for _, item := range expectedItems {
		expectedNormalizedItems = append(expectedNormalizedItems, api.WAFListCreateItem{
			Prefix:  item.Prefix.Masked(),
			Comment: item.Comment,
		})
	}

	return assert.ElementsMatch(t, expectedNormalizedItems, actualItems)
}

func TestListWAFListItems(t *testing.T) {
	t.Parallel()

	emptyListMeta := listMeta{} //nolint:exhaustruct

	for name, tc := range map[string]struct {
		managedWAFListItemsCommentRegex *regexp.Regexp
		lists                           []listMeta
		listRequestLimit                int
		newList                         listMeta
		createRequestLimit              int
		items                           []listItem
		listItemsRequestLimit           int
		ok                              bool
		alreadyExisting                 bool
		output                          []api.WAFListItem
		expectedItemComment             string
		prepareMocks                    func(*mocks.MockPP)
	}{
		"existing": {
			nil,
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{ID: "", Prefix: "10.0.0.1", Comment: ""}, {ID: "", Prefix: "2001:db8::/32", Comment: ""}, {ID: "", Prefix: "10.0.0.0/20", Comment: ""}},
			1,
			true, true,
			[]api.WAFListItem{
				{ID: (mockID("10.0.0.1", 0)), Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: ""},
				{ID: (mockID("2001:db8::/32", 0)), Prefix: netip.MustParsePrefix("2001:db8::/32"), Comment: ""},
				{ID: (mockID("10.0.0.0/20", 0)), Prefix: netip.MustParsePrefix("10.0.0.0/20"), Comment: ""},
			},
			"",
			nil,
		},
		"create": {
			nil,
			[]listMeta{},
			1,
			listMeta{name: "list", size: 5, kind: cloudflare.ListTypeIP},
			1,
			nil,
			0,
			true, false, nil,
			"",
			nil,
		},
		"create-fail": {
			nil,
			[]listMeta{},
			1,
			emptyListMeta,
			0,
			nil,
			0,
			false, false, nil,
			"",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm creation of list %s: %v", "account456/list", gomock.Any())
			},
		},
		"list-fail": {
			nil,
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			0,
			emptyListMeta,
			0, nil, 0,
			false, false, nil,
			"",
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of the list %s", "account456/list"),
				)
			},
		},
		"list-item-fail": {
			nil,
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{ID: "", Prefix: "10.0.0.1", Comment: ""}},
			0,
			false, false, nil,
			"",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to retrieve items in the list %s: %v", "account456/list", gomock.Any())
			},
		},
		"invalid": {
			nil,
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{ID: "", Prefix: "invalid item", Comment: ""}},
			1,
			false, false, nil,
			"",
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP range: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse %q as an IP address as well: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Found an invalid IP range/address %q in the list %s", "invalid item", "account456/list"),
				)
			},
		},
		"nil": {
			nil,
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{ID: "", Prefix: "", Comment: ""}},
			1,
			false, false, nil,
			"",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
					"Found a non-IP in the list %s",
					"account456/list")
			},
		},
		"comment": {
			nil,
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{{ID: "", Prefix: "10.0.0.1", Comment: "hello"}},
			1,
			true, true,
			[]api.WAFListItem{
				{ID: (mockID("10.0.0.1", 0)), Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: "hello"},
			},
			"hello",
			nil,
		},
		"comment-mismatch-warning": {
			nil,
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{
				{ID: "item-1", Prefix: "10.0.0.1", Comment: "current-1"},
				{ID: "item-2", Prefix: "2001:db8::/32", Comment: "current-2"},
			},
			1,
			true, true,
			[]api.WAFListItem{
				{ID: "item-1", Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: "current-1"},
				{ID: "item-2", Prefix: netip.MustParsePrefix("2001:db8::/32"), Comment: "current-2"},
			},
			"expected",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiUserWarning,
					"The comment for item ID %s in list %s is %s. However, the preferred comment for WAF list items is %s. Found %d managed WAF list item(s) with mismatched comments. These mismatches are reported but not corrected.",
					api.ID("item-1"),
					"account456/list",
					`"current-1"`,
					`"expected"`,
					2,
				)
			},
		},
		"managed-item-filter": {
			regexp.MustCompile("^managed$"),
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			1,
			emptyListMeta,
			0,
			[]listItem{
				{ID: "managed-v4", Prefix: "10.0.0.1", Comment: "managed"},
				{ID: "foreign-v4", Prefix: "10.0.0.2", Comment: "foreign"},
				{ID: "managed-v6", Prefix: "2001:db8::/32", Comment: "managed"},
			},
			1,
			true, true,
			[]api.WAFListItem{
				{ID: "managed-v4", Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: "managed"},
				{ID: "managed-v6", Prefix: netip.MustParsePrefix("2001:db8::/32"), Comment: "managed"},
			},
			"managed",
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			options := defaultHandleOptions()
			options.ManagedWAFListItemsCommentRegex = tc.managedWAFListItemsCommentRegex
			options.AllowWholeWAFListDeleteOnShutdown = tc.managedWAFListItemsCommentRegex == nil ||
				tc.managedWAFListItemsCommentRegex.String() == ""
			f := newCloudflareHarnessWithOptions(t, options)
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
			output, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(
				context.Background(),
				f.newPreparedPP(tc.prepareMocks),
				mockWAFList,
				"description",
				tc.expectedItemComment,
			)
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
	output, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(context.Background(), f.newPP(), mockWAFList, "description", "")
	require.True(t, ok)
	require.False(t, cached)
	require.True(t, alreadyExisting)
	require.Equal(t, []api.WAFListItem{
		{ID: mockID("10.0.0.1", 0), Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: ""},
		{ID: mockID("2001:db8::/32", 0), Prefix: netip.MustParsePrefix("2001:db8::/32"), Comment: ""},
		{ID: mockID("10.0.0.0/20", 0), Prefix: netip.MustParsePrefix("10.0.0.0/20"), Comment: ""},
	}, output)
	assertHandlersExhausted(t, lh, lih)

	lh.setRequestLimit(0)
	lih.setRequestLimit(0)
	output, alreadyExisting, cached, ok = f.cfHandle.ListWAFListItems(context.Background(), f.newPP(), mockWAFList, "description", "")
	require.True(t, ok)
	require.True(t, cached)
	require.True(t, alreadyExisting)
	require.Equal(t, []api.WAFListItem{
		{ID: mockID("10.0.0.1", 0), Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: ""},
		{ID: mockID("2001:db8::/32", 0), Prefix: netip.MustParsePrefix("2001:db8::/32"), Comment: ""},
		{ID: mockID("10.0.0.0/20", 0), Prefix: netip.MustParsePrefix("10.0.0.0/20"), Comment: ""},
	}, output)
	assertHandlersExhausted(t, lh, lih)
}

func TestListWAFListItemsCommentMismatchWarningCacheMissOnly(t *testing.T) {
	t.Parallel()

	f := newCloudflareHarness(t)
	lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 1, kind: cloudflare.ListTypeIP}})
	lih := newListListItemsHandler(t, f.serveMux, mockID("list", 0), []listItem{
		{ID: "item-1", Prefix: "10.0.0.1", Comment: "current"},
	})

	lh.setRequestLimit(1)
	lih.setRequestLimit(1)
	firstPP := f.newPP()
	firstPP.EXPECT().Noticef(
		pp.EmojiUserWarning,
		"The comment for item ID %s in list %s is %s. However, the preferred comment for WAF list items is %s. Found %d managed WAF list item(s) with mismatched comments. These mismatches are reported but not corrected.",
		api.ID("item-1"),
		"account456/list",
		`"current"`,
		`"expected"`,
		1,
	)
	output, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(
		context.Background(), firstPP, mockWAFList, "description", "expected")
	require.True(t, ok)
	require.False(t, cached)
	require.True(t, alreadyExisting)
	require.Equal(t, []api.WAFListItem{
		{ID: "item-1", Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: "current"},
	}, output)
	assertHandlersExhausted(t, lh, lih)

	lh.setRequestLimit(0)
	lih.setRequestLimit(0)
	output, alreadyExisting, cached, ok = f.cfHandle.ListWAFListItems(
		context.Background(), f.newPP(), mockWAFList, "description", "expected")
	require.True(t, ok)
	require.True(t, cached)
	require.True(t, alreadyExisting)
	require.Equal(t, []api.WAFListItem{
		{ID: "item-1", Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: "current"},
	}, output)
	assertHandlersExhausted(t, lh, lih)
}

func mockListBulkOperationResponse(id ID) cloudflare.ListBulkOperationResponse {
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

func handleListBulkOperation(t *testing.T, operationID ID, w http.ResponseWriter, r *http.Request) {
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

func mockListItemDeleteResponse(id ID) cloudflare.ListItemDeleteResponse {
	return cloudflare.ListItemDeleteResponse{
		Result: struct {
			OperationID string `json:"operation_id"` //nolint:tagliatelle // Cloudflare uses snake_case field names.
		}{OperationID: string(id)},
		Response: mockResponse(),
	}
}

func newDeleteListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID ID, expectedIDs []api.ID) httpHandler {
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
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm deletion of items from list %s: %v", "account456/list", gomock.Any())
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
			ok := f.cfHandle.DeleteWAFListItems(
				context.Background(),
				f.newPreparedPP(tc.prepareMocks),
				mockWAFList,
				"description",
				tc.idsToDelete,
			)
			require.Equal(t, tc.ok, ok)
			assertHandlersExhausted(t, lh, dih, lih)

			if tc.ok {
				dih.setRequestLimit(tc.deleteRequestLimit)
				lih.setRequestLimit(tc.listItemsRequestLimit)
				ok = f.cfHandle.DeleteWAFListItems(
					context.Background(),
					f.newPP(),
					mockWAFList,
					"description",
					tc.idsToDelete,
				)
				require.Equal(t, tc.ok, ok)
				assertHandlersExhausted(t, lh, dih, lih)
			}
		})
	}
}

func mockListItemCreateResponse(id ID) cloudflare.ListItemCreateResponse {
	return cloudflare.ListItemCreateResponse{
		Result: struct {
			OperationID string `json:"operation_id"` //nolint:tagliatelle // Cloudflare uses snake_case field names.
		}{OperationID: string(id)},
		Response: mockResponse(),
	}
}

func newCreateListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID ID,
	expectedItems []api.WAFListCreateItem,
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

			if !checkListItemCreateRequestPayload(t, r, expectedItems) {
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

	for name, tc := range map[string]struct {
		listRequestLimit      int
		itemsToCreate         []api.WAFListCreateItem
		createRequestLimit    int
		listItemsResponse     []listItem
		listItemsRequestLimit int
		ok                    bool
		prepareMocks          func(*mocks.MockPP)
	}{
		"success": {
			1,
			[]api.WAFListCreateItem{
				{Prefix: netip.MustParsePrefix("10.0.0.1/16"), Comment: "item comment"},
				{Prefix: netip.MustParsePrefix("2001:db8::/50"), Comment: "item comment"},
			},
			1,
			[]listItem{{ID: "", Prefix: "10.0.0.1/32", Comment: ""}, {ID: "", Prefix: "2001:db8::/32", Comment: ""}, {ID: "", Prefix: "10.0.0.0/20", Comment: ""}},
			1,
			true,
			nil,
		},
		"empty": {0, nil, 0, nil, 0, true, nil},
		"list-fail": {
			0,
			[]api.WAFListCreateItem{
				{Prefix: netip.MustParsePrefix("10.0.0.1/16"), Comment: "item comment"},
				{Prefix: netip.MustParsePrefix("2001:db8::/50"), Comment: "item comment"},
			},
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
			[]api.WAFListCreateItem{
				{Prefix: netip.MustParsePrefix("10.0.0.1/16"), Comment: "item comment"},
				{Prefix: netip.MustParsePrefix("2001:db8::/50"), Comment: "item comment"},
			},
			0, nil, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm addition of items to list %s: %v", "account456/list", gomock.Any())
			},
		},
		"list-items-invalid": {
			1,
			[]api.WAFListCreateItem{
				{Prefix: netip.MustParsePrefix("10.0.0.1/16"), Comment: "item comment"},
				{Prefix: netip.MustParsePrefix("2001:db8::/50"), Comment: "item comment"},
			},
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
			cih := newCreateListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), tc.itemsToCreate)
			lih := newListListItemsHandler(t, f.serveMux, mockID("list", 0), tc.listItemsResponse)

			lh.setRequestLimit(tc.listRequestLimit)
			cih.setRequestLimit(tc.createRequestLimit)
			lih.setRequestLimit(tc.listItemsRequestLimit)
			ok := f.cfHandle.CreateWAFListItems(context.Background(), f.newPreparedPP(tc.prepareMocks), mockWAFList, "description", tc.itemsToCreate)
			require.Equal(t, tc.ok, ok)
			assertHandlersExhausted(t, lh, cih, lih)

			if tc.ok {
				cih.setRequestLimit(tc.createRequestLimit)
				lih.setRequestLimit(tc.listItemsRequestLimit)
				ok = f.cfHandle.CreateWAFListItems(context.Background(), f.newPP(), mockWAFList, "description", tc.itemsToCreate)
				require.Equal(t, tc.ok, ok)
				assertHandlersExhausted(t, lh, cih, lih)
			}
		})
	}
}

func TestCreateWAFListItemsUnexpectedCommentAfterMutation(t *testing.T) {
	t.Parallel()

	const expectedComment = "expected"
	itemsToCreate := []api.WAFListCreateItem{{Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: expectedComment}}

	f := newCloudflareHarness(t)
	lh := newListListsHandler(t, f.serveMux, nil)
	clh := newCreateListHandler(t, f.serveMux,
		cloudflare.ListCreateRequest{
			Name:        mockWAFList.Name,
			Description: "description",
			Kind:        cloudflare.ListTypeIP,
		},
		listMeta{name: "list", size: 0, kind: cloudflare.ListTypeIP},
	)
	cih := newCreateListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), itemsToCreate)
	lih := newListListItemsHandlerSequence(t, f.serveMux, mockID("list", 0), [][]listItem{
		{{ID: "new-item", Prefix: "10.0.0.1/32", Comment: "unexpected"}},
	})

	lh.setRequestLimit(1)
	clh.setRequestLimit(1)
	cih.setRequestLimit(1)
	lih.setRequestLimit(1)

	managedItems, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(
		context.Background(), f.newPP(), mockWAFList, "description", expectedComment)
	require.True(t, ok)
	require.False(t, alreadyExisting)
	require.False(t, cached)
	require.Empty(t, managedItems)

	ppfmt := f.newPP()
	ppfmt.EXPECT().Noticef(
		pp.EmojiUserWarning,
		"After updating list %s, item ID %s has comment %s, which is unexpected given allowed post-mutation comments (%s) and pre-update cache state. Found %d managed WAF list item(s) with this anomaly.",
		"account456/list",
		api.ID("new-item"),
		`"unexpected"`,
		`"expected"`,
		1,
	)

	ok = f.cfHandle.CreateWAFListItems(context.Background(), ppfmt, mockWAFList, "description", itemsToCreate)
	require.True(t, ok)
	assertHandlersExhausted(t, lh, clh, cih, lih)
}

func TestDeleteWAFListItemsUnexpectedCommentAfterMutation(t *testing.T) {
	t.Parallel()

	f := newCloudflareHarness(t)
	lh := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 1, kind: cloudflare.ListTypeIP}})
	dih := newDeleteListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), []api.ID{"id1"})
	lih := newListListItemsHandlerSequence(t, f.serveMux, mockID("list", 0), [][]listItem{
		{{ID: "managed-1", Prefix: "10.0.0.1/32", Comment: "current"}},
		{{ID: "managed-1", Prefix: "10.0.0.1/32", Comment: "unexpected"}},
	})

	lh.setRequestLimit(1)
	dih.setRequestLimit(1)
	lih.setRequestLimit(2)

	managedItems, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(
		context.Background(), f.newPP(), mockWAFList, "description", "current")
	require.True(t, ok)
	require.True(t, alreadyExisting)
	require.False(t, cached)
	require.Equal(t, []api.WAFListItem{
		{ID: "managed-1", Prefix: netip.MustParsePrefix("10.0.0.1/32"), Comment: "current"},
	}, managedItems)

	ppfmt := f.newPP()
	ppfmt.EXPECT().Noticef(
		pp.EmojiUserWarning,
		"After updating list %s, item ID %s has comment %s, which is unexpected given allowed post-mutation comments (%s) and pre-update cache state. Found %d managed WAF list item(s) with this anomaly.",
		"account456/list",
		api.ID("managed-1"),
		`"unexpected"`,
		"none",
		1,
	)

	ok = f.cfHandle.DeleteWAFListItems(
		context.Background(),
		ppfmt,
		mockWAFList,
		"description",
		[]api.ID{"id1"},
	)
	require.True(t, ok)
	assertHandlersExhausted(t, lh, dih, lih)
}
