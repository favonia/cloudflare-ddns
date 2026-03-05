package api

import (
	"context"
	"errors"
	"net/netip"
	"regexp"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// WAFListMaxBitLen records the maximum number of bits of an IP range/address
// Cloudflare can support in a WAF list.
//
// According to the Cloudflare docs, an IP range/address in a list must be
// in one of the following formats:
// - An individual IPv4 address
// - An IPv4 CIDR ranges with a prefix from /8 to /32
// - An IPv6 CIDR ranges with a prefix from /4 to /64
// For this updater, only the maximum values matter.
var WAFListMaxBitLen = map[ipnet.Type]int{ //nolint:gochecknoglobals
	ipnet.IP4: 32,
	ipnet.IP6: 64,
}

func hintWAFListPermission(ppfmt pp.PP, err error) {
	var authentication *cloudflare.AuthenticationError
	var authorization *cloudflare.AuthorizationError
	if errors.As(err, &authentication) || errors.As(err, &authorization) {
		ppfmt.NoticeOncef(pp.MessageWAFListPermission, pp.EmojiHint,
			"Double check your API token and account ID. "+
				`Make sure you granted the "Edit" permission of "Account - Account Filter Lists"`)
	}
}

func hintMismatchedDescription(ppfmt pp.PP, list WAFList, m WAFListMeta, expected string) {
	ppfmt.Noticef(pp.EmojiUserWarning,
		`The description for the list %s (ID: %s) is %s. However, its description is expected to be %s. You can either change the description at https://dash.cloudflare.com/%s/configurations/lists or change the value of WAF_LIST_DESCRIPTION to match the current description.`, //nolint:lll
		list.Describe(), m.ID, DescribeFreeFormString(m.Description), DescribeFreeFormString(expected), list.AccountID,
	)
}

func matchManagedWAFListItemComment(regex *regexp.Regexp, comment string) bool {
	if regex == nil {
		return true
	}
	return regex.MatchString(comment)
}

// ListWAFLists lists all IP lists of the given name.
func (h CloudflareHandle) ListWAFLists(ctx context.Context, ppfmt pp.PP, accountID ID) ([]WAFListMeta, bool) {
	if ls := h.cache.listLists.Get(accountID); ls != nil {
		return *ls.Value(), true
	}

	raw, err := h.cf.ListLists(ctx, cloudflare.AccountIdentifier(string(accountID)), cloudflare.ListListsParams{})
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to list existing lists: %v", err)
		hintWAFListPermission(ppfmt, err)
		return nil, false
	}

	ls := make([]WAFListMeta, 0, len(raw))
	for _, l := range raw {
		if l.Kind == cloudflare.ListTypeIP {
			ls = append(ls, WAFListMeta{
				ID:          ID(l.ID),
				Name:        l.Name,
				Description: l.Description,
			})
		}
	}

	h.cache.listLists.DeleteExpired()
	h.cache.listLists.Set(accountID, &ls, ttlcache.DefaultTTL)
	return ls, true
}

// WAFListID finds the ID of the list, if any.
// The second return value indicates whether the list is found.
func (h CloudflareHandle) WAFListID(ctx context.Context, ppfmt pp.PP, list WAFList,
	expectedDescription string,
) (ID, bool, bool) {
	if listID := h.cache.listID.Get(list); listID != nil {
		return listID.Value(), true, true
	}

	ls, ok := h.ListWAFLists(ctx, ppfmt, list.AccountID)
	if !ok {
		return "", false, false
	}

	count := 0
	listID := ID("")
	for _, l := range ls {
		if l.Name == list.Name {
			count++
			if count > 1 {
				ppfmt.Noticef(pp.EmojiImpossible,
					"Found multiple lists named %q within the account %s (IDs: %s and %s); please report this at %s",
					list.Name, list.AccountID, listID, l.ID, pp.IssueReportingURL,
				)
				return "", false, false
			}

			if l.Description != expectedDescription {
				hintMismatchedDescription(ppfmt, list, l, expectedDescription)
			}

			listID = l.ID
		}
	}

	if count == 0 {
		return "", false, true
	}

	h.cache.listID.DeleteExpired()
	h.cache.listID.Set(list, listID, ttlcache.DefaultTTL)
	return listID, true, true
}

