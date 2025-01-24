// vim: nowrap
package api_test

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

//nolint:gochecknoglobals
var mockWAFList = api.WAFList{AccountID: mockAccountID, Name: "list"}

type listMeta struct {
	name string
	size int
	kind string
}

func mockList(meta listMeta, i int) cloudflare.List {
	return cloudflare.List{
		ID:                    string(mockID(meta.name, i)),
		Name:                  meta.name,
		Description:           "description",
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

func newListListsHandler(t *testing.T, mux *http.ServeMux, listMetas []listMeta) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists", mockAccountID),
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
			err := json.NewEncoder(w).Encode(mockListsResponse(listMetas))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestListWAFLists(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lists        []listMeta
		ok           bool
		output       []api.WAFListMeta
		prepareMocks func(*mocks.MockPP)
	}{
		"empty": {
			[]listMeta{},
			true,
			[]api.WAFListMeta{},
			nil,
		},
		"2ip1asn": {
			[]listMeta{
				{name: "list", size: 10, kind: cloudflare.ListTypeIP},
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			true,
			[]api.WAFListMeta{
				{ID: mockID("list", 0), Name: "list", Description: "description"},
				{ID: mockID("list", 2), Name: "list", Description: "description"},
			},
			nil,
		},
		"1ip1asn": {
			[]listMeta{
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			true,
			[]api.WAFListMeta{
				{ID: mockID("list", 1), Name: "list", Description: "description"},
			},
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux, tc.lists)

			lh.setRequestLimit(1)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			lists, ok := h.(api.CloudflareHandle).ListWAFLists(context.Background(), mockPP, mockAccountID)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, lists)
			require.True(t, lh.isExhausted())

			if tc.ok {
				mockPP = mocks.NewMockPP(mockCtrl)
				if tc.prepareMocks != nil {
					tc.prepareMocks(mockPP)
				}
				lists, ok = h.(api.CloudflareHandle).ListWAFLists(context.Background(), mockPP, mockAccountID)
				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.output, lists)
			}

			h.(api.CloudflareHandle).FlushCache() //nolint:forcetypeassert

			mockPP = mocks.NewMockPP(mockCtrl)
			mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any())
			lists, ok = h.(api.CloudflareHandle).ListWAFLists(context.Background(), mockPP, mockAccountID)
			require.False(t, ok)
			require.Zero(t, lists)
			require.True(t, lh.isExhausted())
		})
	}
}

func TestWAFListID(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lists        []listMeta
		description  string
		ok           bool
		found        bool
		output       api.ID
		prepareMocks func(*mocks.MockPP)
	}{
		"empty": {
			[]listMeta{},
			"description",
			true, false, "",
			nil,
		},
		"2ip1asn": {
			[]listMeta{
				{name: "list", size: 10, kind: cloudflare.ListTypeIP},
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			"description",
			false, false, "",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Found multiple lists named %q within the account %s (IDs: %s and %s); please report this at %s", "list", mockAccountID, mockID("list", 0), mockID("list", 2), pp.IssueReportingURL)
			},
		},
		"1ip1asn": {
			[]listMeta{
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			"description",
			true, true, mockID("list", 1),
			nil,
		},
		"mismatched-description": {
			[]listMeta{
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			"mismatched description",
			true, true, mockID("list", 1),
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
						`The description for the list %s (ID: %s) is %s. However, its description is expected to be %s. You can either change the description at https://dash.cloudflare.com/%s/configurations/lists or change the value of WAF_LIST_DESCRIPTION to match the current description.`,
						"account456/list", mockID("list", 1), `"description"`, `"mismatched description"`, api.ID("account456")),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux, tc.lists)

			lh.setRequestLimit(1)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			//nolint: forcetypeassert
			id, found, ok := h.(api.CloudflareHandle).WAFListID(context.Background(), mockPP, mockWAFList, tc.description)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.found, found)
			require.Equal(t, tc.output, id)
			require.True(t, lh.isExhausted())

			if tc.ok {
				mockPP = mocks.NewMockPP(mockCtrl)
				//nolint: forcetypeassert
				id, found, ok = h.(api.CloudflareHandle).WAFListID(context.Background(), mockPP, mockWAFList, tc.description)
				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.found, found)
				require.Equal(t, tc.output, id)
			}

			h.(api.CloudflareHandle).FlushCache() //nolint:forcetypeassert

			mockPP = mocks.NewMockPP(mockCtrl)
			mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any())
			//nolint: forcetypeassert
			id, found, ok = h.(api.CloudflareHandle).WAFListID(context.Background(), mockPP, mockWAFList, tc.description)
			require.False(t, ok)
			require.Zero(t, found)
			require.Zero(t, id)
			require.True(t, lh.isExhausted())
		})
	}
}

