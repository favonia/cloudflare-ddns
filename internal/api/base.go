package api

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/patrickmn/go-cache"

	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

type Handle struct {
	cf *cloudflare.API
}

const (
	DefaultCacheExpiration = time.Hour * 6
	CleanupIntervalFactor  = 2
)

var (
	savedIP4s      *cache.Cache
	savedIP6s      *cache.Cache
	zoneNameOfID   *cache.Cache
	zoneIDOfDomain *cache.Cache
)

// init makes sure the cache exists even if InitCache is not called.
func init() {
	InitCache(DefaultCacheExpiration)
}

func InitCache(expiration time.Duration) {
	cleanupInterval := expiration * CleanupIntervalFactor
	savedIP4s = cache.New(expiration, cleanupInterval)
	savedIP6s = cache.New(expiration, cleanupInterval)
	zoneNameOfID = cache.New(expiration, cleanupInterval)
	zoneIDOfDomain = cache.New(expiration, cleanupInterval)
}

func (h Handle) zoneName(ctx context.Context, zoneID string) (string, bool) {
	if name, found := zoneNameOfID.Get(zoneID); found {
		return name.(string), true
	}

	zone, err := h.cf.ZoneDetails(ctx, zoneID)
	if err != nil {
		log.Printf("🤔 Could not retrieve the name of the zone (ID: %s): %v", zoneID, err)
		return "", false //nolint:nlreturn
	}

	zoneNameOfID.SetDefault(zoneID, zone.Name)

	return zone.Name, true
}

// activeZoneIDsByName replaces the broken built-in ZoneIDByName due to the possibility of multiple zones.
func (h *Handle) activeZoneIDsByName(ctx context.Context, zoneName string) ([]string, bool) {
	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(zoneName, h.cf.AccountID, "active"))
	if err != nil {
		log.Printf("🤔 Could not check whether there's a zone named %s: %v", zoneName, err)
		return nil, false //nolint:nlreturn
	}

	ids := make([]string, 0, len(res.Result))
	for i := range res.Result {
		ids = append(ids, res.Result[i].ID)
	}

	return ids, true
}

func (h *Handle) zoneID(ctx context.Context, domain string) (string, bool) {
	if id, found := zoneIDOfDomain.Get(domain); found {
		return id.(string), true
	}

	// try the whole domain as the zone
	zoneName := domain

	zoneIDs, ok := h.activeZoneIDsByName(ctx, zoneName)
	if !ok {
		return "", false
	}

	switch len(zoneIDs) {
	case 0:
	zoneSearch:
		for i, b := range domain {
			if b == '.' {
				zoneName = domain[i+1:]
				zoneIDs, ok = h.activeZoneIDsByName(ctx, zoneName)
				if !ok {
					return "", false
				}

				switch len(zoneIDs) {
				case 0: // len(zoneIDs) == 0
					continue zoneSearch
				case 1: // len(zoneIDs) == 1
					break zoneSearch
				default: // len(zoneIDs) > 1
					log.Printf("🤔 Found multiple zones named %s. Consider specifying CF_ACCOUNT_ID.", zoneName)
					return "", false //nolint:nlreturn
				}
			}
		}
	case 1: // len(zoneIDs) == 1
		break
	default: // len(zoneIDs) > 1
		log.Printf("🤔 Found multiple zones named %s. Consider specifying CF_ACCOUNT_ID.", zoneName)
		return "", false //nolint:nlreturn
	}

	if len(zoneIDs) != 1 {
		log.Printf("🤔 Could not find the zone of the domain %s.", domain)
		return "", false //nolint:nlreturn
	}

	zoneIDOfDomain.SetDefault(domain, zoneIDs[0])
	zoneNameOfID.SetDefault(zoneIDs[0], zoneName)

	return zoneIDs[0], true
}

// updateRecordsArgs is the type of (named) arguments to updateRecords.
type updateRecordsArgs = struct {
	context    context.Context
	quiet      quiet.Quiet
	target     Target
	recordType string
	ip         net.IP
	ttl        int
	proxied    bool
}

func (h *Handle) getCurrentRecords(args *updateRecordsArgs, zoneID, domain string) (matchedIDs, unmatchedIDs []string, ok bool) { //nolint:lll
	//nolint:exhaustivestruct // Other fields are intentionally unspecified
	rs, err := h.cf.DNSRecords(args.context, zoneID, cloudflare.DNSRecord{
		Name: domain,
		Type: args.recordType,
	})
	if err != nil {
		log.Printf("🤔 Could not retrieve DNS records for the domain %s: %v", domain, err)
		return nil, nil, false //nolint:nlreturn
	}

	for i := range rs {
		if args.ip.Equal(net.ParseIP(rs[i].Content)) {
			matchedIDs = append(matchedIDs, rs[i].ID)
		} else {
			unmatchedIDs = append(unmatchedIDs, rs[i].ID)
		}
	}

	return matchedIDs, unmatchedIDs, true
}