// FindWAFList returns the ID of the IP list with the given name.
func (h CloudflareHandle) FindWAFList(ctx context.Context, ppfmt pp.PP, list WAFList,
	expectedDescription string,
) (ID, bool) {
	listID, found, ok := h.WAFListID(ctx, ppfmt, list, expectedDescription)
	if !ok || !found {
		// When ok is false, ListWAFLists (called by WAFListID) would have output some error messages,
		// but this provides more context.
		ppfmt.Noticef(pp.EmojiError, "Failed to find the list %s", list.Describe())
		return "", false
	}

	return listID, true
}

// FinalCleanWAFList removes managed WAF content during shutdown.
//
// If the handle owns the whole list, it tries to delete the list and falls back
// to deleting list items asynchronously. If the list is shared, it deletes only
// managed items selected by ManagedWAFListItemsCommentRegex.
//
// We delete cached data in listListItems and listID when the underlying list
// or its managed-item view may have changed, but we keep listLists so that we
// do not have to re-query all lists under the same account.
func (h CloudflareHandle) FinalCleanWAFList(ctx context.Context, ppfmt pp.PP,
	list WAFList, expectedDescription string,
) WAFListCleanupCode {
	if h.options.AllowWholeWAFListDeleteOnShutdown {
		return h.finalCleanWAFListWithScope(ctx, ppfmt, list, expectedDescription, false, true)
	}

	return h.finalCleanWAFListWithScope(ctx, ppfmt, list, expectedDescription, true, false)
}

const (
	finalWAFListManagedItemsAlreadyDeletedMessage = "The items managed by this updater in the list %s " +
		"were already deleted"
	finalWAFListManagedItemsAlreadyDeletedCachedMessage = "The items managed by this updater in the list %s " +
		"were already deleted (cached)"
	finalWAFListManagedItemsDeleteFailedMessage = "Failed to properly delete items managed by this updater from " +
		"the list %s; its content may be inconsistent"
	finalWAFListManagedItemsDeletingMessage = "The items managed by this updater in the list %s " +
		"are being deleted (asynchronously)"
)

func (h CloudflareHandle) finalCleanWAFListWithScope(ctx context.Context, ppfmt pp.PP,
	list WAFList, expectedDescription string, useManagedCache, tryDeleteWholeListFirst bool,
) WAFListCleanupCode {
	listID, found, ok := h.WAFListID(ctx, ppfmt, list, expectedDescription)
	if !ok {
		return WAFListCleanupFailed
	}
	if !found {
		if tryDeleteWholeListFirst {
			h.invalidateWAFListCleanupCache(list)
			ppfmt.Noticef(pp.EmojiWarning,
				"The list %s was not found during final cleanup; "+
					"it may have been removed or changed elsewhere, "+
					"so continuing as already cleaned", list.Describe())
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, finalWAFListManagedItemsAlreadyDeletedMessage, list.Describe())
		}
		return WAFListCleanupNoop
	}

	if tryDeleteWholeListFirst {
		if _, err := h.cf.DeleteList(ctx, cloudflare.AccountIdentifier(string(list.AccountID)), string(listID)); err == nil {
			h.invalidateWAFListCleanupCache(list)
			ppfmt.Noticef(pp.EmojiDeletion, "The list %s was deleted", list.Describe())
			return WAFListCleanupUpdated
		} else {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to delete the list %s; deleting its items instead: %v", list.Describe(), err)
		}
	}

	return h.finalCleanWAFListItemsAsync(ctx, ppfmt, list, listID, useManagedCache)
}

func (h CloudflareHandle) finalCleanWAFListItemsAsync(ctx context.Context, ppfmt pp.PP,
	list WAFList, listID ID, useManagedCache bool,
) WAFListCleanupCode {
	var items []WAFListItem
	var cached bool
	var ok bool

	if useManagedCache {
		if existing := h.cache.listListItems.Get(list); existing != nil {
			items = *existing.Value()
			cached = true
		} else {
			var allItems []WAFListItem
			allItems, ok = h.listWAFListItemsByID(ctx, ppfmt, list, listID)
			if !ok {
				return WAFListCleanupFailed
			}
			items = h.cacheManagedWAFListItems(list, allItems)
		}
	} else {
		items, ok = h.listWAFListItemsByID(ctx, ppfmt, list, listID)
		if !ok {
			return WAFListCleanupFailed
		}
	}

	if len(items) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, finalWAFListManagedItemsAlreadyDeletedCachedMessage, list.Describe())
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, finalWAFListManagedItemsAlreadyDeletedMessage, list.Describe())
		}
		return WAFListCleanupNoop
	}

	ids := make([]ID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if !h.startDeletingWAFListItemsAsync(ctx, ppfmt, list, listID, ids) {
		ppfmt.Noticef(pp.EmojiError, finalWAFListManagedItemsDeleteFailedMessage, list.Describe())
		return WAFListCleanupFailed
	}

	ppfmt.Noticef(pp.EmojiClear, finalWAFListManagedItemsDeletingMessage, list.Describe())
	return WAFListCleanupUpdating
}

