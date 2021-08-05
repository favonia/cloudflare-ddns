package api

import (
	"context"
	"net"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/patrickmn/go-cache"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

type Cache = struct {
	listRecords  map[ipnet.Type]*cache.Cache
	activeZones  *cache.Cache
	zoneOfDomain *cache.Cache
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
	URL       string
}

func (t *CloudflareAuth) New(ctx context.Context, indent pp.Indent, cacheExpiration time.Duration) (Handle, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token, cloudflare.UsingAccount(t.AccountID))
	if err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", err)
		return nil, false
	}

	// set the base URL (mostly for testing)
	if t.URL != "" {
		handle.BaseURL = t.URL
	}

	// this is not needed, but is helpful for diagnosing the problem
	if _, err := handle.VerifyAPIToken(ctx); err != nil {
		pp.Printf(indent, pp.EmojiUserError, "The Cloudflare API token is not valid: %v", err)
		pp.Printf(indent, pp.EmojiUserError, "Please double-check CF_API_TOKEN or CF_API_TOKEN_FILE.")
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
			zoneOfDomain: cache.New(cacheExpiration, cleanupInterval),
		},
	}, true
}

func (h *CloudflareHandle) FlushCache() {
	h.cache.listRecords[ipnet.IP4].Flush()
	h.cache.listRecords[ipnet.IP6].Flush()
	h.cache.activeZones.Flush()
	h.cache.zoneOfDomain.Flush()
}

// ActiveZones replaces the broken built-in ZoneIDByName due to the possibility of multiple zones.
func (h *CloudflareHandle) ActiveZones(ctx context.Context, indent pp.Indent, name string) ([]string, bool) {
	if ids, found := h.cache.activeZones.Get(name); found {
		return ids.([]string), true
	}

	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(name, h.cf.AccountID, "active"))
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to check the existence of a zone named %s: %v", name, err)
		return nil, false
	}

	ids := make([]string, 0, len(res.Result))
	for i := range res.Result {
		ids = append(ids, res.Result[i].ID)
	}

	h.cache.activeZones.SetDefault(name, ids)

	return ids, true
}

func (h *CloudflareHandle) ZoneOfDomain(ctx context.Context, indent pp.Indent, domain FQDN) (string, bool) {
	if id, found := h.cache.zoneOfDomain.Get(domain.String()); found {
		return id.(string), true
	}

zoneSearch:
	for s := NewFQDNSplitter(domain); s.IsValid(); s.Next() {
		zoneName := s.Suffix()
		zones, ok := h.ActiveZones(ctx, indent, zoneName)
		if !ok {
			return "", false
		}

		switch len(zones) {
		case 0: // len(zones) == 0
			continue zoneSearch
		case 1: // len(zones) == 1
			h.cache.zoneOfDomain.SetDefault(domain.String(), zones[0])

			return zones[0], true

		default: // len(zones) > 1
			pp.Printf(indent, pp.EmojiImpossible,
				"Found multiple active zones named %q. Specifying CF_ACCOUNT_ID might help.", zoneName)
			return "", false
		}
	}

	pp.Printf(indent, pp.EmojiError, "Failed to find the zone of %s.", domain)
	return "", false
}

func (h *CloudflareHandle) ListRecords(ctx context.Context, indent pp.Indent,
	domain FQDN, ipNet ipnet.Type) (map[string]net.IP, bool) {
	if rmap, found := h.cache.listRecords[ipNet].Get(domain.String()); found {
		return rmap.(map[string]net.IP), true
	}

	zone, ok := h.ZoneOfDomain(ctx, indent, domain)
	if !ok {
		return nil, false
	}

	//nolint:exhaustivestruct // Other fields are intentionally unspecified
	rs, err := h.cf.DNSRecords(ctx, zone, cloudflare.DNSRecord{
		Name: domain.String(),
		Type: ipNet.RecordType(),
	})
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to retrieve records of %s: %v", domain, err)
		return nil, false
	}

	rmap := map[string]net.IP{}
	for i := range rs {
		rmap[rs[i].ID] = net.ParseIP(rs[i].Content)
	}

	return rmap, true
}

func (h *CloudflareHandle) DeleteRecord(ctx context.Context, indent pp.Indent,
	domain FQDN, ipNet ipnet.Type, id string) bool {
	zone, ok := h.ZoneOfDomain(ctx, indent, domain)
	if !ok {
		return false
	}

	if err := h.cf.DeleteDNSRecord(ctx, zone, id); err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to delete a stale %s record of %s (ID: %s): %v",
			ipNet.RecordType(), domain, id, err)

		h.cache.listRecords[ipNet].Delete(domain.String())

		return false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain.String()); found {
		delete(rmap.(map[string]net.IP), id)
	}

	return true
}

func (h *CloudflareHandle) UpdateRecord(ctx context.Context, indent pp.Indent,
	domain FQDN, ipNet ipnet.Type, id string, ip net.IP) bool {
	zone, ok := h.ZoneOfDomain(ctx, indent, domain)
	if !ok {
		return false
	}

	//nolint:exhaustivestruct // Other fields are intentionally omitted
	payload := cloudflare.DNSRecord{
		Name:    domain.String(),
		Type:    ipNet.RecordType(),
		Content: ip.String(),
	}

	if err := h.cf.UpdateDNSRecord(ctx, zone, id, payload); err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to update a stale %s record of %s (ID: %s): %v",
			ipNet.RecordType(), domain, id, err)

		h.cache.listRecords[ipNet].Delete(domain.String())

		return false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain.String()); found {
		rmap.(map[string]net.IP)[id] = ip
	}

	return true
}

func (h *CloudflareHandle) CreateRecord(ctx context.Context, indent pp.Indent,
	domain FQDN, ipNet ipnet.Type, ip net.IP, ttl int, proxied bool) (string, bool) {
	zone, ok := h.ZoneOfDomain(ctx, indent, domain)
	if !ok {
		return "", false
	}

	//nolint:exhaustivestruct // Other fields are intentionally omitted
	payload := cloudflare.DNSRecord{
		Name:    domain.String(),
		Type:    ipNet.RecordType(),
		Content: ip.String(),
		TTL:     ttl,
		Proxied: &proxied,
	}

	res, err := h.cf.CreateDNSRecord(ctx, zone, payload)
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to add a new %s record of %s: %v", ipNet.RecordType(), domain, err)

		h.cache.listRecords[ipNet].Delete(domain.String())

		return "", false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain.String()); found {
		rmap.(map[string]net.IP)[res.Result.ID] = ip
	}

	return res.Result.ID, true
}