func TestListWAFListsHint(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	mux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists", mockAccountID),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`))
			assert.NoError(t, err)
		})

	mockPP = mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
		mockPP.EXPECT().NoticeOncef(pp.MessageWAFListPermission, pp.EmojiHint, `Double check your API token and account ID. Make sure you granted the "Edit" permission of "Account - Account Filter Lists"`),
	)
	lists, ok := h.(api.CloudflareHandle).ListWAFLists(context.Background(), mockPP, mockAccountID)
	require.False(t, ok)
	require.Zero(t, lists)
}

func TestFindWAFList(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lists            []listMeta
		listRequestLimit int
		ok               bool
		output           api.ID
		prepareMocks     func(*mocks.MockPP)
	}{
		"list-fail": {
			nil,
			0,
			false, "",
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find the list %s", "account456/list"),
				)
			},
		},
		"empty": {
			[]listMeta{},
			1,
			false, "",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find the list %s", "account456/list")
			},
		},
		"1ip1asn": {
			[]listMeta{
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			1,
			true, (mockID("list", 1)),
			nil,
		},
		"2ip1asn": {
			[]listMeta{
				{name: "list", size: 10, kind: cloudflare.ListTypeIP},
				{name: "list", size: 11, kind: cloudflare.ListTypeASN},
				{name: "list", size: 12, kind: cloudflare.ListTypeIP},
			},
			1,
			false, "",
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Found multiple lists named %q within the account %s (IDs: %s and %s); please report this at %s", "list", mockAccountID, mockID("list", 0), mockID("list", 2), pp.IssueReportingURL),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to find the list %s", "account456/list"),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux, tc.lists)
			lh.setRequestLimit(tc.listRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			list, ok := h.(api.CloudflareHandle).FindWAFList(context.Background(), mockPP, mockWAFList, "description")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, list)
			require.True(t, lh.isExhausted())

			if tc.ok {
				mockPP = mocks.NewMockPP(mockCtrl)
				if tc.prepareMocks != nil {
					tc.prepareMocks(mockPP)
				}
				list, ok = h.(api.CloudflareHandle).FindWAFList(context.Background(), mockPP, mockWAFList, "description")
				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.output, list)
			}
		})
	}
}

func mockListResponse(meta listMeta) cloudflare.ListResponse {
	return cloudflare.ListResponse{
		Result:   mockList(meta, 0),
		Response: mockResponse(),
	}
}

