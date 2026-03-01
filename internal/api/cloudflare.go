package api

import (
	"strconv"
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
	// This is one whole-list snapshot per handle/list pair. It does not preserve
	// per-item comments, so it cannot distinguish multiple ownership-filtered
	// views of the same Cloudflare list.
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
	cf    *cloudflare.API
	cache CloudflareCache
}

// A CloudflareAuth implements the [Auth] interface, holding the authentication data to create a [CloudflareHandle].
type CloudflareAuth struct {
	Token   string
	BaseURL string
}

// New creates a [CloudflareHandle] from the authentication data.
func (t CloudflareAuth) New(ppfmt pp.PP, cacheExpiration time.Duration) (Handle, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", err)
		return nil, false
	}

	// set the base URL (mostly for testing)
	if t.BaseURL != "" {
		handle.BaseURL = t.BaseURL
	}

	h := CloudflareHandle{
		cf: handle,
		cache: CloudflareCache{
			listZones:      newCache[string, []ID](cacheExpiration),
			zoneIDOfDomain: newCache[string, ID](cacheExpiration),
			listRecords: map[ipnet.Type]*ttlcache.Cache[string, *[]Record]{
				ipnet.IP4: newCache[string, *[]Record](cacheExpiration),
				ipnet.IP6: newCache[string, *[]Record](cacheExpiration),
			},
			listLists:     newCache[ID, *[]WAFListMeta](cacheExpiration),
			listID:        newCache[WAFList, ID](cacheExpiration),
			listListItems: newCache[WAFList, *[]WAFListItem](cacheExpiration),
		},
	}

	return h, true
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
	if str == "" {
		return "empty"
	}
	return strconv.Quote(str)
}
