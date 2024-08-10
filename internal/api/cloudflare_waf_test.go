package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
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

type listMeta struct {
	name string
	size int
	kind string
}

func mockList(meta listMeta, i int) cloudflare.List {
	return cloudflare.List{
		ID:                    mockID(meta.name, i),
		Name:                  meta.name,
		Description:           fmt.Sprintf("%s (%s) of size %d", meta.name, meta.kind, meta.size),
		Kind:                  meta.kind,
		NumItems:              meta.size,
		NumReferencingFilters: 1,
		CreatedOn:             nil,
		ModifiedOn:            nil,
	}
}

func mockListsResponse(listMetas []listMeta) cloudflare.ListListResponse {
	numLists := len(listMetas)

	lists := make([]cloudflare.List, numLists)
	for i, meta := range listMetas {
		lists[i] = mockList(meta, i)
	}

	return cloudflare.ListListResponse{
		Result:   lists,
		Response: mockResponse(),
	}
}

func handleListLists(t *testing.T, listMetas []listMeta, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
		!assert.Equal(t, url.Values{}, r.URL.Query()) {
		panic(http.ErrAbortHandler)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockListsResponse(listMetas))
	assert.NoError(t, err)
}

type listListsHandler = httpHandler[[]listMeta]

func newListListsHandler(t *testing.T, mux *http.ServeMux) listListsHandler {
	t.Helper()

	var (
		listMetas    []listMeta
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists", mockAccountID),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

			handleListLists(t, listMetas, w, r)
		})

	return listListsHandler{
		mux:          mux,
		params:       &listMetas,
		requestLimit: &requestLimit,
	}
}

func TestListWAFLists(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lists  []listMeta
		ok     bool
		output []string
	}{
		"empty": {
			[]listMeta{},
			true,
			nil,
		},
		"2ip1asn": {
			[]listMeta{
				{name: "list", size: 10, kind: cloudflare.ListTypeIP},
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			true,
			mockIDs("list", 0, 2),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newGoodHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux)

			lh.set(tc.lists, 1)
			mockPP = mocks.NewMockPP(mockCtrl)
			lists, ok := h.(api.CloudflareHandle).ListWAFLists(context.Background(), mockPP, "list")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, lists)
			require.True(t, lh.isExhausted())

			lh.set(nil, 0)
			mockPP = mocks.NewMockPP(mockCtrl)
			lists, ok = h.(api.CloudflareHandle).ListWAFLists(context.Background(), mockPP, "list")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, lists)
			require.True(t, lh.isExhausted())

			h.(api.CloudflareHandle).FlushCache() //nolint:forcetypeassert

			mockPP = mocks.NewMockPP(mockCtrl)
			mockPP.EXPECT().Warningf(
				pp.EmojiError,
				"Failed to list existing lists: %v",
				gomock.Any(),
			)
			lists, ok = h.(api.CloudflareHandle).ListWAFLists(context.Background(), mockPP, "list")
			require.False(t, ok)
			require.Nil(t, lists)
			require.True(t, lh.isExhausted())
		})
	}
}

func TestFindWAFList(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lists            []listMeta
		listRequestLimit int
		ok               bool
		output           string
		prepareMocks     func(*mocks.MockPP)
	}{
		"list-fail": {
			nil,
			0,
			false,
			"",
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to list existing lists: %v",
						gomock.Any(),
					),
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to find the list %q",
						"list",
					),
				)
			},
		},
		"empty": {
			[]listMeta{},
			1,
			false,
			"",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Warningf(pp.EmojiError,
					"Failed to find the list %q",
					"list",
				)
			},
		},
		"1ip1asn": {
			[]listMeta{
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			1,
			true,
			mockID("list", 1),
			nil,
		},
		"2ip1asn": {
			[]listMeta{
				{name: "list", size: 10, kind: cloudflare.ListTypeIP},
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			1,
			false,
			"",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
					"Found multiple lists named %q; please report this at https://github.com/favonia/cloudflare-ddns/issues/new",
					"list",
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newGoodHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux)

			lh.set(tc.lists, tc.listRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			list, ok := h.(api.CloudflareHandle).FindWAFList(context.Background(), mockPP, "list")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, list)
			require.True(t, lh.isExhausted())

			lh.set(nil, 0)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			list, ok = h.(api.CloudflareHandle).FindWAFList(context.Background(), mockPP, "list")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, list)
			require.True(t, lh.isExhausted())
		})
	}
}