func (h *Handle) updateRecords(args *updateRecordsArgs) (net.IP, bool) {
	domain, ok := args.target.domain(args.context, h)
	if !ok {
		return nil, false
	}

	zoneID, ok := args.target.zoneID(args.context, h)
	if !ok {
		return nil, false
	}

	matchedIDs, unmatchedIDs, ok := h.getCurrentRecords(args, zoneID, domain)
	if !ok {
		return nil, false
	}

	// whether there was already an up-to-date record
	uptodate := false

	// delete every record if ip is `nil`
	if args.ip == nil {
		uptodate = true
	}

	if !uptodate && len(matchedIDs) > 0 {
		if !args.quiet {
			log.Printf("😃 Found an up-to-date %s record for the domain %s.", args.recordType, domain)
		}

		uptodate = true
		matchedIDs = matchedIDs[1:]
	}

	// the data for updating or creating a record
	//nolint:exhaustivestruct // Other fields are intentionally unspecified
	payload := cloudflare.DNSRecord{
		Name:    domain,
		Type:    args.recordType,
		Content: args.ip.String(),
		TTL:     args.ttl,
		Proxied: &args.proxied,
	}

	if !uptodate && args.ip != nil {
		var unhandled []string

		for i, id := range unmatchedIDs {
			log.Printf("📝 Updating a stale %s record (ID: %s) . . .", args.recordType, id)
			if err := h.cf.UpdateDNSRecord(args.context, zoneID, id, payload); err != nil { //nolint:wsl
				log.Printf("😡 Could not update the record: %v", err)
				log.Printf("🧟 Deleting the record instead . . .")
				if err = h.cf.DeleteDNSRecord(args.context, zoneID, id); err != nil { //nolint:wsl
					log.Printf("😡 Could not delete the record, either: %v", err)
				}

				continue
			}

			uptodate = true
			unhandled = unmatchedIDs[i+1:]

			break
		}

		unmatchedIDs = unhandled
	}

	if !uptodate && args.ip != nil {
		log.Printf("👶 Adding a new %s record for the domain %s.", args.recordType, domain)
		if _, err := h.cf.CreateDNSRecord(args.context, zoneID, payload); err != nil { //nolint:wsl
			log.Printf("😡 Could not add the record: %v", err)
		} else {
			uptodate = true
		}
	}

	for _, id := range unmatchedIDs {
		log.Printf("🧟 Deleting a stale %s record (ID: %s) . . .", args.recordType, id)
		if err := h.cf.DeleteDNSRecord(args.context, zoneID, id); err != nil { //nolint:wsl
			log.Printf("😡 Could not delete the record: %v", err)
		}
	}

	for _, id := range matchedIDs {
		log.Printf("👻 Removing a duplicate %s record (ID: %s) . . .", args.recordType, id)
		if err := h.cf.DeleteDNSRecord(args.context, zoneID, id); err != nil { //nolint:wsl
			log.Printf("😡 Could not remove the record: %v", err)
		}
	}

	if !uptodate {
		log.Printf("😡 Failed to update %s records for the domain %s.", args.recordType, domain)
		return nil, false //nolint:nlreturn
	}

	return args.ip, true
}

type UpdateArgs struct {
	Context    context.Context
	Quiet      quiet.Quiet
	Target     Target
	IP4Managed bool
	IP4        net.IP
	IP6Managed bool
	IP6        net.IP
	TTL        int
	Proxied    bool
}

func (h *Handle) Update(args *UpdateArgs) bool {
	domain, ok := args.Target.domain(args.Context, h)
	if !ok {
		return false
	}

	checkingIP4 := false
	if args.IP4Managed { //nolint:wsl
		savedIP4, saved := savedIP4s.Get(domain)
		checkingIP4 = !(saved && savedIP4.(net.IP).Equal(args.IP4))
	}

	checkingIP6 := false
	if args.IP6Managed { //nolint:wsl
		savedIP6, saved := savedIP6s.Get(domain)
		checkingIP6 = !(saved && savedIP6.(net.IP).Equal(args.IP6))
	}

	if !checkingIP4 && !checkingIP6 {
		if !args.Quiet {
			var readableRecordTypes string
			switch { //nolint:wsl
			case args.IP4Managed && args.IP6Managed:
				readableRecordTypes = "A or AAAA"
			case args.IP4Managed:
				readableRecordTypes = "A"
			case args.IP6Managed:
				readableRecordTypes = "AAAA"
			default:
				log.Fatalf("😱 The impossible happened!")
				return false //nolint:nlreturn
			}

			log.Printf("🤷 IP addresses remain the same; no need to check %s records for %s.", readableRecordTypes, domain)
		}

		return true
	}

	zoneName, ok := args.Target.zoneName(args.Context, h)
	if !ok {
		return false
	}

	if !args.Quiet {
		log.Printf("🧐 Found the zone of the domain %s: %s.", domain, zoneName)
	}

	allOk := true

	if checkingIP4 {
		ip, ok := h.updateRecords(&updateRecordsArgs{
			context:    args.Context,
			quiet:      args.Quiet,
			target:     args.Target,
			recordType: "A",
			ip:         args.IP4.To4(),
			ttl:        args.TTL,
			proxied:    args.Proxied,
		})
		if !ok {
			savedIP4s.Delete(domain)
		} else {
			savedIP4s.SetDefault(domain, ip)
		}

		allOk = allOk && ok
	}

	if checkingIP6 {
		ip, ok := h.updateRecords(&updateRecordsArgs{
			context:    args.Context,
			quiet:      args.Quiet,
			target:     args.Target,
			recordType: "AAAA",
			ip:         args.IP6.To16(),
			ttl:        args.TTL,
			proxied:    args.Proxied,
		})
		if !ok {
			savedIP6s.Delete(domain)
		} else {
			savedIP6s.SetDefault(domain, ip)
		}

		allOk = allOk && ok
	}

	if !allOk {
		log.Printf("😡 Failed to update records for the domain %s.", domain)
		return false //nolint:nlreturn
	}

	return true
}
