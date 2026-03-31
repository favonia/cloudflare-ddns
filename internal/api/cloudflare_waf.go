package api

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func hintWAFListPermission(ppfmt pp.PP, err error) {
	var authentication *cloudflare.AuthenticationError
	var authorization *cloudflare.AuthorizationError
	if errors.As(err, &authentication) || errors.As(err, &authorization) {
		ppfmt.NoticeOncef(pp.MessageWAFListPermission, pp.EmojiHint,
			"Double-check your API token and account ID. "+
				`Make sure you granted the "Edit" permission of "Account - Account Filter Lists"`)
	}
}

func hintMismatchedDescription(ppfmt pp.PP, list WAFList, m wafListMeta, fallbackDescription string) {
	ppfmt.Noticef(pp.EmojiUserWarning,
		`The description for the list %s (ID: %s) is %s, which is different from the fallback description %s. You can change the description at %s.`, //nolint:lll
		list.Describe(), m.ID,
		pp.QuoteOrEmptyLabel(m.Description, "empty"),
		pp.QuoteOrEmptyLabel(fallbackDescription, "(empty)"),
		cloudflareWAFListDeeplink(list.AccountID, m.ID),
	)
}

func hintMismatchedWAFListItemComment(
	ppfmt pp.PP, list WAFList, managedItems []WAFListItem, fallbackItemComment string,
) {
	mismatchedCount := 0
	var sampleID ID
	var sampleComment string
	var sampleDescription string

	for _, item := range managedItems {
		if item.Comment == fallbackItemComment {
			continue
		}

		mismatchedCount++
		if mismatchedCount == 1 {
			sampleID = item.ID
			sampleComment = item.Comment
			sampleDescription = item.Prefix.String()
		}
	}

	if mismatchedCount == 0 {
		return
	}

	// This is intentionally a warn-only drift notice. Comment mismatches for
	// already managed WAF list items may come from pre-existing state, and they
	// are not corrected by reconciliation. Keep the message focused on that
	// observable behavior rather than on internal mutation mechanics.
	ppfmt.Noticef(pp.EmojiUserWarning,
		"The comment on the item %s (ID: %s) in the list %s is %s, which is different from the fallback comment %s. "+
			"Found %d managed WAF list item(s) with mismatched comments in the list. "+
			"These mismatches are reported but not corrected.",
		sampleDescription,
		sampleID,
		list.Describe(),
		pp.QuoteOrEmptyLabel(sampleComment, "empty"),
		pp.QuoteOrEmptyLabel(fallbackItemComment, "(empty)"),
		mismatchedCount,
	)
}

func snapshotManagedWAFListItemCommentsByID(items []WAFListItem) map[ID]string {
	commentsByID := make(map[ID]string, len(items))
	for _, item := range items {
		commentsByID[item.ID] = item.Comment
	}
	return commentsByID
}

func (h cloudflareHandle) cachedManagedWAFListItemCommentsByID(list WAFList) (map[ID]string, bool) {
	cached := h.cache.listListItems.Get(list)
	if cached == nil {
		return nil, false
	}

	return snapshotManagedWAFListItemCommentsByID(*cached.Value()), true
}

func asAllowedWAFListItemCommentsSet(comments []string) map[string]bool {
	result := make(map[string]bool, len(comments))
	for _, comment := range comments {
		result[comment] = true
	}
	return result
}

func allowedWAFListItemCommentsSetFromCreateItems(items []WAFListCreateItem) map[string]bool {
	comments := make([]string, 0, len(items))
	for _, item := range items {
		comments = append(comments, item.Comment)
	}
	return asAllowedWAFListItemCommentsSet(comments)
}

func describeAllowedWAFListItemComments(allowedComments map[string]bool) string {
	if len(allowedComments) == 0 {
		return "none"
	}

	comments := make([]string, 0, len(allowedComments))
	for comment := range allowedComments {
		comments = append(comments, pp.QuoteOrEmptyLabel(comment, "empty"))
	}
	slices.Sort(comments)
	return strings.Join(comments, ", ")
}