func newCreateListHandler(t *testing.T, mux *http.ServeMux, listMeta listMeta) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("POST /accounts/%s/rules/lists", mockAccountID),
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
			err := json.NewEncoder(w).Encode(mockListResponse(listMeta))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

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
		"delete-fail/clear-fail/keep-cache": {
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

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			dh := newDeleteListHandler(t, mux, mockID("list", 0))
			rih := newReplaceListItemsHandler(t, mux, mockID("list", 0), mockID("op", 0))

			lh.setRequestLimit(tc.listRequestLimit)
			dh.setRequestLimit(tc.deleteRequestLimit)
			rih.setRequestLimit(tc.replaceRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			deleted, ok := h.(api.CloudflareHandle).FinalClearWAFListAsync(context.Background(), mockPP,
				mockWAFList, "description")
			require.Equal(t, tc.deleted, deleted)
			require.Equal(t, tc.ok, ok)
			require.True(t, lh.isExhausted())
			require.True(t, dh.isExhausted())
			require.True(t, rih.isExhausted())
		})
	}
}

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
					"The IP range/address %q in the list %s has a non-empty comment %q; the comment might be lost during an IP update.",
					"10.0.0.1", "account456/list", "hello")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux, tc.lists)
			ch := newCreateListHandler(t, mux, tc.newList)
			lih := newListListItemsHandler(t, mux, mockID("list", 0), tc.items)

			lh.setRequestLimit(tc.listRequestLimit)
			ch.setRequestLimit(tc.createRequestLimit)
			lih.setRequestLimit(tc.listItemsRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			//nolint:forcetypeassert
			output, alreadyExisting, cached, ok := h.(api.CloudflareHandle).
				ListWAFListItems(context.Background(), mockPP, mockWAFList, "description")
			require.Equal(t, tc.ok, ok)
			require.False(t, cached)
			require.Equal(t, tc.alreadyExisting, alreadyExisting)
			require.Equal(t, tc.output, output)
			require.True(t, lh.isExhausted())
			require.True(t, ch.isExhausted())
			require.True(t, lih.isExhausted())

			if tc.ok {
				mockPP = mocks.NewMockPP(mockCtrl)
				//nolint:forcetypeassert
				output, alreadyExisting, cached, ok := h.(api.CloudflareHandle).
					ListWAFListItems(context.Background(), mockPP, mockWAFList, "description")
				require.Equal(t, tc.ok, ok)
				require.True(t, cached)
				require.True(t, alreadyExisting)
				require.Equal(t, tc.output, output)
				require.True(t, lh.isExhausted())
				require.True(t, lih.isExhausted())
			}
		})
	}
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

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			dih := newDeleteListItemsHandler(t, mux, mockID("list", 0), mockID("op", 0))
			lih := newListListItemsHandler(t, mux, mockID("list", 0), tc.listItemsResponse)

			lh.setRequestLimit(tc.listRequestLimit)
			dih.setRequestLimit(tc.deleteRequestLimit)
			lih.setRequestLimit(tc.listItemsRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			//nolint:forcetypeassert
			ok = h.(api.CloudflareHandle).
				DeleteWAFListItems(context.Background(), mockPP, mockWAFList, "description", tc.idsToDelete)
			require.Equal(t, tc.ok, ok)
			require.True(t, lh.isExhausted())
			require.True(t, dih.isExhausted())
			require.True(t, lih.isExhausted())

			if tc.ok {
				dih.setRequestLimit(tc.deleteRequestLimit)
				lih.setRequestLimit(tc.listItemsRequestLimit)
				mockPP = mocks.NewMockPP(mockCtrl)
				//nolint:forcetypeassert
				ok := h.(api.CloudflareHandle).
					DeleteWAFListItems(context.Background(), mockPP, mockWAFList, "description", tc.idsToDelete)
				require.Equal(t, tc.ok, ok)
				require.True(t, lh.isExhausted())
				require.True(t, dih.isExhausted())
				require.True(t, lih.isExhausted())
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

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			lh := newListListsHandler(t, mux, []listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}})
			cih := newCreateListItemsHandler(t, mux, mockID("list", 0), mockID("op", 0))
			lih := newListListItemsHandler(t, mux, mockID("list", 0), tc.listItemsResponse)

			lh.setRequestLimit(tc.listRequestLimit)
			cih.setRequestLimit(tc.createRequestLimit)
			lih.setRequestLimit(tc.listItemsRequestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}
			//nolint:forcetypeassert
			ok = h.(api.CloudflareHandle).CreateWAFListItems(context.Background(), mockPP,
				mockWAFList, "description", tc.itemsToCreate, itemComment)
			require.Equal(t, tc.ok, ok)
			require.True(t, lh.isExhausted())
			require.True(t, cih.isExhausted())
			require.True(t, lih.isExhausted())

			if tc.ok {
				cih.setRequestLimit(tc.createRequestLimit)
				lih.setRequestLimit(tc.listItemsRequestLimit)
				mockPP = mocks.NewMockPP(mockCtrl)
				//nolint:forcetypeassert
				ok = h.(api.CloudflareHandle).CreateWAFListItems(context.Background(), mockPP,
					mockWAFList, "description", tc.itemsToCreate, itemComment)
				require.Equal(t, tc.ok, ok)
				require.True(t, lh.isExhausted())
				require.True(t, cih.isExhausted())
				require.True(t, lih.isExhausted())
			}
		})
	}
}
