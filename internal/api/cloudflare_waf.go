package api

import (
	"context"
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

// ListWAFLists lists all IP lists of the given name.
func (h CloudflareHandle) ListWAFLists(ctx context.Context, ppfmt pp.PP, listName string) ([]string, bool) {
	if lmap := h.cache.listLists.Get(struct{}{}); lmap != nil {
		return lmap.Value()[listName], true
	}

	ls, err := h.cf.ListLists(ctx, cloudflare.AccountIdentifier(h.accountID), cloudflare.ListListsParams{})
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to list existing lists: %v", err)
		return nil, false
	}
	h.forcePassSanityCheck()

	lmap := make(map[string][]string)
	for _, l := range ls {
		if l.Kind == cloudflare.ListTypeIP {
			lmap[l.Name] = append(lmap[l.Name], l.ID)
		}
	}

	h.cache.listLists.DeleteExpired()
	h.cache.listLists.Set(struct{}{}, lmap, ttlcache.DefaultTTL)

	return lmap[listName], true
}

// FindWAFList returns the ID of the IP list with the given name.
func (h CloudflareHandle) FindWAFList(ctx context.Context, ppfmt pp.PP, listName string) (string, bool) {
	listIDs, ok := h.ListWAFLists(ctx, ppfmt, listName)

	switch {
	case !ok:
		// ListWAFLists would have output some error messages, but this provides
		// more context.
		ppfmt.Warningf(pp.EmojiError, "Failed to find the list %q", listName)
		return "", false
	case len(listIDs) == 0:
		ppfmt.Warningf(pp.EmojiError, "Failed to find the list %q", listName)
		return "", false
	case len(listIDs) == 1:
		return listIDs[0], true
	default: // len(listIDs) > 1
		ppfmt.Warningf(pp.EmojiImpossible,
			"Found multiple lists named %q; please report this at "+
				"https://github.com/favonia/cloudflare-ddns/issues/new",
			listName)
		return "", false
	}
}

// EnsureWAFList calls cloudflare.CreateList when the list does not already exist.
func (h CloudflareHandle) EnsureWAFList(ctx context.Context, ppfmt pp.PP,
	listName string, description string,
) (bool, bool) {
	switch listIDs, ok := h.ListWAFLists(ctx, ppfmt, listName); {
	case !ok:
		return false, false
	case len(listIDs) >= 1:
		return true, true
	}

	r, err := h.cf.CreateList(ctx, cloudflare.AccountIdentifier(h.accountID),
		cloudflare.ListCreateParams{
			Name:        listName,
			Description: description,
			Kind:        cloudflare.ListTypeIP,
		})
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to create a list named %q: %v", listName, err)
		h.cache.listLists.Delete(struct{}{})
		return false, false
	}
	h.forcePassSanityCheck()

	if lmap := h.cache.listLists.Get(struct{}{}); lmap != nil {
		lmap.Value()[listName] = append(lmap.Value()[listName], r.ID)
	}

	return false, true
}

// DeleteWAFList calls cloudflare.DeleteList.
func (h CloudflareHandle) DeleteWAFList(ctx context.Context, ppfmt pp.PP, listName string) bool {
	listID, ok := h.FindWAFList(ctx, ppfmt, listName)
	if !ok {
		return false
	}

	if _, err := h.cf.DeleteList(ctx, cloudflare.AccountIdentifier(h.accountID), listID); err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to delete the list %q: %v", listName, err)
		h.cache.listLists.Delete(struct{}{})
		return false
	}
	h.forcePassSanityCheck()

	if lmap := h.cache.listLists.Get(struct{}{}); lmap != nil {
		delete(lmap.Value(), listName)
	}

	return true
}

