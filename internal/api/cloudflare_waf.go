package api

import (
	"context"
	"errors"
	"net/netip"

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
	ipnet.IP4: 32, //nolint:mnd
	ipnet.IP6: 64, //nolint:mnd
}

func hintWAFListPermission(ppfmt pp.PP, err error) {
	var authentication *cloudflare.AuthenticationError
	var authorization *cloudflare.AuthorizationError
	if errors.As(err, &authentication) || errors.As(err, &authorization) {
		ppfmt.Hintf(pp.HintWAFListPermission,
			"Double check your API token and account ID. "+
				`Make sure you granted the "Edit" permission of "Account - Account Filter Lists"`)
	}
}

// ListWAFLists lists all IP lists of the given name.
func (h CloudflareHandle) ListWAFLists(ctx context.Context, ppfmt pp.PP, accountID ID) (map[string]ID, bool) {
	if lmap := h.cache.listLists.Get(accountID); lmap != nil {
		return lmap.Value(), true
	}

	ls, err := h.cf.ListLists(ctx, cloudflare.AccountIdentifier(string(accountID)), cloudflare.ListListsParams{})
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to list existing lists: %v", err)
		hintWAFListPermission(ppfmt, err)
		return nil, false
	}

	lmap := map[string]ID{}
	for _, l := range ls {
		if l.Kind == cloudflare.ListTypeIP {
			if anotherListID, conflicting := lmap[l.Name]; conflicting {
				ppfmt.Noticef(pp.EmojiImpossible,
					"Found multiple lists named %q (IDs: %s and %s); please report this at %s",
					l.Name, anotherListID, ID(l.ID), pp.IssueReportingURL)
				return nil, false
			}

			lmap[l.Name] = ID(l.ID)
		}
	}

	h.cache.listLists.DeleteExpired()
	h.cache.listLists.Set(accountID, lmap, ttlcache.DefaultTTL)

	return lmap, true
}

// FindWAFList returns the ID of the IP list with the given name.
func (h CloudflareHandle) FindWAFList(ctx context.Context, ppfmt pp.PP, l WAFList) (ID, bool) {
	listMap, ok := h.ListWAFLists(ctx, ppfmt, l.AccountID)
	if !ok {
		// ListWAFLists would have output some error messages, but this provides more context.
		ppfmt.Noticef(pp.EmojiError, "Failed to find the list %q", l.ListName)
		return "", false
	}

	listID, ok := listMap[l.ListName]
	if !ok {
		ppfmt.Noticef(pp.EmojiError, "Failed to find the list %q", l.ListName)
		return "", false
	}

	return listID, true
}

// EnsureWAFList calls cloudflare.CreateList when the list does not already exist.
func (h CloudflareHandle) EnsureWAFList(ctx context.Context, ppfmt pp.PP, l WAFList, description string,
) (ID, bool, bool) {
	listMap, ok := h.ListWAFLists(ctx, ppfmt, l.AccountID)
	if !ok {
		// ListWAFLists would have output some error messages, but this provides more context.
		ppfmt.Noticef(pp.EmojiError, "Failed to check the existence of the list %q", l.ListName)
		return "", false, false
	}

	listID, existing := listMap[l.ListName]
	if existing {
		return listID, existing, true
	}

	r, err := h.cf.CreateList(ctx, cloudflare.AccountIdentifier(string(l.AccountID)),
		cloudflare.ListCreateParams{
			Name:        l.ListName,
			Description: description,
			Kind:        cloudflare.ListTypeIP,
		})
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to create a list named %q: %v", l.ListName, err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listLists.Delete(l.AccountID)
		return "", false, false
	}

	if lmap := h.cache.listLists.Get(l.AccountID); lmap != nil {
		lmap.Value()[l.ListName] = ID(r.ID)
	}

	return ID(r.ID), false, true
}

// ClearWAFListAsync calls cloudflare.DeleteList and cloudflare.ReplaceListItemsAsync.
// Note: a failure will not clear the cache, assuming that the list deletion/cleaning only happens
// right before exiting the updater.
func (h CloudflareHandle) ClearWAFListAsync(ctx context.Context, ppfmt pp.PP, l WAFList, keepCacheWhenFails bool,
) (bool, bool) {
	listID, ok := h.FindWAFList(ctx, ppfmt, l)
	if !ok {
		return false, false
	}

	if _, err := h.cf.DeleteList(ctx, cloudflare.AccountIdentifier(string(l.AccountID)), string(listID)); err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to delete the list %q (ID: %s); clearing it instead: %v",
			l.ListName, listID, err)
		_, err := h.cf.ReplaceListItemsAsync(ctx, cloudflare.AccountIdentifier(string(l.AccountID)),
			cloudflare.ListReplaceItemsParams{
				ID:    string(listID),
				Items: []cloudflare.ListItemCreateRequest{},
			},
		)
		if err != nil {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to start clearing the list %q (ID: %s): %v", l.ListName, listID, err)
			hintWAFListPermission(ppfmt, err)

			if !keepCacheWhenFails {
				h.cache.listLists.Delete(l.AccountID)
				h.cache.listListItems.Delete(globalListID{l.AccountID, listID})
			}
			return false, false
		}

		h.cache.listListItems.Delete(globalListID{l.AccountID, listID})
		return false, true
	}

	if lmap := h.cache.listLists.Get(l.AccountID); lmap != nil {
		delete(lmap.Value(), l.ListName)
	}
	h.cache.listListItems.Delete(globalListID{l.AccountID, listID})
	return true, true
}

