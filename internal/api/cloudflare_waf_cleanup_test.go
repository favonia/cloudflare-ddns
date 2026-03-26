package api_test

// vim: nowrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func cleanupFamilies(families ...ipnet.Family) map[ipnet.Family]bool {
	result := make(map[ipnet.Family]bool, len(families))
	for _, ipFamily := range families {
		result[ipFamily] = true
	}
	return result
}

func mockDeleteListResponse(listID ID) cloudflare.ListDeleteResponse {
	return cloudflare.ListDeleteResponse{
		Response: mockResponse(),
		Result: struct {
			ID string `json:"id"`
		}{ID: string(listID)},
	}
}

func newDeleteListHandler(t *testing.T, mux *http.ServeMux, listID ID) httpHandler {
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
		lists                []listMeta
		listItems            []listItem
		listRequestLimit     int
		deleteListLimit      int
		listItemsLimit       int
		deleteListItemsLimit int
		deleteListItemIDs    []api.ID
		code                 api.WAFListCleanupCode
		prepareMocks         func(*mocks.MockPP)
	}{
		"success": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			nil,
			1, 1, 0, 0, nil,
			api.WAFListCleanupUpdated,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiDeletion, "The list %s was deleted", "account456/list")
			},
		},
		"list-fail": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			nil,
			0, 0, 0, 0, nil,
			api.WAFListCleanupFailed,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to retrieve existing lists: %v", gomock.Any())
			},
		},
		"list-not-found": {
			nil,
			nil,
			1, 0, 0, 0, nil,
			api.WAFListCleanupNoop,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiWarning,
					"The list %s was not found during final cleanup; treating it as already cleaned",
					"account456/list")
			},
		},
		"delete-fail/delete-items-async": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			[]listItem{
				{ID: "item-v4", Prefix: "10.0.0.1/32", Comment: "managed"},
				{ID: "item-v6", Prefix: "2001:db8::/64", Comment: "managed"},
			},
			1, 0, 1, 1,
			[]api.ID{"item-v4", "item-v6"},
			api.WAFListCleanupUpdating,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm deletion of the list %s; falling back to item deletion: %v", "account456/list", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiClear, "Deleting managed items in the list %s asynchronously", "account456/list"),
				)
			},
		},
		"delete-fail/items-empty": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			nil,
			1, 0, 1, 0, nil,
			api.WAFListCleanupNoop,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm deletion of the list %s; falling back to item deletion: %v", "account456/list", gomock.Any()),
					ppfmt.EXPECT().Infof(pp.EmojiAlreadyDone, "Managed items in the list %s were already deleted", "account456/list"),
				)
			},
		},
		"delete-fail/list-items-fail": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			nil,
			1, 0, 0, 0, nil,
			api.WAFListCleanupFailed,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm deletion of the list %s; falling back to item deletion: %v", "account456/list", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to retrieve items in the list %s: %v", "account456/list", gomock.Any()),
				)
			},
		},
		"delete-fail/delete-items-fail": {
			[]listMeta{{name: "list", size: 5, kind: cloudflare.ListTypeIP}},
			[]listItem{
				{ID: "item-v4", Prefix: "10.0.0.1/32", Comment: "managed"},
			},
			1, 0, 1, 0,
			[]api.ID{"item-v4"},
			api.WAFListCleanupFailed,
			func(ppfmt *mocks.MockPP) {
				gomock.InOrder(
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm deletion of the list %s; falling back to item deletion: %v", "account456/list", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm that item deletion started in the list %s: %v", "account456/list", gomock.Any()),
					ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm deletion of managed items in the list %s; list content may be inconsistent", "account456/list"),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			options := defaultHandleOptions()
			options.AllowWholeWAFListDeleteOnShutdown = true
			f := newCloudflareHarnessWithOptions(t, options)

			lh := newListListsHandler(t, f.serveMux, tc.lists)
			dh := newDeleteListHandler(t, f.serveMux, mockID("list", 0))
			lih := newListListItemsHandler(t, f.serveMux, mockID("list", 0), tc.listItems)
			dih := newDeleteListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), tc.deleteListItemIDs)

			lh.setRequestLimit(tc.listRequestLimit)
			dh.setRequestLimit(tc.deleteListLimit)
			lih.setRequestLimit(tc.listItemsLimit)
			dih.setRequestLimit(tc.deleteListItemsLimit)
			code := f.cfHandle.FinalCleanWAFList(
				context.Background(),
				f.newPreparedPP(tc.prepareMocks),
				mockWAFList,
				"description",
				cleanupFamilies(ipnet.IP4, ipnet.IP6),
			)
			require.Equal(t, tc.code, code)
			assertHandlersExhausted(t, lh, dh, lih, dih)
		})
	}
}