func readWAFListItems(ppfmt pp.PP, listName, listID string, rawItems []cloudflare.ListItem) ([]WAFListItem, bool) {
	items := make([]WAFListItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		if rawItem.IP == nil {
			ppfmt.Warningf(pp.EmojiImpossible, "Found a non-IP in the list %q (ID: %s)", listName, listID)
			return nil, false
		}
		p, ok := ipnet.ParsePrefixOrIP(ppfmt, *rawItem.IP)
		if !ok {
			ppfmt.Warningf(pp.EmojiImpossible, "Found an invalid IP range/address %q in the list %q (ID: %s)",
				*rawItem.IP, listName, listID)
			return nil, false
		}
		items = append(items, WAFListItem{ID: rawItem.ID, Prefix: p})
	}
	return items, true
}

// ListWAFListItems calls cloudflare.ListListItems.
func (h CloudflareHandle) ListWAFListItems(ctx context.Context, ppfmt pp.PP, listName string) ([]WAFListItem, bool, bool) { //nolint:lll
	listID, ok := h.FindWAFList(ctx, ppfmt, listName)
	if !ok {
		return nil, false, false
	}

	if items := h.cache.listListItems.Get(listID); items != nil {
		return items.Value(), true, true
	}

	rawItems, err := h.cf.ListListItems(ctx, cloudflare.AccountIdentifier(h.accountID),
		cloudflare.ListListItemsParams{ID: listID}, //nolint:exhaustruct
	)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to retrieve items in the list %q (ID: %s): %v", listName, listID, err)
		return nil, false, false
	}

	h.forcePassSanityCheck()

	items, ok := readWAFListItems(ppfmt, listName, listID, rawItems)
	if !ok {
		return nil, false, false
	}

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(listID, items, ttlcache.DefaultTTL)

	return items, false, true
}

// DeleteWAFListItems calls cloudflare.DeleteListItems.
func (h CloudflareHandle) DeleteWAFListItems(ctx context.Context, ppfmt pp.PP, listName string, ids []string) bool {
	if len(ids) == 0 {
		return true
	}

	listID, ok := h.FindWAFList(ctx, ppfmt, listName)
	if !ok {
		return false
	}

	itemRequests := make([]cloudflare.ListItemDeleteItemRequest, 0, len(ids))
	for _, id := range ids {
		itemRequests = append(itemRequests, cloudflare.ListItemDeleteItemRequest{ID: id})
	}

	rawItems, err := h.cf.DeleteListItems(ctx, cloudflare.AccountIdentifier(h.accountID),
		cloudflare.ListDeleteItemsParams{
			ID:    listID,
			Items: cloudflare.ListItemDeleteRequest{Items: itemRequests},
		},
	)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to finish deleting items from the list %q (ID: %s): %v", listName, listID, err)
		h.cache.listListItems.Delete(listID)
		return false
	}

	h.forcePassSanityCheck()

	items, ok := readWAFListItems(ppfmt, listName, listID, rawItems)
	if !ok {
		return false
	}

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(listID, items, ttlcache.DefaultTTL)

	return true
}

// CreateWAFListItems calls cloudflare.CreateListItems.
func (h CloudflareHandle) CreateWAFListItems(ctx context.Context, ppfmt pp.PP,
	listName string, itemsToCreate []netip.Prefix, comment string,
) bool {
	if len(itemsToCreate) == 0 {
		return true
	}

	listID, ok := h.FindWAFList(ctx, ppfmt, listName)
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

	rawItems, err := h.cf.CreateListItems(ctx, cloudflare.AccountIdentifier(h.accountID),
		cloudflare.ListCreateItemsParams{
			ID:    listID,
			Items: rawItemsToCreate,
		},
	)
	if err != nil {
		ppfmt.Warningf(
			pp.EmojiError, "Failed to finish adding items to the list %q (ID: %s): %v",
			listName, listID, err)
		h.cache.listListItems.Delete(listID)
		return false
	}

	h.forcePassSanityCheck()

	items, ok := readWAFListItems(ppfmt, listName, listID, rawItems)
	if !ok {
		return false
	}

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(listID, items, ttlcache.DefaultTTL)

	return true
}
