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

			f := newCloudflareHarness(t)

			lh := newListListsHandler(t, f.serveMux, tc.lists)
			lh.setRequestLimit(1)

			lists, ok := f.cfHandle.ListWAFLists(context.Background(), f.newPreparedPP(tc.prepareMocks), mockAccountID)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, lists)
			assertHandlersExhausted(t, lh)

			f.cfHandle.FlushCache()

			mockPP := f.newPP()
			mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any())
			lists, ok = f.cfHandle.ListWAFLists(context.Background(), mockPP, mockAccountID)
			require.False(t, ok)
			require.Zero(t, lists)
			assertHandlersExhausted(t, lh)
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

			f := newCloudflareHarness(t)

			lh := newListListsHandler(t, f.serveMux, tc.lists)
			lh.setRequestLimit(1)

			id, found, ok := f.cfHandle.WAFListID(context.Background(), f.newPreparedPP(tc.prepareMocks), mockWAFList, tc.description)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.found, found)
			require.Equal(t, tc.output, id)
			assertHandlersExhausted(t, lh)

			f.cfHandle.FlushCache()

			mockPP := f.newPP()
			mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any())
			id, found, ok = f.cfHandle.WAFListID(context.Background(), mockPP, mockWAFList, tc.description)
			require.False(t, ok)
			require.Zero(t, found)
			require.Zero(t, id)
			assertHandlersExhausted(t, lh)
		})
	}
}

func TestListWAFListsCache(t *testing.T) {
	t.Parallel()

	f := newCloudflareHarness(t)
	lh := newListListsHandler(t, f.serveMux, []listMeta{
		{name: "list", size: 10, kind: cloudflare.ListTypeIP},
		{name: "list", size: 11, kind: cloudflare.ListTypeASN},
		{name: "list", size: 12, kind: cloudflare.ListTypeIP},
	})

	lh.setRequestLimit(1)
	lists, ok := f.cfHandle.ListWAFLists(context.Background(), f.newPP(), mockAccountID)
	require.True(t, ok)
	require.Equal(t, []api.WAFListMeta{
		{ID: mockID("list", 0), Name: "list", Description: "description"},
		{ID: mockID("list", 2), Name: "list", Description: "description"},
	}, lists)
	assertHandlersExhausted(t, lh)

	lh.setRequestLimit(0)
	lists, ok = f.cfHandle.ListWAFLists(context.Background(), f.newPP(), mockAccountID)
	require.True(t, ok)
	require.Equal(t, []api.WAFListMeta{
		{ID: mockID("list", 0), Name: "list", Description: "description"},
		{ID: mockID("list", 2), Name: "list", Description: "description"},
	}, lists)
	assertHandlersExhausted(t, lh)
}

func TestWAFListIDCache(t *testing.T) {
	t.Parallel()

	f := newCloudflareHarness(t)
	lh := newListListsHandler(t, f.serveMux, []listMeta{
		{name: "list", size: 11, kind: cloudflare.ListTypeASN},
		{name: "list", size: 12, kind: cloudflare.ListTypeIP},
	})

	lh.setRequestLimit(1)
	id, found, ok := f.cfHandle.WAFListID(context.Background(), f.newPP(), mockWAFList, "description")
	require.True(t, ok)
	require.True(t, found)
	require.Equal(t, mockID("list", 1), id)
	assertHandlersExhausted(t, lh)

	lh.setRequestLimit(0)
	id, found, ok = f.cfHandle.WAFListID(context.Background(), f.newPP(), mockWAFList, "description")
	require.True(t, ok)
	require.True(t, found)
	require.Equal(t, mockID("list", 1), id)
	assertHandlersExhausted(t, lh)
}

func TestListWAFListsHint(t *testing.T) {
	t.Parallel()

	f := newCloudflareHarness(t)

	f.serveMux.HandleFunc(fmt.Sprintf("GET /accounts/%s/rules/lists", mockAccountID),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`))
			assert.NoError(t, err)
		})

	mockPP := f.newPP()
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to list existing lists: %v", gomock.Any()),
		mockPP.EXPECT().NoticeOncef(pp.MessageWAFListPermission, pp.EmojiHint, `Double check your API token and account ID. Make sure you granted the "Edit" permission of "Account - Account Filter Lists"`),
	)
	lists, ok := f.cfHandle.ListWAFLists(context.Background(), mockPP, mockAccountID)
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

			f := newCloudflareHarness(t)
			lh := newListListsHandler(t, f.serveMux, tc.lists)
			lh.setRequestLimit(tc.listRequestLimit)
			list, ok := f.cfHandle.FindWAFList(context.Background(), f.newPreparedPP(tc.prepareMocks), mockWAFList, "description")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, list)
			assertHandlersExhausted(t, lh)
		})
	}
}

func TestFindWAFListCache(t *testing.T) {
	t.Parallel()

	f := newCloudflareHarness(t)
	lh := newListListsHandler(t, f.serveMux, []listMeta{
		{name: "list", size: 11, kind: cloudflare.ListTypeASN},
		{name: "list", size: 12, kind: cloudflare.ListTypeIP},
	})

	lh.setRequestLimit(1)
	list, ok := f.cfHandle.FindWAFList(context.Background(), f.newPP(), mockWAFList, "description")
	require.True(t, ok)
	require.Equal(t, mockID("list", 1), list)
	assertHandlersExhausted(t, lh)

	lh.setRequestLimit(0)
	list, ok = f.cfHandle.FindWAFList(context.Background(), f.newPP(), mockWAFList, "description")
	require.True(t, ok)
	require.Equal(t, mockID("list", 1), list)
	assertHandlersExhausted(t, lh)
}

func mockListResponse(meta listMeta) cloudflare.ListResponse {
	return cloudflare.ListResponse{
		Result:   mockList(meta, 0),
		Response: mockResponse(),
	}
}

func newCreateListHandler(t *testing.T, mux *http.ServeMux,
	expected cloudflare.ListCreateRequest, listMeta listMeta,
) httpHandler {
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

			var createRequest cloudflare.ListCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&createRequest); !assert.NoError(t, err) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if !assert.Equal(t, expected, createRequest) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(mockListResponse(listMeta))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}