func TestFinalCleanWAFListSharedOwnership(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lists            []listMeta
		listItems        []listItem
		listRequestLimit int
		listItemsLimit   int
		deleteItemsLimit int
		deleteItemIDs    []api.ID
		code             api.WAFListCleanupCode
		prepareMocks     func(*mocks.MockPP)
	}{
		"delete-managed-items-async": {
			[]listMeta{{name: "list", size: 2, kind: cloudflare.ListTypeIP}},
			[]listItem{
				{ID: "managed-v4", Prefix: "10.0.0.1/32", Comment: "managed"},
				{ID: "foreign-v4", Prefix: "10.0.0.2/32", Comment: "foreign"},
			},
			1, 1, 1,
			[]api.ID{"managed-v4"},
			api.WAFListCleanupUpdating,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiClear,
					"Deleting managed items in the list %s asynchronously", "account456/list")
			},
		},
		"list-not-found": {
			nil,
			nil,
			1, 0, 0, nil,
			api.WAFListCleanupNoop,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Infof(pp.EmojiAlreadyDone,
					"Managed items in the list %s were already deleted", "account456/list")
			},
		},
		"list-items-fail": {
			[]listMeta{{name: "list", size: 2, kind: cloudflare.ListTypeIP}},
			nil,
			1, 0, 0, nil,
			api.WAFListCleanupFailed,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to retrieve items in the list %s: %v", "account456/list", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			options := defaultHandleOptions()
			options.ManagedWAFListItemsCommentRegex = regexp.MustCompile("^managed$")
			options.AllowWholeWAFListDeleteOnShutdown = false
			f := newCloudflareHarnessWithOptions(t, options)

			listHandler := newListListsHandler(t, f.serveMux, tc.lists)
			itemsHandler := newListListItemsHandler(t, f.serveMux, mockID("list", 0), tc.listItems)
			deleteHandler := newDeleteListItemsHandler(
				t, f.serveMux, mockID("list", 0), mockID("op", 0), tc.deleteItemIDs)

			listHandler.setRequestLimit(tc.listRequestLimit)
			itemsHandler.setRequestLimit(tc.listItemsLimit)
			deleteHandler.setRequestLimit(tc.deleteItemsLimit)
			code := f.cfHandle.FinalCleanWAFList(
				context.Background(),
				f.newPreparedPP(tc.prepareMocks),
				mockWAFList,
				"description",
				cleanupFamilies(ipnet.IP4, ipnet.IP6),
			)
			require.Equal(t, tc.code, code)
			assertHandlersExhausted(t, listHandler, itemsHandler, deleteHandler)
		})
	}
}

func TestFinalCleanWAFListSharedOwnershipCachedNoop(t *testing.T) {
	t.Parallel()

	options := defaultHandleOptions()
	options.ManagedWAFListItemsCommentRegex = regexp.MustCompile("^managed$")
	options.AllowWholeWAFListDeleteOnShutdown = false
	f := newCloudflareHarnessWithOptions(t, options)

	listHandler := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 1, kind: cloudflare.ListTypeIP}})
	itemsHandler := newListListItemsHandler(t, f.serveMux, mockID("list", 0),
		[]listItem{{ID: "foreign-v4", Prefix: "10.0.0.2/32", Comment: "foreign"}})
	deleteHandler := newDeleteListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), nil)

	listHandler.setRequestLimit(1)
	itemsHandler.setRequestLimit(1)
	deleteHandler.setRequestLimit(0)
	items, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(context.Background(), f.newPP(), mockWAFList, "description", "")
	require.True(t, ok)
	require.True(t, alreadyExisting)
	require.False(t, cached)
	require.Empty(t, items)

	cleanupPP := f.newPP()
	cleanupPP.EXPECT().Infof(pp.EmojiAlreadyDone,
		"Managed items in the list %s were already deleted (cached)", "account456/list")
	code := f.cfHandle.FinalCleanWAFList(context.Background(), cleanupPP, mockWAFList, "description",
		cleanupFamilies(ipnet.IP4, ipnet.IP6))
	require.Equal(t, api.WAFListCleanupNoop, code)
	assertHandlersExhausted(t, listHandler, itemsHandler, deleteHandler)
}