func mockListResponse(meta listMeta) cloudflare.ListResponse {
	return cloudflare.ListResponse{
		Result:   mockList(meta, 0),
		Response: mockResponse(),
	}
}

func handleCreateList(t *testing.T, meta listMeta, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
		!assert.Equal(t, url.Values{}, r.URL.Query()) {
		panic(http.ErrAbortHandler)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockListResponse(meta))
	assert.NoError(t, err)
}

type createListHandler = httpHandler[listMeta]

func newCreateListHandler(t *testing.T, mux *http.ServeMux) createListHandler {
	t.Helper()

	var (
		listMeta     listMeta
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("POST /accounts/%s/rules/lists", mockAccountID),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

			handleCreateList(t, listMeta, w, r)
		})

	return createListHandler{
		mux:          mux,
		params:       &listMeta,
		requestLimit: &requestLimit,
	}
}

func TestEnsureWAFList(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lists              []listMeta
		listRequestLimit   int
		list               listMeta
		createRequestLimit int
		ok                 bool
		output             bool
		prepareMocks       func(*mocks.MockPP)
	}{
		"list-fail": {
			nil,
			0,
			listMeta{}, //nolint:exhaustruct
			0,
			false,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Warningf(pp.EmojiError,
					"Failed to list existing lists: %v",
					gomock.Any(),
				)
			},
		},
		"empty": {
			[]listMeta{},
			1,
			listMeta{name: "list", size: 13, kind: cloudflare.ListTypeIP},
			1,
			true,
			false,
			nil,
		},
		"empty/create-fail": {
			[]listMeta{},
			1,
			listMeta{}, //nolint:exhaustruct
			0,
			false,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Warningf(pp.EmojiError,
					"Failed to create a list named %q: %v",
					"list", gomock.Any(),
				)
			},
		},
		"1ip1asn": {
			[]listMeta{
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			1,
			listMeta{}, //nolint:exhaustruct
			0,
			true,
			true,
			nil,
		},
		"2ip1asn": {
			[]listMeta{
				{name: "list", size: 10, kind: cloudflare.ListTypeIP},
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			1,
			listMeta{}, //nolint:exhaustruct
			0,
			true,
			true,
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newGoodHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux)
			ch := newCreateListHandler(t, mux)

			lh.set(tc.lists, tc.listRequestLimit)
			ch.set(tc.list, tc.createRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			output, ok := h.(api.CloudflareHandle).EnsureWAFList(context.Background(), mockPP, "list", "description")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
			require.True(t, lh.isExhausted())
			require.True(t, ch.isExhausted())

			if tc.ok {
				lh.set(nil, 0)
				ch.set(listMeta{}, 0) //nolint:exhaustruct
				mockPP = mocks.NewMockPP(mockCtrl)
				output, ok = h.(api.CloudflareHandle).EnsureWAFList(context.Background(), mockPP, "list", "description")
				require.Equal(t, tc.ok, ok)
				require.True(t, output)
				require.True(t, lh.isExhausted())
				require.True(t, ch.isExhausted())
			}
		})
	}
}

func mockDeleteListResponse(listID string) cloudflare.ListDeleteResponse {
	return cloudflare.ListDeleteResponse{
		Response: mockResponse(),
		Result: struct {
			ID string `json:"id"`
		}{ID: listID},
	}
}

func handleDeleteList(t *testing.T, listID string, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
		!assert.Equal(t, url.Values{}, r.URL.Query()) {
		panic(http.ErrAbortHandler)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockDeleteListResponse(listID))
	assert.NoError(t, err)
}

type deleteListHandler = httpHandler[struct{}]

func newDeleteListHandler(t *testing.T, mux *http.ServeMux, listID string) deleteListHandler {
	t.Helper()

	var (
		dummy        struct{}
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("DELETE /accounts/%s/rules/lists/%s", mockAccountID, listID),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

			handleDeleteList(t, listID, w, r)
		})

	return deleteListItemsHandler{
		mux:          mux,
		params:       &dummy,
		requestLimit: &requestLimit,
	}
}

