package api

import (
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// globalListID pairs up an account ID and a list ID.
type globalListID struct {
	Account ID
	List    ID
}

// CloudflareCache holds the previous repsonses from the Cloudflare API.
type CloudflareCache = struct {
	// domains to zones
	listZones    *ttlcache.Cache[string, []ID] // zone names to zone IDs
	zoneOfDomain *ttlcache.Cache[string, ID]   // domains to their zone IDs
	// records of domains
	listRecords map[ipnet.Type]*ttlcache.Cache[string, *[]Record] // domains to records.
	// lists
	listLists     *ttlcache.Cache[ID, map[string]ID]           // account IDs to list names to list IDs
	listListItems *ttlcache.Cache[globalListID, []WAFListItem] // list IDs to list items
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
			listZones:    newCache[string, []ID](cacheExpiration),
			zoneOfDomain: newCache[string, ID](cacheExpiration),
			listRecords: map[ipnet.Type]*ttlcache.Cache[string, *[]Record]{
				ipnet.IP4: newCache[string, *[]Record](cacheExpiration),
				ipnet.IP6: newCache[string, *[]Record](cacheExpiration),
			},
			listLists:     newCache[ID, map[string]ID](cacheExpiration),
			listListItems: newCache[globalListID, []WAFListItem](cacheExpiration),
		},
	}

	return h, true
}

// FlushCache flushes the API cache.
func (h CloudflareHandle) FlushCache() {
	h.cache.listZones.DeleteAll()
	h.cache.zoneOfDomain.DeleteAll()
	for _, cache := range h.cache.listRecords {
		cache.DeleteAll()
	}
	h.cache.listLists.DeleteAll()
	h.cache.listListItems.DeleteAll()
}
