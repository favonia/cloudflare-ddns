package api

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/patrickmn/go-cache"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

type Handle struct {
	cf *cloudflare.API
}

const (
	CleanupIntervalFactor = 2
)

var apiCache struct { //nolint:gochecknoglobals
	savedIP      map[ipnet.Type]*cache.Cache
	activeZones  *cache.Cache
	zoneOfDomain *cache.Cache
}

func InitCache(expiration time.Duration) {
	cleanupInterval := expiration * CleanupIntervalFactor
	apiCache.savedIP = map[ipnet.Type]*cache.Cache{
		ipnet.IP4: cache.New(expiration, cleanupInterval),
		ipnet.IP6: cache.New(expiration, cleanupInterval),
	}
	apiCache.activeZones = cache.New(expiration, cleanupInterval)
	apiCache.zoneOfDomain = cache.New(expiration, cleanupInterval)
}

type record = struct {
	ID string
	IP net.IP
}

// activeZoneIDsByName replaces the broken built-in ZoneIDByName due to the possibility of multiple zones.
func (h *Handle) activeZones(ctx context.Context, name string) ([]string, bool) {
	if ids, found := apiCache.activeZones.Get(name); found {
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

	apiCache.activeZones.SetDefault(name, ids)

	return ids, true
}

func (h *Handle) zoneOfDomain(ctx context.Context, domain string) (string, bool) {
	if id, found := apiCache.zoneOfDomain.Get(domain); found {
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
				apiCache.zoneOfDomain.SetDefault(domain, zones[0])

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

func (h *Handle) listRecords(ctx context.Context, domain string, ipNet ipnet.Type) ([]record, bool) {
	zone, ok := h.zoneOfDomain(ctx, domain)
	if !ok {
		return nil, false
	}

	//nolint:exhaustivestruct // Other fields are intentionally unspecified
	rawRecords, err := h.cf.DNSRecords(ctx, zone, cloudflare.DNSRecord{
		Name: domain,
		Type: ipNet.RecordType(),
	})
	if err != nil {
		log.Printf("🤔 Failed to retrieve records of %s: %v", domain, err)
		return nil, false
	}

	rs := make([]record, 0, len(rawRecords))
	for i := range rawRecords {
		rs = append(rs, record{
			ID: rawRecords[i].ID,
			IP: net.ParseIP(rawRecords[i].Content),
		})
	}

	return rs, true
}

func (h *Handle) listRecordIDs(ctx context.Context, domain string, ipNet ipnet.Type, ip net.IP) (matchedIDs, unmatchedIDs []string, ok bool) {
	rs, ok := h.listRecords(ctx, domain, ipNet)
	if !ok {
		return nil, nil, false
	}

	for _, r := range rs {
		if ip.Equal(r.IP) {
			matchedIDs = append(matchedIDs, r.ID)
		} else {
			unmatchedIDs = append(unmatchedIDs, r.ID)
		}
	}

	return matchedIDs, unmatchedIDs, true
}