func hintUnexpectedWAFListItemCommentAfterMutation(ppfmt pp.PP, list WAFList,
	beforeCommentsByID map[ID]string, managedItems []WAFListItem, allowedPostMutationComments map[string]bool,
) {
	mismatchedCount := 0
	var sampleID ID
	var sampleComment string
	var sampleDescription string

	for _, item := range managedItems {
		beforeComment, hadBefore := beforeCommentsByID[item.ID]
		if hadBefore {
			if item.Comment == beforeComment || allowedPostMutationComments[item.Comment] {
				continue
			}
		} else if allowedPostMutationComments[item.Comment] {
			continue
		}

		mismatchedCount++
		if mismatchedCount == 1 {
			sampleID = item.ID
			sampleComment = item.Comment
			sampleDescription = item.Prefix.String()
		}
	}

	if mismatchedCount == 0 {
		return
	}

	ppfmt.Noticef(pp.EmojiUserWarning,
		"After updating the list %s, the comment on the item %s (ID: %s) is %s, which is unexpected given "+
			"allowed post-mutation comments (%s) and pre-update cache state. "+
			"Found %d managed WAF list item(s) with this anomaly.",
		list.Describe(),
		sampleDescription,
		sampleID,
		pp.QuoteOrEmptyLabel(sampleComment, `empty`),
		describeAllowedWAFListItemComments(allowedPostMutationComments),
		mismatchedCount,
	)
}

