package api

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// This file exposes a narrow test-only view of selected Cloudflare
// implementation details for black-box tests in package api_test.
//
// Rationale:
// - Production code should keep the concrete Cloudflare helpers private.
// - Several black-box tests live in package api_test so they can exercise the
//   package the same way external callers do.
// - Those tests also reuse mocks from internal/mocks, which already imports
//   this package. Moving the tests into package api would therefore create an
//   import cycle.
//
// The compromise is to keep the production surface small and provide only the
// minimal aliases/wrappers needed by api_test, in a *_test.go file so none of
// this is compiled into normal builds.
//
// Important boundary:
// - Do add wrappers here when a package api_test integration-style test needs
//   a narrow internal hook and cannot be moved without creating an import
//   cycle.
// - Do not add wrappers here for small white-box tests of private helpers.
//   Those tests should live in package api instead; see
//   cloudflare_internal_test.go and cloudflare_waf_internal_test.go.

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
func (h cloudflareHandle) WAFListID(ctx context.Context, ppfmt pp.PP, list WAFList, configuredDescription string) (ID, bool, bool) {
	return h.wafListID(ctx, ppfmt, list, configuredDescription)
}

// FindWAFList is a test-only wrapper around the internal list-resolution
// helper that reports a user-facing error when lookup fails.
func (h cloudflareHandle) FindWAFList(ctx context.Context, ppfmt pp.PP, list WAFList, configuredDescription string) (ID, bool) {
	return h.findWAFList(ctx, ppfmt, list, configuredDescription)
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