func (h CloudflareHandle) invalidateWAFListCleanupCache(list WAFList) {
	h.cache.listListItems.Delete(list)
	h.cache.listID.Delete(list)
}

func (h CloudflareHandle) listWAFListItemsByID(ctx context.Context, ppfmt pp.PP,
	list WAFList, listID ID,
) ([]WAFListItem, bool) {
	rawItems, err := h.cf.ListListItems(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
		cloudflare.ListListItemsParams{ID: string(listID)}, //nolint:exhaustruct
	)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to retrieve items in the list %s: %v", list.Describe(), err)
		hintWAFListPermission(ppfmt, err)
		return nil, false
	}

	items, ok := readWAFListItems(ppfmt, list, rawItems)
	if !ok {
		return nil, false
	}

	return items, true
}

func (h CloudflareHandle) startDeletingWAFListItemsAsync(ctx context.Context, ppfmt pp.PP,
	list WAFList, listID ID, ids []ID,
) bool {
	if len(ids) == 0 {
		return true
	}

	itemRequests := make([]cloudflare.ListItemDeleteItemRequest, 0, len(ids))
	for _, id := range ids {
		itemRequests = append(itemRequests, cloudflare.ListItemDeleteItemRequest{ID: string(id)})
	}

	_, err := h.cf.DeleteListItemsAsync(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
		cloudflare.ListDeleteItemsParams{
			ID:    string(listID),
			Items: cloudflare.ListItemDeleteRequest{Items: itemRequests},
		},
	)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to start deleting items from the list %s: %v", list.Describe(), err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listListItems.Delete(list)
		return false
	}

	h.cache.listListItems.Delete(list)
	return true
}

func readWAFListItems(ppfmt pp.PP, list WAFList, rawItems []cloudflare.ListItem) ([]WAFListItem, bool) {
	items := make([]WAFListItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		if rawItem.IP == nil {
			ppfmt.Noticef(pp.EmojiImpossible, "Found a non-IP in the list %s", list.Describe())
			return nil, false
		}
		p, ok := ipnet.ParsePrefixOrIP(ppfmt, *rawItem.IP)
		if !ok {
			ppfmt.Noticef(pp.EmojiImpossible, "Found an invalid IP range/address %q in the list %s",
				*rawItem.IP, list.Describe())
			return nil, false
		}
		items = append(items, WAFListItem{ID: ID(rawItem.ID), Prefix: p, Comment: rawItem.Comment})
	}
	return items, true
}

func (h CloudflareHandle) filterManagedWAFListItems(items []WAFListItem) []WAFListItem {
	if h.options.ManagedWAFListItemsCommentRegex == nil {
		return items
	}

	managedItems := make([]WAFListItem, 0, len(items))
	for _, item := range items {
		if matchManagedWAFListItemComment(h.options.ManagedWAFListItemsCommentRegex, item.Comment) {
			managedItems = append(managedItems, item)
		}
	}
	return managedItems
}

func (h CloudflareHandle) cacheManagedWAFListItems(list WAFList, items []WAFListItem) []WAFListItem {
	managedItems := h.filterManagedWAFListItems(items)
	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(list, &managedItems, ttlcache.DefaultTTL)
	return managedItems
}