// listWAFLists lists all IP lists of the given name.
func (h cloudflareHandle) listWAFLists(ctx context.Context, ppfmt pp.PP, accountID ID) ([]wafListMeta, bool) {
	if ls := h.cache.listLists.Get(accountID); ls != nil {
		return *ls.Value(), true
	}

	raw, err := h.cf.ListLists(ctx, cloudflare.AccountIdentifier(string(accountID)), cloudflare.ListListsParams{})
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to retrieve existing lists: %v", err)
		hintWAFListPermission(ppfmt, err)
		return nil, false
	}

	ls := make([]wafListMeta, 0, len(raw))
	for _, l := range raw {
		if l.Kind == cloudflare.ListTypeIP {
			ls = append(ls, wafListMeta{
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

// wafListID finds the ID of the list, if any.
// The second return value indicates whether the list is found.
func (h cloudflareHandle) wafListID(ctx context.Context, ppfmt pp.PP, list WAFList,
	fallbackDescription string,
) (ID, bool, bool) {
	if listID := h.cache.listID.Get(list); listID != nil {
		return listID.Value(), true, true
	}

	ls, ok := h.listWAFLists(ctx, ppfmt, list.AccountID)
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

			if l.Description != fallbackDescription {
				hintMismatchedDescription(ppfmt, list, l, fallbackDescription)
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

// findWAFList returns the ID of the IP list with the given name.
func (h cloudflareHandle) findWAFList(ctx context.Context, ppfmt pp.PP, list WAFList,
	fallbackDescription string,
) (ID, bool) {
	listID, found, ok := h.wafListID(ctx, ppfmt, list, fallbackDescription)
	if !ok || !found {
		// When ok is false, listWAFLists (called by wafListID) would have output some error messages,
		// but this provides more context.
		ppfmt.Noticef(pp.EmojiError, "Failed to find the list %s", list.Describe())
		return "", false
	}

	return listID, true
}

// ensureWAFList returns the ID of the IP list with the given name, creating it if needed.
func (h cloudflareHandle) ensureWAFList(ctx context.Context, ppfmt pp.PP, list WAFList,
	fallbackDescription string,
) (ID, bool) {
	listID, found, ok := h.wafListID(ctx, ppfmt, list, fallbackDescription)
	if !ok {
		ppfmt.Noticef(pp.EmojiError, "Failed to find the list %s", list.Describe())
		return "", false
	}
	if found {
		return listID, true
	}

	r, err := h.cf.CreateList(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
		cloudflare.ListCreateParams{
			Name:        list.Name,
			Description: fallbackDescription,
			Kind:        cloudflare.ListTypeIP,
		})
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Could not confirm that the list %s was created: %v", list.Describe(), err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listLists.Delete(list.AccountID)
		return "", false
	}

	listID = ID(r.ID)
	if ls := h.cache.listLists.Get(list.AccountID); ls != nil {
		*ls.Value() = append([]wafListMeta{{
			ID:          listID,
			Description: fallbackDescription,
			Name:        list.Name,
		}}, *ls.Value()...)
	}
	h.cache.listID.DeleteExpired()
	h.cache.listID.Set(list, listID, ttlcache.DefaultTTL)
	ppfmt.Noticef(pp.EmojiCreation, "Created a new list %s", list.Describe())
	return listID, true
}

// FinalCleanWAFList removes managed WAF content during shutdown.
//
// Whole-list ownership tries deleting the list first, then falls back to async
// item deletion. Shared ownership deletes only items selected by the handle's
// managed-item selector.
//
// We delete cached data in listListItems and listID when the underlying list
// or its managed-item view may have changed, but we keep listLists so that we
// do not have to re-query all lists under the same account.
//
// The flow is intentionally unified for both ownership modes. Whole-list mode
// invalidates managed-item cache before fallback item deletion so fallback
// never trusts outdated managed-item views.
func (h cloudflareHandle) FinalCleanWAFList(ctx context.Context, ppfmt pp.PP,
	list WAFList, fallbackDescription string, managedFamilies map[ipnet.Family]bool,
) WAFListCleanupCode {
	allFamiliesInScope := true
	for ipFamily := range ipnet.All {
		if !managedFamilies[ipFamily] {
			allFamiliesInScope = false
			break
		}
	}
	tryDeleteWholeListFirst := h.options.AllowWholeWAFListDeleteOnShutdown && allFamiliesInScope

	// Resolve list existence/ID first for both ownership modes.
	listID, found, ok := h.wafListID(ctx, ppfmt, list, fallbackDescription)
	if !ok {
		return WAFListCleanupFailed
	}
	if !found {
		if tryDeleteWholeListFirst {
			h.invalidateWAFListCleanupCache(list)
			ppfmt.Noticef(pp.EmojiWarning,
				"The list %s was not found during final cleanup; treating it as already cleaned",
				list.Describe())
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
				"Could not confirm deletion of the list %s; falling back to item deletion: %v", list.Describe(), err)
			// Ensure fallback cleanup does not trust outdated managed-item cache.
			h.cache.listListItems.Delete(list)
		}
	}

	// Shared ownership always uses managed-item cache when present.
	// Whole-list ownership reaches this block only after delete-list failure.
	// In that case, cache was just invalidated above.
	var items []WAFListItem
	var cached bool

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

	itemsToDelete := items
	if !allFamiliesInScope {
		itemsToDelete = make([]WAFListItem, 0, len(items))
		for _, item := range items {
			for ipFamily := range ipnet.All {
				if managedFamilies[ipFamily] && ipFamily.Matches(item.Prefix.Addr()) {
					itemsToDelete = append(itemsToDelete, item)
					break
				}
			}
		}
	}

	alreadyDeletedMessage := finalWAFListManagedItemsAlreadyDeletedMessage
	alreadyDeletedCachedMessage := finalWAFListManagedItemsAlreadyDeletedCachedMessage
	deleteFailedMessage := finalWAFListManagedItemsDeleteFailedMessage
	deletingMessage := finalWAFListManagedItemsDeletingMessage
	if !allFamiliesInScope {
		familiesDescription := describeInScopeWAFFamilies(managedFamilies)
		alreadyDeletedMessage = "Managed " + familiesDescription + " items in the list %s were already deleted"
		alreadyDeletedCachedMessage = "Managed " + familiesDescription + " items in the list %s were already deleted (cached)"
		deleteFailedMessage = "Could not confirm deletion of managed " + familiesDescription +
			" items in the list %s; list content may be inconsistent"
		deletingMessage = "Deleting managed " + familiesDescription + " items in the list %s asynchronously"
	}

	if len(itemsToDelete) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, alreadyDeletedCachedMessage, list.Describe())
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, alreadyDeletedMessage, list.Describe())
		}
		return WAFListCleanupNoop
	}

	ids := make([]ID, 0, len(itemsToDelete))
	for _, item := range itemsToDelete {
		ids = append(ids, item.ID)
	}
	if !h.startDeletingWAFListItemsAsync(ctx, ppfmt, list, listID, ids) {
		ppfmt.Noticef(pp.EmojiError, deleteFailedMessage, list.Describe())
		return WAFListCleanupFailed
	}

	ppfmt.Noticef(pp.EmojiClear, deletingMessage, list.Describe())
	return WAFListCleanupUpdating
}