func readWAFListItems(ppfmt pp.PP, listName string, listID ID, rawItems []cloudflare.ListItem) ([]WAFListItem, bool) {
	items := make([]WAFListItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		if rawItem.IP == nil {
			ppfmt.Noticef(pp.EmojiImpossible, "Found a non-IP in the list %q (ID: %s)", listName, listID)
			return nil, false
		}
		p, ok := ipnet.ParsePrefixOrIP(ppfmt, *rawItem.IP)
		if !ok {
			ppfmt.Noticef(pp.EmojiImpossible, "Found an invalid IP range/address %q in the list %q (ID: %s)",
				*rawItem.IP, listName, listID)
			return nil, false
		}
		items = append(items, WAFListItem{ID: ID(rawItem.ID), Prefix: p})
	}
	return items, true
}

// ListWAFListItems calls cloudflare.ListListItems.
func (h CloudflareHandle) ListWAFListItems(ctx context.Context, ppfmt pp.PP, l WAFList) ([]WAFListItem, bool, bool) {
	listID, ok := h.FindWAFList(ctx, ppfmt, l)
	if !ok {
		return nil, false, false
	}

	if items := h.cache.listListItems.Get(globalListID{l.AccountID, listID}); items != nil {
		return items.Value(), true, true
	}

	rawItems, err := h.cf.ListListItems(ctx, cloudflare.AccountIdentifier(string(l.AccountID)),
		cloudflare.ListListItemsParams{ID: string(listID)}, //nolint:exhaustruct
	)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to retrieve items in the list %q (ID: %s): %v", l.ListName, listID, err)
		hintWAFListPermission(ppfmt, err)
		return nil, false, false
	}

	items, ok := readWAFListItems(ppfmt, l.ListName, listID, rawItems)
	if !ok {
		return nil, false, false
	}

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(globalListID{l.AccountID, listID}, items, ttlcache.DefaultTTL)

	return items, false, true
}

// DeleteWAFListItems calls cloudflare.DeleteListItems.
func (h CloudflareHandle) DeleteWAFListItems(ctx context.Context, ppfmt pp.PP, l WAFList, ids []ID) bool {
	if len(ids) == 0 {
		return true
	}

	listID, ok := h.FindWAFList(ctx, ppfmt, l)
	if !ok {
		return false
	}

	itemRequests := make([]cloudflare.ListItemDeleteItemRequest, 0, len(ids))
	for _, id := range ids {
		itemRequests = append(itemRequests, cloudflare.ListItemDeleteItemRequest{ID: string(id)})
	}

	rawItems, err := h.cf.DeleteListItems(ctx, cloudflare.AccountIdentifier(string(l.AccountID)),
		cloudflare.ListDeleteItemsParams{
			ID:    string(listID),
			Items: cloudflare.ListItemDeleteRequest{Items: itemRequests},
		},
	)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to finish deleting items from the list %q (ID: %s): %v", l.ListName, listID, err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listListItems.Delete(globalListID{l.AccountID, listID})
		return false
	}

	items, ok := readWAFListItems(ppfmt, l.ListName, listID, rawItems)
	if !ok {
		return false
	}

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(globalListID{l.AccountID, listID}, items, ttlcache.DefaultTTL)

	return true
}

// CreateWAFListItems calls cloudflare.CreateListItems.
func (h CloudflareHandle) CreateWAFListItems(ctx context.Context, ppfmt pp.PP,
	l WAFList, itemsToCreate []netip.Prefix, comment string,
) bool {
	if len(itemsToCreate) == 0 {
		return true
	}

	listID, ok := h.FindWAFList(ctx, ppfmt, l)
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

	rawItems, err := h.cf.CreateListItems(ctx, cloudflare.AccountIdentifier(string(l.AccountID)),
		cloudflare.ListCreateItemsParams{
			ID:    string(listID),
			Items: rawItemsToCreate,
		},
	)
	if err != nil {
		ppfmt.Noticef(
			pp.EmojiError, "Failed to finish adding items to the list %q (ID: %s): %v",
			l.ListName, listID, err)
		hintWAFListPermission(ppfmt, err)
		h.cache.listListItems.Delete(globalListID{l.AccountID, listID})
		return false
	}

	items, ok := readWAFListItems(ppfmt, l.ListName, listID, rawItems)
	if !ok {
		return false
	}

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(globalListID{l.AccountID, listID}, items, ttlcache.DefaultTTL)

	return true
}
