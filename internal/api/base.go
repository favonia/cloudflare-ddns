package api

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/patrickmn/go-cache"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

type Handle struct {
	cf *cloudflare.API
}

const (
	CleanupIntervalFactor = 2
)

var apiCache struct { //nolint:gochecknoglobals
	listRecords  map[ipnet.Type]*cache.Cache
	activeZones  *cache.Cache
	zoneOfDomain *cache.Cache
}

func InitCache(expiration time.Duration) {
	cleanupInterval := expiration * CleanupIntervalFactor
	apiCache.listRecords = map[ipnet.Type]*cache.Cache{
		ipnet.IP4: cache.New(expiration, cleanupInterval),
		ipnet.IP6: cache.New(expiration, cleanupInterval),
	}
	apiCache.activeZones = cache.New(expiration, cleanupInterval)
	apiCache.zoneOfDomain = cache.New(expiration, cleanupInterval)
}

// activeZoneIDsByName replaces the broken built-in ZoneIDByName due to the possibility of multiple zones.
func (h *Handle) activeZones(ctx context.Context, name string) ([]string, bool) {
	if ids, found := apiCache.activeZones.Get(name); found {
		return ids.([]string), true
	}

	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(name, h.cf.AccountID, "active"))
	if err != nil {
		log.Printf("🤔 Failed to check the existence of a zone named %s: %v", name, err)
		return nil, false //nolint:nlreturn
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
			case 0: // len(zoneIDs) == 0
				continue zoneSearch
			case 1: // len(zoneIDs) == 1
				apiCache.zoneOfDomain.SetDefault(domain, zones[0])

				return zones[0], true

			default: // len(zoneIDs) > 1
				log.Printf("🤔 Found multiple zones named %s. Consider specifying CF_ACCOUNT_ID.", zoneName)
				return "", false //nolint:nlreturn
			}
		}
	}

	log.Printf("🤔 Failed to find the zone of %s.", domain)
	return "", false //nolint:nlreturn,wsl
}

func (h *Handle) getCurrentRecords(ctx context.Context, zoneID, domain string, ipNet ipnet.Type, ip net.IP) (matchedIDs, unmatchedIDs []string, ok bool) { //nolint:lll
	//nolint:exhaustivestruct // Other fields are intentionally unspecified
	rs, err := h.cf.DNSRecords(ctx, zoneID, cloudflare.DNSRecord{
		Name: domain,
		Type: ipNet.RecordType(),
	})
	if err != nil {
		log.Printf("🤔 Failed to retrieve records of %s: %v", domain, err)
		return nil, nil, false //nolint:nlreturn
	}

	for i := range rs {
		if ip.Equal(net.ParseIP(rs[i].Content)) {
			matchedIDs = append(matchedIDs, rs[i].ID)
		} else {
			unmatchedIDs = append(unmatchedIDs, rs[i].ID)
		}
	}

	return matchedIDs, unmatchedIDs, true
}

// UpdateArgs is the type of (named) arguments to updateRecords.
type UpdateArgs struct {
	Quiet     quiet.Quiet
	IPNetwork ipnet.Type
	IP        net.IP
	Target    Target
	TTL       int
	Proxied   bool
}

func (h *Handle) updateNoCache(ctx context.Context, args *UpdateArgs) (net.IP, bool) { //nolint:funlen,cyclop,gocognit
	domain, ok := args.Target.domain(ctx, h)
	if !ok {
		return nil, false
	}

	zone, ok := args.Target.zone(ctx, h)
	if !ok {
		return nil, false
	}

	matchedIDs, unmatchedIDs, ok := h.getCurrentRecords(ctx, zone, domain, args.IPNetwork, args.IP)
	if !ok {
		return nil, false
	}

	// whether there was already an up-to-date record
	uptodate := false

	// delete every record if ip is `nil`
	if args.IP == nil {
		uptodate = true
	}

	if !uptodate && len(matchedIDs) > 0 {
		if !args.Quiet {
			log.Printf("😃 Found an up-to-date %s record of %s.", args.IPNetwork.RecordType(), domain)
		}

		uptodate = true
		matchedIDs = matchedIDs[1:]
	}

	// the data for updating or creating a record
	//nolint:exhaustivestruct // Other fields are intentionally unspecified
	payload := cloudflare.DNSRecord{
		Name:    domain,
		Type:    args.IPNetwork.RecordType(),
		Content: args.IP.String(),
		TTL:     args.TTL,
		Proxied: &args.Proxied,
	}

	if !uptodate && args.IP != nil {
		var unhandled []string

		for i, id := range unmatchedIDs {
			log.Printf("📝 Updating a stale %s record of %s (ID: %s) . . .", args.IPNetwork.RecordType(), domain, id)
			if err := h.cf.UpdateDNSRecord(ctx, zone, id, payload); err != nil { //nolint:wsl
				log.Printf("😡 Failed to update the record: %v", err)
				log.Printf("🧟 Deleting the record instead . . .")
				if err = h.cf.DeleteDNSRecord(ctx, zone, id); err != nil { //nolint:wsl
					log.Printf("😡 Failed to delete the record, too: %v", err)
				}

				continue
			}

			uptodate = true
			unhandled = unmatchedIDs[i+1:]

			break
		}

		unmatchedIDs = unhandled
	}

	if !uptodate && args.IP != nil {
		log.Printf("👶 Adding a new %s record for %s.", args.IPNetwork.RecordType(), domain)
		if _, err := h.cf.CreateDNSRecord(ctx, zone, payload); err != nil { //nolint:wsl
			log.Printf("😡 Failed to add the record: %v", err)
		} else {
			uptodate = true
		}
	}

	for _, id := range unmatchedIDs {
		log.Printf("🧟 Deleting a stale %s record of %s (ID: %s) . . .", args.IPNetwork.RecordType(), domain, id)
		if err := h.cf.DeleteDNSRecord(ctx, zone, id); err != nil { //nolint:wsl
			log.Printf("😡 Failed to delete the record: %v", err)
		}
	}

	for _, id := range matchedIDs {
		log.Printf("👻 Removing a duplicate %s record of %s (ID: %s) . . .", args.IPNetwork.RecordType(), domain, id)
		if err := h.cf.DeleteDNSRecord(ctx, zone, id); err != nil { //nolint:wsl
			log.Printf("😡 Failed to remove the record: %v", err)
		}
	}

	if !uptodate {
		log.Printf("😡 Failed to update %s records of %s.", args.IPNetwork.RecordType(), domain)
		return nil, false //nolint:nlreturn
	}

	return args.IP, true
}

func (h *Handle) Update(ctx context.Context, args *UpdateArgs) bool {
	domain, ok := args.Target.domain(ctx, h)
	if !ok {
		return false
	}

	savedIP, saved := apiCache.listRecords[args.IPNetwork].Get(domain)

	if saved && savedIP.(net.IP).Equal(args.IP) {
		if !args.Quiet {
			log.Printf("🤷 No need to change %s records of %s.", args.IPNetwork.RecordType(), domain)
		}

		return true
	}

	ip, ok := h.updateNoCache(ctx, args)
	if !ok {
		apiCache.listRecords[args.IPNetwork].Delete(domain)

		log.Printf("😡 Failed to update %s records of %s.", args.IPNetwork.RecordType(), domain)
		return false //nolint:nlreturn,wsl
	}

	apiCache.listRecords[args.IPNetwork].SetDefault(domain, ip)
	return true //nolint:nlreturn,wsl
}