func TestDeleteWAFList(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		listRequestLimit   int
		listID             string
		deleteRequestLimit int
		ok                 bool
		prepareMocks       func(*mocks.MockPP)
	}{
		"success": {
			1,
			mockID("list", 0),
			1,
			true,
			nil,
		},
		"list-fail": {
			0,
			mockID("list", 0),
			0,
			false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to list existing lists: %v",
						gomock.Any(),
					),
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to find the list %q",
						"list",
					),
				)
			},
		},
		"delete-fail": {
			1,
			mockID("list", 0),
			0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Warningf(pp.EmojiError, "Failed to delete the list %q: %v", "list", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newGoodHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux)
			dh := newDeleteListHandler(t, mux, mockID("list", 0))

			lh.set([]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}}, tc.listRequestLimit)
			dh.set(struct{}{}, tc.deleteRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			//nolint:forcetypeassert
			ok = h.(api.CloudflareHandle).DeleteWAFList(context.Background(), mockPP, "list")
			require.Equal(t, tc.ok, ok)
			require.True(t, lh.isExhausted())
			require.True(t, dh.isExhausted())
		})
	}
}

type listItem = string

func mockListItem(listItem listItem) cloudflare.ListItem {
	return cloudflare.ListItem{
		ID:         mockID(listItem, 0),
		IP:         &listItem,
		Redirect:   nil,
		Hostname:   nil,
		ASN:        nil,
		Comment:    "",
		CreatedOn:  nil,
		ModifiedOn: nil,
	}
}

func mockListListItemsResponse(listItems []listItem) cloudflare.ListItemsListResponse {
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

func handleListListItems(t *testing.T, metas []listItem, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
		!assert.Equal(t, url.Values{}, r.URL.Query()) {
		panic(http.ErrAbortHandler)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockListListItemsResponse(metas))
	assert.NoError(t, err)
}

type listListItemsHandler = httpHandler[[]listItem]

func newListListItemsHandler(t *testing.T, mux *http.ServeMux, listID string) listListItemsHandler {
	t.Helper()

	var (
		listItems    []listItem
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists/%s/items", mockAccountID, listID),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

			handleListListItems(t, listItems, w, r)
		})

	return listListItemsHandler{
		mux:          mux,
		params:       &listItems,
		requestLimit: &requestLimit,
	}
}

func TestListWAFListItems(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		listRequestLimit      int
		items                 []listItem
		listItemsRequestLimit int
		ok                    bool
		output                []api.WAFListItem
		prepareMocks          func(*mocks.MockPP)
	}{
		"success": {
			1,
			[]listItem{"10.0.0.1", "2001:db8::/32", "10.0.0.0/20"},
			1,
			true,
			[]api.WAFListItem{
				{ID: mockID("10.0.0.1", 0), Prefix: netip.MustParsePrefix("10.0.0.1/32")},
				{ID: mockID("2001:db8::/32", 0), Prefix: netip.MustParsePrefix("2001:db8::/32")},
				{ID: mockID("10.0.0.0/20", 0), Prefix: netip.MustParsePrefix("10.0.0.0/20")},
			},
			nil,
		},
		"list-fail": {
			0, nil, 0,
			false, nil,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to list existing lists: %v",
						gomock.Any(),
					),
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to find the list %q",
						"list",
					),
				)
			},
		},
		"list-item-fail": {
			1,
			[]listItem{"10.0.0.1"},
			0,
			false, nil,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Warningf(pp.EmojiError,
					"Failed to retrieve items in the list %q (ID: %s): %v",
					"list", mockID("list", 0), gomock.Any())
			},
		},
		"invalid": {
			1,
			[]listItem{"invalid item"},
			1,
			false, nil,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Failed to parse %q as an IP range: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Failed to parse %q as an IP address as well: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Found an invalid IP range/address %q in the list %q (ID: %s)",
						"invalid item", "list", mockID("list", 0)),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newGoodHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux)
			lih := newListListItemsHandler(t, mux, mockID("list", 0))

			lh.set([]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}}, tc.listRequestLimit)
			lih.set(tc.items, tc.listItemsRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			//nolint:forcetypeassert
			output, cached, ok := h.(api.CloudflareHandle).ListWAFListItems(context.Background(), mockPP, "list")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
			require.False(t, cached)
			require.True(t, lh.isExhausted())
			require.True(t, lih.isExhausted())

			if tc.ok {
				lh.set(nil, 0)
				lih.set(nil, 0)
				mockPP = mocks.NewMockPP(mockCtrl)
				//nolint:forcetypeassert
				output, cached, ok := h.(api.CloudflareHandle).ListWAFListItems(context.Background(), mockPP, "list")

				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.output, output)
				require.True(t, cached)
				require.True(t, lh.isExhausted())
				require.True(t, lih.isExhausted())
			}
		})
	}
}

