package api

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// CloudflareCache holds the previous repsonses from the Cloudflare API.
type CloudflareCache = struct {
	// sanity check
	sanityCheck *ttlcache.Cache[struct{}, bool] // whether token is valid
	// domains to zones
	listZones    *ttlcache.Cache[string, []string] // zone names to zone IDs
	zoneOfDomain *ttlcache.Cache[string, string]   // domain names to the zone ID
	// records of domains
	listRecords map[ipnet.Type]*ttlcache.Cache[string, map[string]netip.Addr] // domain names to IPs
	// lists
	listLists     *ttlcache.Cache[struct{}, map[string][]string] // list names to list IDs
	listListItems *ttlcache.Cache[string, []WAFListItem]         // list IDs to list items
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
func (t CloudflareAuth) New(_ context.Context, ppfmt pp.PP, cacheExpiration time.Duration) (Handle, bool) {
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
			sanityCheck:  newCache[struct{}, bool](cacheExpiration),
			listZones:    newCache[string, []string](cacheExpiration),
			zoneOfDomain: newCache[string, string](cacheExpiration),
			listRecords: map[ipnet.Type]*ttlcache.Cache[string, map[string]netip.Addr]{
				ipnet.IP4: newCache[string, map[string]netip.Addr](cacheExpiration),
				ipnet.IP6: newCache[string, map[string]netip.Addr](cacheExpiration),
			},
			listLists:     newCache[struct{}, map[string][]string](cacheExpiration),
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

// SanityCheck verifies Cloudflare tokens.
//
// Ideally, we should also verify accountID here, but that is impossible without
// more permissions included in the API token.
func (h CloudflareHandle) SanityCheck(ctx context.Context, ppfmt pp.PP) bool {
	if valid := h.cache.sanityCheck.Get(struct{}{}); valid != nil {
		return valid.Value()
	}

	quickCtx, cancel := context.WithTimeoutCause(ctx, time.Second, errTimeout)
	defer cancel()

	ok := true
	res, err := h.cf.VerifyAPIToken(quickCtx)
	if err != nil {
		// Check if the token is permanently invalid...
		var aerr *cloudflare.AuthorizationError
		var rerr *cloudflare.RequestError
		if errors.As(err, &aerr) || errors.As(err, &rerr) {
			ppfmt.Errorf(pp.EmojiUserError, "The Cloudflare API token is invalid: %v", err)
			ok = false
			goto permanently
		}
		if !errors.Is(context.Cause(quickCtx), errTimeout) {
			ppfmt.Warningf(pp.EmojiWarning, "Failed to verify the Cloudflare API token; will retry later: %v", err)
		}
		return true // It could be that the network is temporarily down.
	}
	switch res.Status {
	case "active":
	case "disabled", "expired":
		ppfmt.Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", res.Status)
		ok = false
		goto permanently
	default:
		ppfmt.Warningf(pp.EmojiImpossible, "The Cloudflare API token is in an undocumented state: %s", res.Status)
		ppfmt.Warningf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new") //nolint:lll
		goto permanently
	}

	if !res.ExpiresOn.IsZero() {
		ppfmt.Warningf(pp.EmojiAlarm, "The token will expire at %s",
			res.ExpiresOn.In(time.Local).Format(time.RFC1123Z))
	}

permanently:
	if !ok {
		ppfmt.Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE")
	}
	h.cache.sanityCheck.Set(struct{}{}, ok, ttlcache.DefaultTTL)
	return ok
}

func (h CloudflareHandle) forcePassSanityCheck() {
	h.cache.sanityCheck.Set(struct{}{}, true, ttlcache.DefaultTTL)
}