func TestFinalCleanWAFListPartialFamilyCleanup(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		initialItems      []listItem
		managedFamilies   map[ipnet.Family]bool
		deleteItemsLimit  int
		deleteItemIDs     []api.ID
		prepareCleanupPP  func(*mocks.MockPP)
		expectedCode      api.WAFListCleanupCode
		expectedCachedRun bool
	}{
		"delete-ipv4-only": {
			initialItems: []listItem{
				{ID: "item-v4", Prefix: "10.0.0.1/32", Comment: "managed"},
				{ID: "item-v6", Prefix: "2001:db8::/64", Comment: "managed"},
			},
			managedFamilies:  cleanupFamilies(ipnet.IP4),
			deleteItemsLimit: 1,
			deleteItemIDs:    []api.ID{"item-v4"},
			prepareCleanupPP: func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiClear,
					"Deleting managed IPv4 items in the list %s asynchronously", "account456/list")
			},
			expectedCode:      api.WAFListCleanupUpdating,
			expectedCachedRun: false,
		},
		"cached-noop-ipv6-only": {
			initialItems: []listItem{
				{ID: "item-v4", Prefix: "10.0.0.1/32", Comment: "managed"},
			},
			managedFamilies:  cleanupFamilies(ipnet.IP6),
			deleteItemsLimit: 0,
			deleteItemIDs:    nil,
			prepareCleanupPP: func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Infof(pp.EmojiAlreadyDone,
					"Managed IPv6 items in the list %s were already deleted (cached)", "account456/list")
			},
			expectedCode:      api.WAFListCleanupNoop,
			expectedCachedRun: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newCloudflareHarness(t)
			listHandler := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: len(tc.initialItems), kind: cloudflare.ListTypeIP}})
			itemsHandler := newListListItemsHandler(t, f.serveMux, mockID("list", 0), tc.initialItems)
			deleteHandler := newDeleteListItemsHandler(
				t, f.serveMux, mockID("list", 0), mockID("op", 0), tc.deleteItemIDs)

			listHandler.setRequestLimit(1)
			itemsHandler.setRequestLimit(1)
			deleteHandler.setRequestLimit(tc.deleteItemsLimit)

			items, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(
				context.Background(), f.newPP(), mockWAFList, "description", "managed")
			require.True(t, ok)
			require.True(t, alreadyExisting)
			require.False(t, cached)
			require.NotNil(t, items)

			cleanupPP := f.newPreparedPP(tc.prepareCleanupPP)
			code := f.cfHandle.FinalCleanWAFList(
				context.Background(), cleanupPP, mockWAFList, "description", tc.managedFamilies)
			require.Equal(t, tc.expectedCode, code)

			if tc.expectedCachedRun {
				listHandler.setRequestLimit(0)
				itemsHandler.setRequestLimit(0)
			}
			assertHandlersExhausted(t, listHandler, itemsHandler, deleteHandler)
		})
	}
}