const (
	finalWAFListManagedItemsAlreadyDeletedMessage       = "Managed items in the list %s were already deleted"
	finalWAFListManagedItemsAlreadyDeletedCachedMessage = "Managed items in the list %s were already deleted (cached)"
	finalWAFListManagedItemsDeleteFailedMessage         = "Could not confirm deletion of managed items in the list %s; " +
		"list content may be inconsistent"
	finalWAFListManagedItemsDeletingMessage = "Deleting managed items in the list %s asynchronously"
)

func describeInScopeWAFFamilies(managedFamilies map[ipnet.Family]bool) string {
	ip4InScope := managedFamilies[ipnet.IP4]
	ip6InScope := managedFamilies[ipnet.IP6]
	switch {
	case ip4InScope && ip6InScope:
		return "IPv4 and IPv6"
	case ip4InScope:
		return "IPv4"
	case ip6InScope:
		return "IPv6"
	default:
		return "no"
	}
}

func (h cloudflareHandle) invalidateWAFListCleanupCache(list WAFList) {
	h.cache.listListItems.Delete(list)
	h.cache.listID.Delete(list)
}

func (h cloudflareHandle) listWAFListItemsByID(ctx context.Context, ppfmt pp.PP,
	list WAFList, listID ID,
) ([]WAFListItem, bool) {
	rawItems, err := h.cf.ListListItems(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
		cloudflare.ListListItemsParams{
			ID:      string(listID),
			Search:  "",
			PerPage: 0,
			Cursor:  "",
		},
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

func (h cloudflareHandle) startDeletingWAFListItemsAsync(ctx context.Context, ppfmt pp.PP,
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
			"Could not confirm that item deletion started in the list %s: %v", list.Describe(), err)
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
			ppfmt.Noticef(pp.EmojiImpossible, "Found a non-IP entry in the list %s", list.Describe())
			return nil, false
		}
		p, ok := ipnet.ParseAddrOrPrefix(ppfmt, *rawItem.IP)
		if !ok {
			ppfmt.Noticef(pp.EmojiImpossible, "Found an invalid IP range or IP address %q in the list %s",
				*rawItem.IP, list.Describe())
			return nil, false
		}
		items = append(items, WAFListItem{ID: ID(rawItem.ID), Prefix: p, Comment: rawItem.Comment})
	}
	return items, true
}

func (h cloudflareHandle) filterManagedWAFListItems(items []WAFListItem) []WAFListItem {
	if h.options.ManagedWAFListItemsCommentRegex == nil {
		return items
	}

	managedItems := make([]WAFListItem, 0, len(items))
	for _, item := range items {
		if h.options.MatchManagedWAFListItemComment(item.Comment) {
			managedItems = append(managedItems, item)
		}
	}
	return managedItems
}

func (h cloudflareHandle) cacheManagedWAFListItems(list WAFList, items []WAFListItem) []WAFListItem {
	managedItems := h.filterManagedWAFListItems(items)
	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(list, &managedItems, ttlcache.DefaultTTL)
	return managedItems
}

