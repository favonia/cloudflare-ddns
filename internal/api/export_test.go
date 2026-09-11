// This file exposes Cloudflare lookup and cache operations to api_test while
// keeping the concrete handle private in production. The external tests use
// the shared mocks package, which imports api; moving them into package api
// would create an import cycle.
//
// SDK options let those tests use in-memory HTTP servers with normal handle
// construction. StopCaches lets their harness clean up the background cache
// tasks created by that construction before a synctest bubble exits.

package api

import (
	"context"

	"github.com/cloudflare/cloudflare-go"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// CloudflareHandle is a test-only alias for the concrete Cloudflare-backed
// handle. External tests use it for type assertions when they need to verify
// cache and lookup behavior that is intentionally outside the public Handle
// interface.
type CloudflareHandle = cloudflareHandle

// WAFListMeta is a test-only alias for list lookup metadata returned by the
// internal list-discovery helper.
type WAFListMeta = wafListMeta

// FlushCache clears all Cloudflare API caches in tests so cache-hit and
// cache-miss scenarios can be exercised deterministically.
func (h cloudflareHandle) FlushCache() {
	h.flushCache()
}

// ListWAFLists is a test-only wrapper around the internal list enumeration
// helper.
func (h cloudflareHandle) ListWAFLists(ctx context.Context, ppfmt pp.PP, accountID ID) ([]WAFListMeta, bool) {
	return h.listWAFLists(ctx, ppfmt, accountID)
}

// WAFListID is a test-only wrapper around the internal list-ID lookup helper.
func (h cloudflareHandle) WAFListID(ctx context.Context, ppfmt pp.PP, list WAFList, fallbackDescription string) (ID, bool, bool) {
	return h.wafListID(ctx, ppfmt, list, fallbackDescription)
}

// FindWAFList is a test-only wrapper around the internal list-resolution
// helper that reports a user-facing error when lookup fails.
func (h cloudflareHandle) FindWAFList(ctx context.Context, ppfmt pp.PP, list WAFList, fallbackDescription string) (ID, bool) {
	return h.findWAFList(ctx, ppfmt, list, fallbackDescription)
}

// ListZones is a test-only wrapper around the zone-enumeration helper.
func (h cloudflareHandle) ListZones(ctx context.Context, ppfmt pp.PP, name string) ([]ID, bool) {
	return h.listZones(ctx, ppfmt, name)
}

// ZoneIDOfDomain is a test-only wrapper around the zone-resolution helper used
// by the DNS record code paths.
func (h cloudflareHandle) ZoneIDOfDomain(ctx context.Context, ppfmt pp.PP, domain domain.Domain) (ID, bool) {
	return h.zoneIDOfDomain(ctx, ppfmt, domain)
}

// NewWithSDKOptions constructs a handle and its caches with the supplied SDK options.
func (t CloudflareAuth) NewWithSDKOptions(
	ppfmt pp.PP, handleOptions HandleOptions, sdkOptions ...cloudflare.Option,
) (Handle, bool) {
	return t.newWithSDKOptions(ppfmt, handleOptions, sdkOptions...)
}

// StopCaches stops the handle's background cache cleanup tasks and waits for them
// to exit.
func (h cloudflareHandle) StopCaches() {
	h.cache.listZones.Stop()
	h.cache.zoneOfDomain.Stop()
	for _, cache := range h.cache.listRecords {
		cache.Stop()
	}
	h.cache.listLists.Stop()
	h.cache.listID.Stop()
	h.cache.listListItems.Stop()
}
