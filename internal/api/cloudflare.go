package api

import (
	"context"
	"net/netip"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// CloudflareCache holds the previous repsonses from the Cloudflare API.
type CloudflareCache = struct {
	listRecords  map[ipnet.Type]*ttlcache.Cache[string, map[string]netip.Addr]
	activeZones  *ttlcache.Cache[string, []string]
	zoneOfDomain *ttlcache.Cache[string, string]
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
func (t *CloudflareAuth) New(ctx context.Context, ppfmt pp.PP, cacheExpiration time.Duration) (Handle, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", err)
		return nil, false
	}

	// set the base URL (mostly for testing)
	if t.BaseURL != "" {
		handle.BaseURL = t.BaseURL
	}

	// this is not needed, but is helpful for diagnosing the problem
	if _, err := handle.VerifyAPIToken(ctx); err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "The Cloudflare API token could not be verified: %v", err)
		ppfmt.Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE")
		return nil, false
	}

	return &CloudflareHandle{
		cf:        handle,
		accountID: t.AccountID,
		cache: CloudflareCache{
			listRecords: map[ipnet.Type]*ttlcache.Cache[string, map[string]netip.Addr]{
				ipnet.IP4: newCache[string, map[string]netip.Addr](cacheExpiration),
				ipnet.IP6: newCache[string, map[string]netip.Addr](cacheExpiration),
			},
			activeZones:  newCache[string, []string](cacheExpiration),
			zoneOfDomain: newCache[string, string](cacheExpiration),
		},
	}, true
}

// FlushCache flushes the API cache.
func (h *CloudflareHandle) FlushCache() {
	for _, cache := range h.cache.listRecords {
		cache.DeleteAll()
	}
	h.cache.activeZones.DeleteAll()
	h.cache.zoneOfDomain.DeleteAll()
}

// ActiveZones returns a list of zone IDs with the zone name.
func (h *CloudflareHandle) ActiveZones(ctx context.Context, ppfmt pp.PP, name string) ([]string, bool) {
	// WithZoneFilters does not work with the empty zone name,
	// and the owner of the DNS root zone will not be managed by Cloudflare anyways!
	if name == "" {
		return []string{}, true
	}

	if ids := h.cache.activeZones.Get(name); ids != nil {
		return ids.Value(), true
	}

	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(name, h.accountID, ""))
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v", name, err)
		return nil, false
	}

	ids := make([]string, 0, len(res.Result))
	for _, zone := range res.Result {
		// The list of possible statuses was at https://api.cloudflare.com/#zone-list-zones
		// but the documentation is missing now.
		switch zone.Status {
		case "active": // fully working
			ids = append(ids, zone.ID)
		case
			"deactivated",  // violating term of service, etc.
			"initializing", // the setup was just started?
			"moved",        // domain registrar not pointing to Cloudflare
			"pending":      // the setup was not completed
			ppfmt.Warningf(pp.EmojiWarning, "Zone %q is %q; your Cloudflare setup is incomplete; some features might not work as expected", name, zone.Status) //nolint:lll
			ids = append(ids, zone.ID)
		case
			"deleted": // archived, pending/moved for too long
			ppfmt.Infof(pp.EmojiWarning, "Zone %q is %q and thus skipped", name, zone.Status)
			// skip these
		default:
			ppfmt.Warningf(pp.EmojiImpossible, "Zone %q is in an undocumented status %q", name, zone.Status)
			ppfmt.Warningf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new") //nolint:lll
			ids = append(ids, zone.ID)
		}
	}

	h.cache.activeZones.Set(name, ids, ttlcache.DefaultTTL)

	return ids, true
}

