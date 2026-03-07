package api

import (
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// WAFListMeta contains the metadata of a list.
type WAFListMeta struct {
	ID          ID
	Name        string
	Description string
}

// Keep advisory value previews short in warning logs while preserving
// full-fidelity values for mismatch diagnostics.
const advisoryValuePreviewLimit = 48

// CloudflareCache holds the previous repsonses from the Cloudflare API.
type CloudflareCache = struct {
	// domains to zone IDs
	listZones      *ttlcache.Cache[string, []ID] // zone names to zone IDs
	zoneIDOfDomain *ttlcache.Cache[string, ID]   // domain names to their zone IDs
	// records of domains
	listRecords map[ipnet.Type]*ttlcache.Cache[string, *[]Record] // domain names to records.
	// lists to list IDs
	listLists *ttlcache.Cache[ID, *[]WAFListMeta] // account IDs to list names to list IDs and other meta information
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

// A CloudflareHandle implements the [Handle] interface with the Cloudflare API.
type CloudflareHandle struct {
	cf      *cloudflare.API
	options HandleOptions
	cache   CloudflareCache
}

// A CloudflareAuth implements the [Auth] interface, holding the authentication data to create a [CloudflareHandle].
type CloudflareAuth struct {
	Token   string
	BaseURL string
}

// New creates a [CloudflareHandle] from the authentication data and handle options.
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

	h := CloudflareHandle{
		cf:      handle,
		options: options,
		cache: CloudflareCache{
			listZones:      newCache[string, []ID](options.CacheExpiration),
			zoneIDOfDomain: newCache[string, ID](options.CacheExpiration),
			listRecords: map[ipnet.Type]*ttlcache.Cache[string, *[]Record]{
				ipnet.IP4: newCache[string, *[]Record](options.CacheExpiration),
				ipnet.IP6: newCache[string, *[]Record](options.CacheExpiration),
			},
			listLists:     newCache[ID, *[]WAFListMeta](options.CacheExpiration),
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

// FlushCache flushes the API cache.
func (h CloudflareHandle) FlushCache() {
	h.cache.listZones.DeleteAll()
	h.cache.zoneIDOfDomain.DeleteAll()
	for _, cache := range h.cache.listRecords {
		cache.DeleteAll()
	}
	h.cache.listLists.DeleteAll()
	h.cache.listID.DeleteAll()
	h.cache.listListItems.DeleteAll()
}

// DescribeFreeFormString essentially quotes a string for printing.
func DescribeFreeFormString(str string) string {
	return pp.QuoteOrEmptyLabel(str, "empty")
}
