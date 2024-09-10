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

func hintMismatchedWAFListAttributes(ppfmt pp.PP, accountID ID) {
	ppfmt.Hintf(pp.HintMismatchedWAFListAttributes,
		"The updater will not overwrite WAF list descriptions; "+
			"you can change them at https://dash.cloudflare.com/%s/configurations/lists",
		accountID,
	)
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
					"Found multiple lists named %q (IDs: %s and %s); please report this at %s",
					l.Name, listID, l.ID, pp.IssueReportingURL,
				)
				return "", false, false
			}

			if l.Description != expectedDescription {
				ppfmt.Infof(pp.EmojiUserWarning,
					"The description of the list %s (ID: %s) differs from the value of WAF_LIST_DESCRIPTION (%q)",
					list.Describe(), l.ID, expectedDescription,
				)
				hintMismatchedWAFListAttributes(ppfmt, list.AccountID)
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

// EnsureWAFList calls cloudflare.CreateList when the list does not already exist.
func (h CloudflareHandle) EnsureWAFList(ctx context.Context, ppfmt pp.PP, list WAFList,
	expectedDescription string,
) (ID, bool, bool) {
	listID, found, ok := h.WAFListID(ctx, ppfmt, list, expectedDescription)
	if !ok {
		// ListWAFLists (called by WAFListID) would have output some error messages,
		// but this provides more context.
		ppfmt.Noticef(pp.EmojiError, "Failed to check the existence of the list %s", list.Describe())
		return "", false, false
	}

	if found {
		return listID, found, true
	}

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
		return "", false, false
	}

	listID = ID(r.ID)
	if ls := h.cache.listLists.Get(list.AccountID); ls != nil {
		*ls.Value() = append([]WAFListMeta{{ID: listID, Description: expectedDescription, Name: list.Name}}, *ls.Value()...)
	}

	h.cache.listID.DeleteExpired()
	h.cache.listID.Set(list, listID, ttlcache.DefaultTTL)
	return listID, false, true
}

// FinalClearWAFListAsync calls cloudflare.DeleteList and cloudflare.ReplaceListItemsAsync.
//
// We only deleted cached data in listListItems and listID, but not the cached lists
// in listLists so that we do not have to re-query the lists under the same account.
// Managing multiple lists under the same account makes little sense in practice,
// but the tool should still do the right thing even under rare circumstances.
func (h CloudflareHandle) FinalClearWAFListAsync(ctx context.Context, ppfmt pp.PP,
	list WAFList, expectedDescription string,
) (bool, bool) {
	listID, ok := h.FindWAFList(ctx, ppfmt, list, expectedDescription)
	if !ok {
		return false, false
	}

	if _, err := h.cf.DeleteList(ctx, cloudflare.AccountIdentifier(string(list.AccountID)), string(listID)); err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to delete the list %s; clearing it instead: %v",
			list.Describe(), err)
		_, err := h.cf.ReplaceListItemsAsync(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
			cloudflare.ListReplaceItemsParams{
				ID:    string(listID),
				Items: []cloudflare.ListItemCreateRequest{},
			},
		)
		if err != nil {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to start clearing the list %s: %v", list.Describe(), err)
			hintWAFListPermission(ppfmt, err)

			h.cache.listListItems.Delete(list)
			h.cache.listID.Delete(list)
			return false, false
		}

		h.cache.listListItems.Delete(list)
		h.cache.listID.Delete(list)
		return false, true
	}

	h.cache.listListItems.Delete(list)
	h.cache.listID.Delete(list)
	return true, true
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
		items = append(items, WAFListItem{ID: ID(rawItem.ID), Prefix: p})
	}
	return items, true
}

// ListWAFListItems calls cloudflare.ListListItems.
func (h CloudflareHandle) ListWAFListItems(ctx context.Context, ppfmt pp.PP,
	list WAFList, expectedDescription string,
) ([]WAFListItem, bool, bool) {
	if items := h.cache.listListItems.Get(list); items != nil {
		return *items.Value(), true, true
	}

	listID, alreadyExisting, ok := h.EnsureWAFList(ctx, ppfmt, list, expectedDescription)
	if !ok {
		return nil, false, false
	}
	if !alreadyExisting {
		ppfmt.Noticef(pp.EmojiCreation, "Created a new list %s", list.Describe())
	}

	rawItems, err := h.cf.ListListItems(ctx, cloudflare.AccountIdentifier(string(list.AccountID)),
		cloudflare.ListListItemsParams{ID: string(listID)}, //nolint:exhaustruct
	)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to retrieve items in the list %s: %v", list.Describe(), err)
		hintWAFListPermission(ppfmt, err)
		return nil, false, false
	}

	items, ok := readWAFListItems(ppfmt, list, rawItems)
	if !ok {
		return nil, false, false
	}

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(list, &items, ttlcache.DefaultTTL)
	return items, false, true
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

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(list, &items, ttlcache.DefaultTTL)
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

	h.cache.listListItems.DeleteExpired()
	h.cache.listListItems.Set(list, &items, ttlcache.DefaultTTL)
	return true
}