// ZoneOfDomain finds the active zone ID governing a particular domain.
func (h *CloudflareHandle) ZoneOfDomain(ctx context.Context, ppfmt pp.PP, domain domain.Domain) (string, bool) {
	if id := h.cache.zoneOfDomain.Get(domain.DNSNameASCII()); id != nil {
		return id.Value(), true
	}

zoneSearch:
	for s := domain.Split(); s.IsValid(); s.Next() {
		zoneName := s.ZoneNameASCII()
		zones, ok := h.ActiveZones(ctx, ppfmt, zoneName)
		if !ok {
			return "", false
		}

		switch len(zones) {
		case 0: // len(zones) == 0
			continue zoneSearch
		case 1: // len(zones) == 1
			h.cache.zoneOfDomain.Set(domain.DNSNameASCII(), zones[0], ttlcache.DefaultTTL)
			return zones[0], true
		default: // len(zones) > 1
			ppfmt.Warningf(pp.EmojiImpossible, "Found multiple active zones named %q. Specifying CF_ACCOUNT_ID might help", zoneName)                    //nolint:lll
			ppfmt.Warningf(pp.EmojiImpossible, "Please consider reporting this rare situation at https://github.com/favonia/cloudflare-ddns/issues/new") //nolint:lll
			return "", false
		}
	}

	ppfmt.Warningf(pp.EmojiError, "Failed to find the zone of %q", domain.Describe())
	return "", false
}

// ListRecords lists all matching DNS records.
func (h *CloudflareHandle) ListRecords(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipNet ipnet.Type,
) (map[string]netip.Addr, bool) {
	if rmap := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rmap != nil {
		return rmap.Value(), true
	}

	zone, ok := h.ZoneOfDomain(ctx, ppfmt, domain)
	if !ok {
		return nil, false
	}

	//nolint:exhaustruct // Other fields are intentionally unspecified
	rs, _, err := h.cf.ListDNSRecords(ctx,
		cloudflare.ZoneIdentifier(zone),
		cloudflare.ListDNSRecordsParams{
			Name: domain.DNSNameASCII(),
			Type: ipNet.RecordType(),
		})
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to retrieve records of %q: %v", domain.Describe(), err)
		return nil, false
	}

	rmap := map[string]netip.Addr{}
	for i := range rs {
		rmap[rs[i].ID], err = netip.ParseAddr(rs[i].Content)
		if err != nil {
			ppfmt.Warningf(pp.EmojiImpossible, "Failed to parse the IP address in records of %q: %v", domain.Describe(), err)
			return nil, false
		}
	}

	h.cache.listRecords[ipNet].Set(domain.DNSNameASCII(), rmap, ttlcache.DefaultTTL)

	return rmap, true
}

// DeleteRecord deletes one DNS record.
func (h *CloudflareHandle) DeleteRecord(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipNet ipnet.Type, id string,
) bool {
	zone, ok := h.ZoneOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	if err := h.cf.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zone), id); err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to delete a stale %s record of %q (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return false
	}

	if rmap := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rmap != nil {
		delete(rmap.Value(), id)
	}

	return true
}

// UpdateRecord updates one DNS record.
func (h *CloudflareHandle) UpdateRecord(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipNet ipnet.Type, id string, ip netip.Addr,
) bool {
	zone, ok := h.ZoneOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	//nolint:exhaustruct // Other fields are intentionally omitted
	params := cloudflare.UpdateDNSRecordParams{
		ID:      id,
		Content: ip.String(),
	}

	if _, err := h.cf.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zone), params); err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to update a stale %s record of %q (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return false
	}

	if rmap := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rmap != nil {
		rmap.Value()[id] = ip
	}

	return true
}

// CreateRecord creates one DNS record.
func (h *CloudflareHandle) CreateRecord(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipNet ipnet.Type, ip netip.Addr, ttl TTL, proxied bool,
) (string, bool) {
	zone, ok := h.ZoneOfDomain(ctx, ppfmt, domain)
	if !ok {
		return "", false
	}

	//nolint:exhaustruct // Other fields are intentionally omitted
	params := cloudflare.CreateDNSRecordParams{
		Name:    domain.DNSNameASCII(),
		Type:    ipNet.RecordType(),
		Content: ip.String(),
		TTL:     ttl.Int(),
		Proxied: &proxied,
		Comment: "Created by cloudflare-ddns",
	}

	res, err := h.cf.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zone), params)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to add a new %s record of %q: %v",
			ipNet.RecordType(), domain.Describe(), err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return "", false
	}

	if rmap := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rmap != nil {
		rmap.Value()[res.ID] = ip
	}

	return res.ID, true
}