// ListWAFListItems calls cloudflare.ListListItems, and maybe cloudflare.CreateList when needed.
// It caches one managed-item view per handle/list pair.
func (h cloudflareHandle) ListWAFListItems(ctx context.Context, ppfmt pp.PP,
	list WAFList, fallbackDescription, fallbackItemComment string,
) ([]WAFListItem, bool, bool, bool) {
	if items := h.cache.listListItems.Get(list); items != nil {
		return *items.Value(), true, true, true
	}

	listID, found, ok := h.wafListID(ctx, ppfmt, list, fallbackDescription)
	if !ok {
		// listWAFLists (called by wafListID) would have output some error messages,
		// but this provides more context.
		ppfmt.Noticef(pp.EmojiError, "Failed to check the existence of the list %s", list.Describe())
		return nil, false, false, false
	}
	if !found {
		return nil, false, false, true
	}

	items, ok := h.listWAFListItemsByID(ctx, ppfmt, list, listID)
	if !ok {
		return nil, false, false, false
	}

	managedItems := h.cacheManagedWAFListItems(list, items)
	hintMismatchedWAFListItemComment(ppfmt, list, managedItems, fallbackItemComment)
	return managedItems, true, false, true
}

// DeleteWAFListItems calls cloudflare.DeleteListItems.
func (h cloudflareHandle) DeleteWAFListItems(ctx context.Context, ppfmt pp.PP,
	list WAFList, fallbackDescription string, ids []ID,
) bool {
	if len(ids) == 0 {
		return true
	}

	beforeCommentsByID, hasBeforeComments := h.cachedManagedWAFListItemCommentsByID(list)

	listID, ok := h.findWAFList(ctx, ppfmt, list, fallbackDescription)
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
			"Could not confirm that items were deleted from the list %s: %v", list.Describe(), err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listListItems.Delete(list)
		return false
	}

	items, ok := readWAFListItems(ppfmt, list, rawItems)
	if !ok {
		return false
	}

	managedItems := h.cacheManagedWAFListItems(list, items)
	if hasBeforeComments {
		// Delete-only mutations have no "new expected comment" set. Without a
		// cached pre-mutation snapshot, every surviving managed item would look
		// suspicious here simply because we cannot prove its old comment.
		hintUnexpectedWAFListItemCommentAfterMutation(
			ppfmt,
			list,
			beforeCommentsByID,
			managedItems,
			nil,
		)
	}
	return true
}

// CreateWAFListItems calls cloudflare.CreateListItems.
func (h cloudflareHandle) CreateWAFListItems(ctx context.Context, ppfmt pp.PP,
	list WAFList, fallbackDescription string,
	itemsToCreate []WAFListCreateItem,
) bool {
	if len(itemsToCreate) == 0 {
		return true
	}

	beforeCommentsByID, hasBeforeComments := h.cachedManagedWAFListItemCommentsByID(list)

	listID, ok := h.ensureWAFList(ctx, ppfmt, list, fallbackDescription)
	if !ok {
		return false
	}

	rawItemsToCreate := make([]cloudflare.ListItemCreateRequest, 0, len(itemsToCreate))
	for _, item := range itemsToCreate {
		formattedPrefix := item.Prefix.Masked().String()
		rawItemsToCreate = append(rawItemsToCreate, cloudflare.ListItemCreateRequest{
			IP:       &formattedPrefix,
			Redirect: nil,
			Hostname: nil,
			ASN:      nil,
			Comment:  item.Comment,
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
			pp.EmojiError, "Could not confirm that items were added to the list %s: %v",
			list.Describe(), err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listListItems.Delete(list)
		return false
	}

	items, ok := readWAFListItems(ppfmt, list, rawItems)
	if !ok {
		return false
	}

	managedItems := h.cacheManagedWAFListItems(list, items)
	if hasBeforeComments {
		// Create mutations may return the whole post-mutation list, not only the
		// newly created items. Without a cached pre-mutation snapshot, older
		// managed items with different comments would be indistinguishable from
		// suspicious post-mutation drift and would trigger false warnings here.
		hintUnexpectedWAFListItemCommentAfterMutation(
			ppfmt,
			list,
			beforeCommentsByID,
			managedItems,
			allowedWAFListItemCommentsSetFromCreateItems(itemsToCreate),
		)
	}
	return true
}
