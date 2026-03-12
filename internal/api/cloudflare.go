package api

import (
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// wafListMeta contains the metadata of a list.
type wafListMeta struct {
	ID          ID
	Name        string
	Description string
}

// Keep advisory value previews short in warning logs while preserving
// full-fidelity values for mismatch diagnostics.
const advisoryValuePreviewLimit = 48

// cloudflareCache holds the previous repsonses from the Cloudflare API.
type cloudflareCache = struct {
	// domains to zone IDs
	listZones    *ttlcache.Cache[string, []zoneMeta] // zone names to their zone/account IDs
	zoneOfDomain *ttlcache.Cache[string, zoneMeta]   // domain names to their zone/account IDs
	// records of domains
	listRecords map[ipnet.Type]*ttlcache.Cache[string, *[]Record] // domain names to records.
	// lists to list IDs
	listLists *ttlcache.Cache[ID, *[]wafListMeta] // account IDs to list names to list IDs and other meta information
	listID    *ttlcache.Cache[WAFList, ID]        // lists to list IDs
	//
	// This is one managed-item view per handle/list pair.
	listListItems *ttlcache.Cache[WAFList, *[]WAFListItem] // lists to list items
}

func newCache[K comparable, V any](cacheExpiration time.Duration) *ttlcache.Cache[K, V] {
	cache := ttlcache.New(
		ttlcache.WithDisableTouchOnHit[K, V](),
		ttlcache.WithTTL[K, V](cacheExpiration),
	)

	go cache.Start()

	return cache
}

// A cloudflareHandle implements the [Handle] interface with the Cloudflare API.
type cloudflareHandle struct {
	cf      *cloudflare.API
	options HandleOptions
	cache   cloudflareCache
}

// A CloudflareAuth implements the [Auth] interface, holding the authentication data to create a [cloudflareHandle].
type CloudflareAuth struct {
	Token   string
	BaseURL string
}

// New creates a [cloudflareHandle] from the authentication data and handle options.
func (t CloudflareAuth) New(ppfmt pp.PP, options HandleOptions) (Handle, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", err)
		return nil, false
	}

	options = sanitizeHandleOptions(ppfmt, options)

	// set the base URL (mostly for testing)
	if t.BaseURL != "" {
		handle.BaseURL = t.BaseURL
	}

	h := cloudflareHandle{
		cf:      handle,
		options: options,
		cache: cloudflareCache{
			listZones:    newCache[string, []zoneMeta](options.CacheExpiration),
			zoneOfDomain: newCache[string, zoneMeta](options.CacheExpiration),
			listRecords: map[ipnet.Type]*ttlcache.Cache[string, *[]Record]{
				ipnet.IP4: newCache[string, *[]Record](options.CacheExpiration),
				ipnet.IP6: newCache[string, *[]Record](options.CacheExpiration),
			},
			listLists:     newCache[ID, *[]wafListMeta](options.CacheExpiration),
			listID:        newCache[WAFList, ID](options.CacheExpiration),
			listListItems: newCache[WAFList, *[]WAFListItem](options.CacheExpiration),
		},
	}

	return h, true
}

func sanitizeHandleOptions(ppfmt pp.PP, options HandleOptions) HandleOptions {
	if !options.AllowWholeWAFListDeleteOnShutdown {
		return options
	}

	// Whole-list final deletion is only allowed for the empty default selector.
	// A nil selector is treated as the empty default for backward compatibility.
	if options.ManagedWAFListItemsCommentRegex == nil || options.ManagedWAFListItemsCommentRegex.String() == "" {
		return options
	}

	ppfmt.Noticef(pp.EmojiUserWarning,
		"DELETE_ON_STOP is enabled, but "+
			"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX (%s) is non-empty; "+
			"the updater will keep the list and delete only items managed by this updater",
		pp.QuotePreview(options.ManagedWAFListItemsCommentRegex.String(), advisoryValuePreviewLimit),
	)
	options.AllowWholeWAFListDeleteOnShutdown = false
	return options
}

// flushCache flushes the API cache.
func (h cloudflareHandle) flushCache() {
	h.cache.listZones.DeleteAll()
	h.cache.zoneOfDomain.DeleteAll()
	for _, cache := range h.cache.listRecords {
		cache.DeleteAll()
	}
	h.cache.listLists.DeleteAll()
	h.cache.listID.DeleteAll()
	h.cache.listListItems.DeleteAll()
}

// describeFreeFormString essentially quotes a string for printing.
func describeFreeFormString(str string) string {
	return pp.QuoteOrEmptyLabel(str, "empty")
}