func TestFinalCleanWAFListWholeListOwnershipFallbackIgnoresOutdatedCache(t *testing.T) {
	t.Parallel()

	options := defaultHandleOptions()
	options.AllowWholeWAFListDeleteOnShutdown = true
	f := newCloudflareHarnessWithOptions(t, options)

	listHandler := newListListsHandler(t, f.serveMux, []listMeta{{name: "list", size: 1, kind: cloudflare.ListTypeIP}})
	deleteListHandler := newDeleteListHandler(t, f.serveMux, mockID("list", 0))
	itemsHandler := newListListItemsHandler(t, f.serveMux, mockID("list", 0), nil)
	deleteItemsHandler := newDeleteListItemsHandler(t, f.serveMux, mockID("list", 0), mockID("op", 0), nil)

	listHandler.setRequestLimit(1)
	deleteListHandler.setRequestLimit(0)
	// First ListListItems call primes cache; second call verifies fallback re-fetches.
	itemsHandler.setRequestLimit(2)
	deleteItemsHandler.setRequestLimit(0)

	items, alreadyExisting, cached, ok := f.cfHandle.ListWAFListItems(context.Background(), f.newPP(), mockWAFList, "description", "")
	require.True(t, ok)
	require.True(t, alreadyExisting)
	require.False(t, cached)
	require.Empty(t, items)

	cleanupPP := f.newPP()
	gomock.InOrder(
		cleanupPP.EXPECT().Noticef(pp.EmojiError,
			"Could not confirm deletion of the list %s; falling back to item deletion: %v", "account456/list", gomock.Any()),
		cleanupPP.EXPECT().Infof(pp.EmojiAlreadyDone,
			"Managed items in the list %s were already deleted", "account456/list"),
	)
	code := f.cfHandle.FinalCleanWAFList(context.Background(), cleanupPP, mockWAFList, "description",
		cleanupFamilies(ipnet.IP4, ipnet.IP6))
	require.Equal(t, api.WAFListCleanupNoop, code)
	assertHandlersExhausted(t, listHandler, deleteListHandler, itemsHandler, deleteItemsHandler)
}

func TestFinalCleanWAFListWholeListModeSafeguard(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	newPP := mocks.NewMockPP(mockCtrl)
	options := defaultHandleOptions()
	options.ManagedWAFListItemsCommentRegex = regexp.MustCompile("^managed$")
	options.AllowWholeWAFListDeleteOnShutdown = true

	newPP.EXPECT().Noticef(pp.EmojiUserWarning,
		"DELETE_ON_STOP is enabled, but "+
			"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX (%s) is non-empty; "+
			"the updater will keep the list and delete only items managed by this updater",
		`"^managed$"`,
	)
	serveMux, h, ok := newHandleWithOptions(t, newPP, options)
	require.True(t, ok)
	cfHandle, ok := h.(api.CloudflareHandle)
	require.True(t, ok)

	listHandler := newListListsHandler(t, serveMux, nil)
	listHandler.setRequestLimit(1)

	cleanupPP := mocks.NewMockPP(mockCtrl)
	cleanupPP.EXPECT().Infof(pp.EmojiAlreadyDone,
		"Managed items in the list %s were already deleted", "account456/list")
	code := cfHandle.FinalCleanWAFList(context.Background(), cleanupPP, mockWAFList, "description",
		cleanupFamilies(ipnet.IP4, ipnet.IP6))
	require.Equal(t, api.WAFListCleanupNoop, code)
	assertHandlersExhausted(t, listHandler)
}

func TestFinalCleanWAFListWholeListModeSafeguardWithLongRegexPreview(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	newPP := mocks.NewMockPP(mockCtrl)
	options := defaultHandleOptions()
	regex := strings.Repeat("a", 49)
	options.ManagedWAFListItemsCommentRegex = regexp.MustCompile(regex)
	options.AllowWholeWAFListDeleteOnShutdown = true

	newPP.EXPECT().Noticef(pp.EmojiUserWarning,
		"DELETE_ON_STOP is enabled, but "+
			"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX (%s) is non-empty; "+
			"the updater will keep the list and delete only items managed by this updater",
		`"`+strings.Repeat("a", 48)+`..."`,
	)
	serveMux, h, ok := newHandleWithOptions(t, newPP, options)
	require.True(t, ok)
	cfHandle, ok := h.(api.CloudflareHandle)
	require.True(t, ok)

	listHandler := newListListsHandler(t, serveMux, nil)
	listHandler.setRequestLimit(1)

	cleanupPP := mocks.NewMockPP(mockCtrl)
	cleanupPP.EXPECT().Infof(pp.EmojiAlreadyDone,
		"Managed items in the list %s were already deleted", "account456/list")
	code := cfHandle.FinalCleanWAFList(context.Background(), cleanupPP, mockWAFList, "description",
		cleanupFamilies(ipnet.IP4, ipnet.IP6))
	require.Equal(t, api.WAFListCleanupNoop, code)
	assertHandlersExhausted(t, listHandler)
}
