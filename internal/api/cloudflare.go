package api

import (
	"context"
	"errors"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type sanityCheckType int

const (
	sanityCheckToken sanityCheckType = iota
	sanityCheckAccount
)

// CloudflareCache holds the previous repsonses from the Cloudflare API.
type CloudflareCache = struct {
	// sanity check
	sanityCheck *ttlcache.Cache[sanityCheckType, bool] // whether token or account is valid
	// domains to zones
	listZones    *ttlcache.Cache[string, []string] // zone names to zone IDs
	zoneOfDomain *ttlcache.Cache[string, string]   // domain names to the zone ID
	// records of domains
	listRecords map[ipnet.Type]*ttlcache.Cache[string, *[]Record] // domain names to records.
	// lists
	listLists     *ttlcache.Cache[struct{}, map[string]string] // list names to list IDs
	listListItems *ttlcache.Cache[string, []WAFListItem]       // list IDs to list items
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
	cf        *cloudflare.API
	accountID string
	cache     CloudflareCache
}

// A CloudflareAuth implements the [Auth] interface, holding the authentication data to create a [CloudflareHandle].
type CloudflareAuth struct {
	Token     string
	AccountID string
	BaseURL   string
}

// New creates a [CloudflareHandle] from the authentication data.
func (t CloudflareAuth) New(ppfmt pp.PP, cacheExpiration time.Duration) (Handle, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", err)
		return nil, false
	}

	// set the base URL (mostly for testing)
	if t.BaseURL != "" {
		handle.BaseURL = t.BaseURL
	}

	h := CloudflareHandle{
		cf:        handle,
		accountID: t.AccountID,
		cache: CloudflareCache{
			sanityCheck:  newCache[sanityCheckType, bool](cacheExpiration),
			listZones:    newCache[string, []string](cacheExpiration),
			zoneOfDomain: newCache[string, string](cacheExpiration),
			listRecords: map[ipnet.Type]*ttlcache.Cache[string, *[]Record]{
				ipnet.IP4: newCache[string, *[]Record](cacheExpiration),
				ipnet.IP6: newCache[string, *[]Record](cacheExpiration),
			},
			listLists:     newCache[struct{}, map[string]string](cacheExpiration),
			listListItems: newCache[string, []WAFListItem](cacheExpiration),
		},
	}

	return h, true
}

// SupportsRecords checks whether it's good for DNS records.
func (t CloudflareAuth) SupportsRecords() bool {
	return t.Token != ""
}

// SupportsWAFLists checks whether it's good for DNS records.
func (t CloudflareAuth) SupportsWAFLists() bool {
	return t.Token != "" && t.AccountID != ""
}

// FlushCache flushes the API cache.
func (h CloudflareHandle) FlushCache() {
	h.cache.sanityCheck.DeleteAll()
	h.cache.listZones.DeleteAll()
	h.cache.zoneOfDomain.DeleteAll()
	for _, cache := range h.cache.listRecords {
		cache.DeleteAll()
	}
	h.cache.listLists.DeleteAll()
	h.cache.listListItems.DeleteAll()
}

// errTimeout for checking if it's timeout.
var errTimeout = errors.New("timeout")

func (h CloudflareHandle) skipSanityCheckToken() {
	h.cache.sanityCheck.Set(sanityCheckToken, true, ttlcache.DefaultTTL)
}

func (h CloudflareHandle) skipSanityCheck() {
	h.skipSanityCheckToken()
	h.cache.sanityCheck.Set(sanityCheckAccount, true, ttlcache.DefaultTTL)
}

// SanityCheckToken verifies the Cloudflare token.
func (h CloudflareHandle) SanityCheckToken(ctx context.Context, ppfmt pp.PP) (bool, bool) {
	if valid := h.cache.sanityCheck.Get(sanityCheckToken); valid != nil {
		return valid.Value(), true
	}

	quickCtx, cancel := context.WithTimeoutCause(ctx, time.Second, errTimeout)
	defer cancel()

	res, err := h.cf.VerifyAPIToken(quickCtx)
	if err != nil {
		if quickCtx.Err() != nil {
			return true, false
		}

		var requestError *cloudflare.RequestError
		var authorizationError *cloudflare.AuthorizationError

		// known error messages
		// 400:6003:"Invalid request headers"
		// 400:6111:"Invalid format for Authorization header"
		// 401:1000:"Invalid API Token"

		switch {
		case errors.As(err, &requestError),
			errors.As(err, &requestError) && requestError.InternalErrorCodeIs(6111),             //nolint:mnd
			errors.As(err, &authorizationError) && authorizationError.InternalErrorCodeIs(1000): //nolint:mnd

			ppfmt.Errorf(pp.EmojiUserError,
				"The Cloudflare API token is invalid; "+
					"please check the value of CF_API_TOKEN or CF_API_TOKEN_FILE")
			goto certainlyBad

		default:
			// We will try again later.
			return true, false
		}
	}

	// The API call succeeded, but the token might be in a bad status.
	switch res.Status {
	case "active":
	case "disabled", "expired":
		ppfmt.Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", res.Status)
		goto certainlyBad
	default:
		ppfmt.Warningf(pp.EmojiImpossible,
			"The Cloudflare API token is in an undocumented state %q; please report this at %s",
			res.Status, pp.IssueReportingURL)
		return true, false
	}

	if !res.ExpiresOn.IsZero() {
		now := time.Now()
		remainingLifespan := max(res.ExpiresOn.Sub(now), 0)

		ppfmt.Warningf(pp.EmojiAlarm, "The Cloudflare API token will expire at %s (%v left)",
			cron.DescribeIntuitively(now, res.ExpiresOn), remainingLifespan)
	}

	h.cache.sanityCheck.Set(sanityCheckToken, true, ttlcache.DefaultTTL)
	return true, true

certainlyBad:
	return false, true
}

// SanityCheck verifies both the Cloudflare API token and account ID.
// It returns false only when the token or the account ID is certainly bad.
func (h CloudflareHandle) SanityCheck(ctx context.Context, ppfmt pp.PP) (bool, bool) {
	return h.SanityCheckToken(ctx, ppfmt)
}