func mockListBulkOperationResponse(id string) cloudflare.ListBulkOperationResponse {
	t := time.Now()
	return cloudflare.ListBulkOperationResponse{
		Response: mockResponse(),
		Result: cloudflare.ListBulkOperation{
			ID:        id,
			Status:    "completed",
			Error:     "",
			Completed: &t,
		},
	}
}

func handleListBulkOperation(t *testing.T, operationID string, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
		!assert.Equal(t, url.Values{}, r.URL.Query()) {
		panic(http.ErrAbortHandler)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockListBulkOperationResponse(operationID))
	assert.NoError(t, err)
}

func mockListItemDeleteResponse(id string) cloudflare.ListItemDeleteResponse {
	return cloudflare.ListItemDeleteResponse{
		Result: struct {
			OperationID string `json:"operation_id"` //nolint:tagliatelle
		}{OperationID: id},
		Response: mockResponse(),
	}
}

func handleDeleteListItems(t *testing.T, operationID string, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
		!assert.Equal(t, url.Values{}, r.URL.Query()) {
		panic(http.ErrAbortHandler)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockListItemDeleteResponse(operationID))
	assert.NoError(t, err)
}

type deleteListItemsHandler = httpHandler[struct{}]

func newDeleteListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID string) deleteListItemsHandler {
	t.Helper()

	var (
		dummy        struct{}
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("DELETE /accounts/%s/rules/lists/%s/items", mockAccountID, listID),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

			handleDeleteListItems(t, operationID, w, r)
		})

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists/bulk_operations/%s", mockAccountID, operationID),
		func(w http.ResponseWriter, r *http.Request) {
			handleListBulkOperation(t, operationID, w, r)
		})

	return deleteListItemsHandler{
		mux:          mux,
		params:       &dummy,
		requestLimit: &requestLimit,
	}
}

func TestDeleteWAFListItems(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		listRequestLimit      int
		idsToDelete           []string
		deleteRequestLimit    int
		listItemsResponse     []listItem
		listItemsRequestLimit int
		ok                    bool
		prepareMocks          func(*mocks.MockPP)
	}{
		"success": {
			1,
			[]string{"id1", "id2", "id3"},
			1,
			[]listItem{"10.0.0.1/32", "2001:db8::/32", "10.0.0.0/20"},
			1, true,
			nil,
		},
		"empty": {0, nil, 0, nil, 0, true, nil},
		"list-fail": {
			0,
			[]string{"id1", "id2", "id3"},
			0, nil, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to list existing lists: %v",
						gomock.Any(),
					),
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to find the list %q",
						"list",
					),
				)
			},
		},
		"delete-fail": {
			1,
			[]string{"id1", "id2", "id3"},
			0, nil, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Warningf(pp.EmojiError,
					"Failed to finish deleting items from the list %q (ID: %s): %v",
					"list", mockID("list", 0), gomock.Any())
			},
		},
		"list-items-invalid": {
			1,
			[]string{"id1", "id2", "id3"},
			1,
			[]listItem{"10.0.0.1/32", "2001:db8::/32", "invalid item"},
			1,
			false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Failed to parse %q as an IP range: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Failed to parse %q as an IP address as well: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Found an invalid IP range/address %q in the list %q (ID: %s)",
						"invalid item", "list", mockID("list", 0)),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newGoodHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux)
			dih := newDeleteListItemsHandler(t, mux, mockID("list", 0), mockID("op", 0))
			lih := newListListItemsHandler(t, mux, mockID("list", 0))

			lh.set([]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}}, tc.listRequestLimit)
			dih.set(struct{}{}, tc.deleteRequestLimit)
			lih.set(tc.listItemsResponse, tc.listItemsRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			//nolint:forcetypeassert
			ok = h.(api.CloudflareHandle).DeleteWAFListItems(context.Background(), mockPP, "list", tc.idsToDelete)
			require.Equal(t, tc.ok, ok)
			require.True(t, lh.isExhausted())
			require.True(t, dih.isExhausted())
			require.True(t, lih.isExhausted())

			if tc.ok {
				lh.set(nil, 0)
				dih.set(struct{}{}, tc.deleteRequestLimit)
				lih.set(nil, tc.listItemsRequestLimit)
				mockPP = mocks.NewMockPP(mockCtrl)
				//nolint:forcetypeassert
				ok := h.(api.CloudflareHandle).DeleteWAFListItems(context.Background(), mockPP, "list", tc.idsToDelete)
				require.Equal(t, tc.ok, ok)
				require.True(t, lh.isExhausted())
				require.True(t, dih.isExhausted())
				require.True(t, lih.isExhausted())
			}
		})
	}
}

