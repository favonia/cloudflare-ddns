package api

import (
	"context"
	"net"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/patrickmn/go-cache"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Cache = struct {
	listRecords  map[ipnet.Type]*cache.Cache
	activeZones  *cache.Cache
	locateDomain *cache.Cache
}

type CloudflareHandle struct {
	cf    *cloudflare.API
	cache Cache
}

const (
	CleanupIntervalFactor = 2
)

type CloudflareAuth struct {
	Token     string
	AccountID string
	BaseURL   string
}

func (t *CloudflareAuth) New(ctx context.Context, ppfmt pp.PP, cacheExpiration time.Duration) (Handle, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token, cloudflare.UsingAccount(t.AccountID))
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
		ppfmt.Errorf(pp.EmojiUserError, "The Cloudflare API token is not valid: %v", err)
		ppfmt.Errorf(pp.EmojiUserError, "Please double-check CF_API_TOKEN or CF_API_TOKEN_FILE")
		return nil, false
	}

	cleanupInterval := cacheExpiration * CleanupIntervalFactor

	return &CloudflareHandle{
		cf: handle,
		cache: Cache{
			listRecords: map[ipnet.Type]*cache.Cache{
				ipnet.IP4: cache.New(cacheExpiration, cleanupInterval),
				ipnet.IP6: cache.New(cacheExpiration, cleanupInterval),
			},
			activeZones:  cache.New(cacheExpiration, cleanupInterval),
			locateDomain: cache.New(cacheExpiration, cleanupInterval),
		},
	}, true
}

func (h *CloudflareHandle) FlushCache() {
	h.cache.listRecords[ipnet.IP4].Flush()
	h.cache.listRecords[ipnet.IP6].Flush()
	h.cache.activeZones.Flush()
	h.cache.locateDomain.Flush()
}

// ActiveZones lists all active zones of the given name.
func (h *CloudflareHandle) ActiveZones(ctx context.Context, ppfmt pp.PP, name string) ([]string, bool) {
	// WithZoneFilters does not work with the empty zone name,
	// and the owner of the DNS root zone will not be managed by Cloudflare anyways!
	if name == "" {
		return []string{}, true
	}

	if ids, found := h.cache.activeZones.Get(name); found {
		return ids.([]string), true
	}

	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(name, h.cf.AccountID, "active"))
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v", name, err)
		return nil, false
	}

	ids := make([]string, 0, len(res.Result))
	for i := range res.Result {
		ids = append(ids, res.Result[i].ID)
	}

	h.cache.activeZones.SetDefault(name, ids)

	return ids, true
}

type LocatedDomain struct {
	Zone    string
	DNSName string
}

func (h *CloudflareHandle) LocateDomain(ctx context.Context, ppfmt pp.PP, domain Domain) (LocatedDomain, bool) {
	if id, found := h.cache.locateDomain.Get(domain.String()); found {
		return id.(LocatedDomain), true
	}

zoneSearch:
	for s := domain.Split(); s.IsValid(); s.Next() {
		zoneName := s.ZoneNameASCII()
		zones, ok := h.ActiveZones(ctx, ppfmt, zoneName)
		if !ok {
			return LocatedDomain{Zone: "", DNSName: ""}, false
		}

		switch len(zones) {
		case 0: // len(zones) == 0
			continue zoneSearch
		case 1: // len(zones) == 1
			ret := LocatedDomain{Zone: zones[0], DNSName: s.DNSNameASCII()}
			h.cache.locateDomain.SetDefault(domain.String(), ret)
			return ret, true
		default: // len(zones) > 1
			ppfmt.Warningf(pp.EmojiImpossible,
				"Found multiple active zones named %q. Specifying CF_ACCOUNT_ID might help", zoneName)
			return LocatedDomain{Zone: "", DNSName: ""}, false
		}
	}

	ppfmt.Warningf(pp.EmojiError, "Failed to find the zone of %q", domain.Describe())
	return LocatedDomain{Zone: "", DNSName: ""}, false
}

func (h *CloudflareHandle) ListRecords(ctx context.Context, ppfmt pp.PP,
	domain Domain, ipNet ipnet.Type) (map[string]net.IP, bool) {
	if rmap, found := h.cache.listRecords[ipNet].Get(domain.String()); found {
		return rmap.(map[string]net.IP), true
	}

	locatedDomain, ok := h.LocateDomain(ctx, ppfmt, domain)
	if !ok {
		return nil, false
	}

	//nolint:exhaustivestruct // Other fields are intentionally unspecified
	rs, err := h.cf.DNSRecords(ctx, locatedDomain.Zone, cloudflare.DNSRecord{
		Name: locatedDomain.DNSName,
		Type: ipNet.RecordType(),
	})
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to retrieve records of %q: %v", domain.Describe(), err)
		return nil, false
	}

	rmap := map[string]net.IP{}
	for i := range rs {
		rmap[rs[i].ID] = net.ParseIP(rs[i].Content)
	}

	h.cache.listRecords[ipNet].SetDefault(domain.String(), rmap)

	return rmap, true
}

func (h *CloudflareHandle) DeleteRecord(ctx context.Context, ppfmt pp.PP,
	domain Domain, ipNet ipnet.Type, id string) bool {
	locatedDomain, ok := h.LocateDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	if err := h.cf.DeleteDNSRecord(ctx, locatedDomain.Zone, id); err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to delete a stale %s record of %q (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)

		h.cache.listRecords[ipNet].Delete(domain.String())

		return false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain.String()); found {
		delete(rmap.(map[string]net.IP), id)
	}

	return true
}

func (h *CloudflareHandle) UpdateRecord(ctx context.Context, ppfmt pp.PP,
	domain Domain, ipNet ipnet.Type, id string, ip net.IP) bool {
	locatedDomain, ok := h.LocateDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	//nolint:exhaustivestruct // Other fields are intentionally omitted
	payload := cloudflare.DNSRecord{
		Name:    locatedDomain.DNSName,
		Type:    ipNet.RecordType(),
		Content: ip.String(),
	}

	if err := h.cf.UpdateDNSRecord(ctx, locatedDomain.Zone, id, payload); err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to update a stale %s record of %q (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)

		h.cache.listRecords[ipNet].Delete(domain.String())

		return false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain.String()); found {
		rmap.(map[string]net.IP)[id] = ip
	}

	return true
}

func (h *CloudflareHandle) CreateRecord(ctx context.Context, ppfmt pp.PP,
	domain Domain, ipNet ipnet.Type, ip net.IP, ttl TTL, proxied bool) (string, bool) {
	locatedDomain, ok := h.LocateDomain(ctx, ppfmt, domain)
	if !ok {
		return "", false
	}

	//nolint:exhaustivestruct // Other fields are intentionally omitted
	payload := cloudflare.DNSRecord{
		Name:    locatedDomain.DNSName,
		Type:    ipNet.RecordType(),
		Content: ip.String(),
		TTL:     ttl.Int(),
		Proxied: &proxied,
	}

	res, err := h.cf.CreateDNSRecord(ctx, locatedDomain.Zone, payload)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to add a new %s record of %q: %v",
			ipNet.RecordType(), domain.Describe(), err)

		h.cache.listRecords[ipNet].Delete(domain.String())

		return "", false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain.String()); found {
		rmap.(map[string]net.IP)[res.Result.ID] = ip
	}

	return res.Result.ID, true
}
