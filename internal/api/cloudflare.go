package api

import (
	"context"
	"errors"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

const authVerifyTimeout = time.Second

// wafListMeta contains the metadata of a list.
type wafListMeta struct {
	ID          ID
	Name        string
	Description string
}

// cloudflareCache holds the previous repsonses from the Cloudflare API.
type cloudflareCache = struct {
	// domains to zone IDs
	listZones    *ttlcache.Cache[string, []zoneMeta] // zone names to their zone/account IDs
	zoneOfDomain *ttlcache.Cache[string, zoneMeta]   // domain names to their zone/account IDs
	// records of domains
	listRecords map[ipnet.Family]*ttlcache.Cache[string, *[]Record] // domain names to records.
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
	handle, ok := t.newClient(ppfmt)
	if !ok {
		return nil, false
	}

	options.HandleOwnershipPolicy = options.Sanitize(ppfmt)

	h := cloudflareHandle{
		cf:      handle,
		options: options,
		cache: cloudflareCache{
			listZones:    newCache[string, []zoneMeta](options.CacheExpiration),
			zoneOfDomain: newCache[string, zoneMeta](options.CacheExpiration),
			listRecords: map[ipnet.Family]*ttlcache.Cache[string, *[]Record]{
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

// CheckUsability performs an early token check so obviously bad credentials
// fail with a targeted message, while ambiguous network failures still allow
// startup.
//
// The return contract is intentionally boolean because the caller only needs to
// know whether startup must stop. This method keeps the finer Cloudflare-
// specific classification local and reports the operator-facing details here.
func (t CloudflareAuth) CheckUsability(ctx context.Context, ppfmt pp.PP) bool {
	handle, ok := t.newClient(ppfmt)
	if !ok {
		return false
	}

	quickCtx, cancel := context.WithTimeout(ctx, authVerifyTimeout)
	defer cancel()

	res, err := handle.VerifyAPIToken(quickCtx)
	if err != nil {
		var authorizationError *cloudflare.AuthorizationError
		var authenticationError *cloudflare.AuthenticationError
		var requestError *cloudflare.RequestError
		// Startup verification intentionally classifies only evidence-backed
		// "broken token" cases as fatal.
		//
		// The expected snapshot for this observed contract was adopted on
		// 2026-03-22. Update that date only when the contract probe in
		// scripts/github-actions/cloudflare-verify-contract/config/verify-contract.json
		// changes the expected behavior. The contract probe currently observes:
		// - 400 for malformed or missing Authorization headers
		// - 401 for well-formed but invalid bearer tokens
		// cloudflare-go maps those to RequestError and AuthorizationError.
		//
		// Everything else is treated as non-fatal unless Cloudflare explicitly
		// says the token status is bad below. This keeps temporary outages and
		// undocumented server behavior from being mislabeled as misconfiguration.
		if errors.As(err, &authorizationError) || errors.As(err, &requestError) {
			ppfmt.Noticef(pp.EmojiUserError, "The Cloudflare API token appears to be invalid: %v", err)
			ppfmt.Noticef(pp.EmojiUserError,
				"Please double-check the value of CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_TOKEN_FILE")
			return false
		}
		// cloudflare-go reserves AuthenticationError for HTTP 403. We did not
		// find a documented or observed 403 variant for /user/tokens/verify, so
		// keep this as a defensive unexpected-response branch instead of claiming
		// the token is definitely invalid.
		if errors.As(err, &authenticationError) {
			ppfmt.Noticef(pp.EmojiWarning,
				"Unexpected authorization failure while verifying the Cloudflare API token: %v; the updater will continue",
				err)
			return true
		}
		// This startup probe intentionally uses a short fixed budget. If the
		// probe context is already done here, treat the result as ambiguous and
		// continue startup instead of mislabeling the token as invalid.
		if quickCtx.Err() != nil {
			ppfmt.Noticef(pp.EmojiWarning,
				"Cloudflare API token verification timed out after %v; the updater will continue",
				authVerifyTimeout)
			return true
		}

		// Other errors here are usually transport failures, timeouts, or future
		// undocumented client-library/server behavior. Keep startup running and
		// let later operations provide more context if the problem persists.
		ppfmt.Noticef(pp.EmojiWarning,
			"Cloudflare API token verification failed: %v; the updater will continue", err)
		return true
	}

	switch res.Status {
	case "active":
		return true
	case "disabled", "expired":
		// These statuses come from Cloudflare's success response and are safe to
		// classify as fatal startup configuration errors.
		ppfmt.Noticef(pp.EmojiUserError, "The Cloudflare API token is %s", res.Status)
		ppfmt.Noticef(pp.EmojiUserError,
			"Please double-check the value of CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_TOKEN_FILE")
		return false
	default:
		// Cloudflare documents "active", and we have seen "disabled" and
		// "expired" in the client contract. Anything else should be preserved as
		// an unexpected-but-non-fatal warning until there is real evidence for a
		// stricter interpretation.
		ppfmt.Noticef(pp.EmojiWarning,
			"Cloudflare reported the API token status as %q during startup verification; the updater will continue", res.Status)
		return true
	}
}

func (t CloudflareAuth) newClient(ppfmt pp.PP) (*cloudflare.API, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", err)
		return nil, false
	}

	// set the base URL (mostly for testing)
	if t.BaseURL != "" {
		handle.BaseURL = t.BaseURL
	}

	return handle, true
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