func mockListItemCreateResponse(id string) cloudflare.ListItemCreateResponse {
	return cloudflare.ListItemCreateResponse{
		Result: struct {
			OperationID string `json:"operation_id"` //nolint:tagliatelle
		}{OperationID: id},
		Response: mockResponse(),
	}
}

func handleCreateListItems(t *testing.T, operationID string, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
		!assert.Equal(t, url.Values{}, r.URL.Query()) {
		panic(http.ErrAbortHandler)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockListItemCreateResponse(operationID))
	assert.NoError(t, err)
}

type createListItemsHandler = httpHandler[struct{}]

func newCreateListItemsHandler(t *testing.T, mux *http.ServeMux, listID, operationID string) createListItemsHandler {
	t.Helper()

	var (
		dummy        struct{}
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("POST /accounts/%s/rules/lists/%s/items", mockAccountID, listID),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

			handleCreateListItems(t, operationID, w, r)
		})

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists/bulk_operations/%s", mockAccountID, operationID),
		func(w http.ResponseWriter, r *http.Request) {
			handleListBulkOperation(t, operationID, w, r)
		})

	return createListItemsHandler{
		mux:          mux,
		params:       &dummy,
		requestLimit: &requestLimit,
	}
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
			[]listItem{"10.0.0.1/32", "2001:db8::/32", "10.0.0.0/20"},
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
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to list existing lists: %v",
						gomock.Any(),
					),
					ppfmt.EXPECT().Warningf(pp.EmojiError,
						"Failed to find the list %q",
						"list",
					),
				)
			},
		},
		"create-fail": {
			1,
			[]netip.Prefix{netip.MustParsePrefix("10.0.0.1/16"), netip.MustParsePrefix("2001:db8::/50")},
			0, nil, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Warningf(pp.EmojiError, "Failed to finish adding items to the list %q (ID: %s): %v",
					"list", mockID("list", 0), gomock.Any())
			},
		},
		"list-items-invalid": {
			1,
			[]netip.Prefix{netip.MustParsePrefix("10.0.0.1/16"), netip.MustParsePrefix("2001:db8::/50")},
			1,
			[]listItem{"10.0.0.1/32", "2001:db8::/32", "invalid item"},
			1,
			false,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Failed to parse %q as an IP range: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Failed to parse %q as an IP address as well: %v", "invalid item", gomock.Any()),
					ppfmt.EXPECT().Warningf(pp.EmojiImpossible,
						"Found an invalid IP range/address %q in the list %q (ID: %s)",
						"invalid item", "list", mockID("list", 0)),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newGoodHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux)
			cih := newCreateListItemsHandler(t, mux, mockID("list", 0), mockID("op", 0))
			lih := newListListItemsHandler(t, mux, mockID("list", 0))

			lh.set([]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}}, tc.listRequestLimit)
			cih.set(struct{}{}, tc.createRequestLimit)
			lih.set(tc.listItemsResponse, tc.listItemsRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			//nolint:forcetypeassert
			ok = h.(api.CloudflareHandle).CreateWAFListItems(context.Background(), mockPP,
				"list", tc.itemsToCreate, itemComment)
			require.Equal(t, tc.ok, ok)
			require.True(t, lh.isExhausted())
			require.True(t, cih.isExhausted())
			require.True(t, lih.isExhausted())

			if tc.ok {
				lh.set(nil, 0)
				cih.set(struct{}{}, tc.createRequestLimit)
				lih.set(nil, tc.listItemsRequestLimit)
				mockPP = mocks.NewMockPP(mockCtrl)
				//nolint:forcetypeassert
				ok = h.(api.CloudflareHandle).CreateWAFListItems(context.Background(), mockPP,
					"list", tc.itemsToCreate, itemComment)
				require.Equal(t, tc.ok, ok)
				require.True(t, lh.isExhausted())
				require.True(t, cih.isExhausted())
				require.True(t, lih.isExhausted())
			}
		})
	}
}