// ListWAFListItems calls cloudflare.ListListItems, and maybe cloudflare.CreateList when needed.
// It caches one managed-item view per handle/list pair.
func (h CloudflareHandle) ListWAFListItems(ctx context.Context, ppfmt pp.PP,
	list WAFList, expectedDescription string,
) ([]WAFListItem, bool, bool, bool) {
	if items := h.cache.listListItems.Get(list); items != nil {
		return *items.Value(), true, true, true
	}

	listID, found, ok := h.WAFListID(ctx, ppfmt, list, expectedDescription)
	if !ok {
		// ListWAFLists (called by WAFListID) would have output some error messages,
		// but this provides more context.
		ppfmt.Noticef(pp.EmojiError, "Failed to check the existence of the list %s", list.Describe())
		return nil, false, false, false
	}
	if !found {
		r, err := h.cf.CreateList(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
			cloudflare.ListCreateParams{
				Name:        list.Name,
				Description: expectedDescription,
				Kind:        cloudflare.ListTypeIP,
			})
		if err != nil {
			ppfmt.Noticef(pp.EmojiError, "Failed to create the list %s: %v", list.Describe(), err)
			hintWAFListPermission(ppfmt, err)
			h.cache.listLists.Delete(list.AccountID)
			return nil, false, false, false
		}

		listID = ID(r.ID)
		var items []WAFListItem

		if ls := h.cache.listLists.Get(list.AccountID); ls != nil {
			*ls.Value() = append([]WAFListMeta{{ID: listID, Description: expectedDescription, Name: list.Name}}, *ls.Value()...)
		}
		h.cache.listID.DeleteExpired()
		h.cache.listID.Set(list, listID, ttlcache.DefaultTTL)
		return h.cacheManagedWAFListItems(list, items), false, false, true
	}

	items, ok := h.listWAFListItemsByID(ctx, ppfmt, list, listID)
	if !ok {
		return nil, false, false, false
	}

	return h.cacheManagedWAFListItems(list, items), true, false, true
}

// DeleteWAFListItems calls cloudflare.DeleteListItems.
func (h CloudflareHandle) DeleteWAFListItems(ctx context.Context, ppfmt pp.PP,
	list WAFList, expectedDescription string,
	ids []ID,
) bool {
	if len(ids) == 0 {
		return true
	}

	listID, ok := h.FindWAFList(ctx, ppfmt, list, expectedDescription)
	if !ok {
		return false
	}

	itemRequests := make([]cloudflare.ListItemDeleteItemRequest, 0, len(ids))
	for _, id := range ids {
		itemRequests = append(itemRequests, cloudflare.ListItemDeleteItemRequest{ID: string(id)})
	}

	rawItems, err := h.cf.DeleteListItems(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
		cloudflare.ListDeleteItemsParams{
			ID:    string(listID),
			Items: cloudflare.ListItemDeleteRequest{Items: itemRequests},
		},
	)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to finish deleting items from the list %s: %v", list.Describe(), err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listListItems.Delete(list)
		return false
	}

	items, ok := readWAFListItems(ppfmt, list, rawItems)
	if !ok {
		return false
	}

	h.cacheManagedWAFListItems(list, items)
	return true
}

// CreateWAFListItems calls cloudflare.CreateListItems.
func (h CloudflareHandle) CreateWAFListItems(ctx context.Context, ppfmt pp.PP,
	list WAFList, expectedDescription string,
	itemsToCreate []netip.Prefix, comment string,
) bool {
	if len(itemsToCreate) == 0 {
		return true
	}

	listID, ok := h.FindWAFList(ctx, ppfmt, list, expectedDescription)
	if !ok {
		return false
	}

	rawItemsToCreate := make([]cloudflare.ListItemCreateRequest, 0, len(itemsToCreate))
	for _, item := range itemsToCreate {
		formattedPrefix := ipnet.DescribePrefixOrIP(item)
		rawItemsToCreate = append(rawItemsToCreate, cloudflare.ListItemCreateRequest{ //nolint:exhaustruct
			IP:      &formattedPrefix,
			Comment: comment,
		})
	}

	rawItems, err := h.cf.CreateListItems(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
		cloudflare.ListCreateItemsParams{
			ID:    string(listID),
			Items: rawItemsToCreate,
		},
	)
	if err != nil {
		ppfmt.Noticef(
			pp.EmojiError, "Failed to finish adding items to the list %s: %v",
			list.Describe(), err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listListItems.Delete(list)
		return false
	}

	items, ok := readWAFListItems(ppfmt, list, rawItems)
	if !ok {
		return false
	}

	h.cacheManagedWAFListItems(list, items)
	return true
}
