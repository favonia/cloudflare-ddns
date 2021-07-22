package api

import (
	"context"
	"log"
	"net"

	"github.com/cloudflare/cloudflare-go"
	"github.com/patrickmn/go-cache"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

type Cache = struct {
	listRecords  map[ipnet.Type]*cache.Cache
	activeZones  *cache.Cache
	zoneOfDomain *cache.Cache
}

type Handle struct {
	cf    *cloudflare.API
	cache Cache
}

const (
	CleanupIntervalFactor = 2
)

// activeZoneIDsByName replaces the broken built-in ZoneIDByName due to the possibility of multiple zones.
func (h *Handle) activeZones(ctx context.Context, name string) ([]string, bool) {
	if ids, found := h.cache.activeZones.Get(name); found {
		return ids.([]string), true
	}

	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(name, h.cf.AccountID, "active"))
	if err != nil {
		log.Printf("🤔 Failed to check the existence of a zone named %s: %v", name, err)
		return nil, false
	}

	ids := make([]string, 0, len(res.Result))
	for i := range res.Result {
		ids = append(ids, res.Result[i].ID)
	}

	h.cache.activeZones.SetDefault(name, ids)

	return ids, true
}

func (h *Handle) zoneOfDomain(ctx context.Context, domain string) (string, bool) {
	if id, found := h.cache.zoneOfDomain.Get(domain); found {
		return id.(string), true
	}

zoneSearch:
	for i := -1; i < len(domain); i++ {
		if i == -1 || domain[i] == '.' {
			zoneName := domain[i+1:]
			zones, ok := h.activeZones(ctx, zoneName)
			if !ok {
				return "", false
			}

			switch len(zones) {
			case 0: // len(zones) == 0
				continue zoneSearch
			case 1: // len(zones) == 1
				h.cache.zoneOfDomain.SetDefault(domain, zones[0])

				return zones[0], true

			default: // len(zones) > 1
				log.Printf("🤔 Found multiple zones named %s. Consider specifying CF_ACCOUNT_ID.", zoneName)
				return "", false
			}
		}
	}

	log.Printf("🤔 Failed to find the zone of %s.", domain)
	return "", false
}

func (h *Handle) listRecords(ctx context.Context, domain string, ipNet ipnet.Type) (map[string]net.IP, bool) {
	if rmap, found := h.cache.listRecords[ipNet].Get(domain); found {
		return rmap.(map[string]net.IP), true
	}

	zone, ok := h.zoneOfDomain(ctx, domain)
	if !ok {
		return nil, false
	}

	//nolint:exhaustivestruct // Other fields are intentionally unspecified
	rs, err := h.cf.DNSRecords(ctx, zone, cloudflare.DNSRecord{
		Name: domain,
		Type: ipNet.RecordType(),
	})
	if err != nil {
		log.Printf("🤔 Failed to retrieve records of %s: %v", domain, err)
		return nil, false
	}

	rmap := map[string]net.IP{}
	for i := range rs {
		rmap[rs[i].ID] = net.ParseIP(rs[i].Content)
	}

	return rmap, true
}

func (h *Handle) deleteRecord(ctx context.Context, domain string, ipNet ipnet.Type, id string) bool {
	zone, ok := h.zoneOfDomain(ctx, domain)
	if !ok {
		return false
	}

	if err := h.cf.DeleteDNSRecord(ctx, zone, id); err != nil {
		log.Printf("😡 Failed to delete a stale %s record of %s (ID: %s): %v", ipNet.RecordType(), domain, id, err)

		h.cache.listRecords[ipNet].Delete(domain)

		return false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain); found {
		delete(rmap.(map[string]net.IP), id)
	}

	return true
}

func (h *Handle) updateRecord(ctx context.Context, domain string, ipNet ipnet.Type, id string, ip net.IP) bool {
	zone, ok := h.zoneOfDomain(ctx, domain)
	if !ok {
		return false
	}

	//nolint:exhaustivestruct // Other fields are intentionally omitted
	payload := cloudflare.DNSRecord{
		Name:    domain,
		Type:    ipNet.RecordType(),
		Content: ip.String(),
	}

	if err := h.cf.UpdateDNSRecord(ctx, zone, id, payload); err != nil {
		log.Printf("😡 Failed to update a stale %s record of %s (ID: %s): %v", ipNet.RecordType(), domain, id, err)

		h.cache.listRecords[ipNet].Delete(domain)

		return false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain); found {
		rmap.(map[string]net.IP)[id] = ip
	}

	return true
}

func (h *Handle) createRecord(ctx context.Context,
	domain string, ipNet ipnet.Type, ip net.IP, ttl int, proxied bool,
) (string, bool) {
	zone, ok := h.zoneOfDomain(ctx, domain)
	if !ok {
		return "", false
	}

	//nolint:exhaustivestruct // Other fields are intentionally omitted
	payload := cloudflare.DNSRecord{
		Name:    domain,
		Type:    ipNet.RecordType(),
		Content: ip.String(),
		TTL:     ttl,
		Proxied: &proxied,
	}

	res, err := h.cf.CreateDNSRecord(ctx, zone, payload)
	if err != nil {
		log.Printf("😡 Failed to add a new %s record of %s: %v", ipNet.RecordType(), domain, err)

		h.cache.listRecords[ipNet].Delete(domain)

		return "", false
	}

	if rmap, found := h.cache.listRecords[ipNet].Get(domain); found {
		rmap.(map[string]net.IP)[res.Result.ID] = ip
	}

	return res.Result.ID, true
}
