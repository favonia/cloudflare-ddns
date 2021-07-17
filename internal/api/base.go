package api

import (
	"context"
	"fmt"
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
)

var (
	savedIP4s      *cache.Cache
	savedIP6s      *cache.Cache
	zoneNameOfID   *cache.Cache
	zoneIDOfDomain *cache.Cache
)

// init makes sure the cache exists even if InitCache is not called
func init() {
	InitCache(DefaultCacheExpiration)
}

func InitCache(expiration time.Duration) {
	savedIP4s = cache.New(expiration, expiration*2)
	savedIP6s = cache.New(expiration, expiration*2)
	zoneNameOfID = cache.New(expiration, expiration*2)
	zoneIDOfDomain = cache.New(expiration, expiration*2)
}

func (h Handle) zoneName(ctx context.Context, zoneID string) (string, error) {
	if name, found := zoneNameOfID.Get(zoneID); found {
		return name.(string), nil
	}

	zone, err := h.cf.ZoneDetails(ctx, zoneID)
	if err != nil {
		return "", fmt.Errorf("🤔 Could not retrieve the name of the zone (ID: %s): %v", zoneID, err)
	}

	zoneNameOfID.SetDefault(zoneID, zone.Name)
	return zone.Name, nil
}

// The built-in ZoneIDByName is broken due to the possibility of multiple zones
func (h *Handle) activeZoneIDsByName(ctx context.Context, zoneName string) ([]string, error) {
	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(zoneName, h.cf.AccountID, "active"))
	if err != nil {
		return nil, fmt.Errorf("🤔 Could not check whether there's a zone named %s: %v", zoneName, err)
	}

	var ids []string
	for i := range res.Result {
		ids = append(ids, res.Result[i].ID)
	}
	return ids, nil
}

func (h *Handle) zoneID(ctx context.Context, domain string) (string, error) {
	if id, found := zoneIDOfDomain.Get(domain); found {
		return id.(string), nil
	}

	// try the whole domain as the zone
	zoneName := domain

	zoneIDs, err := h.activeZoneIDsByName(ctx, zoneName)
	if err != nil {
		return "", err
	}
	if len(zoneIDs) > 1 {
		return "", fmt.Errorf("🤔 Found multiple zones named %s. Consider specifying CF_ACCOUNT_ID.", zoneName)
	}
	if len(zoneIDs) == 0 {

		// search for the zone
	zoneSearch:
		for i, b := range domain {
			if b == '.' {
				zoneName = domain[i+1:]
				zoneIDs, err = h.activeZoneIDsByName(ctx, zoneName)
				if err != nil {
					return "", err
				}
				if len(zoneIDs) > 1 {
					return "", fmt.Errorf("🤔 Found multiple zones named %s. Consider specifying CF_ACCOUNT_ID.", zoneName)
				}
				if len(zoneIDs) == 0 {
					continue zoneSearch
				}
				break zoneSearch
			}
		}
	}

	if len(zoneIDs) != 1 {
		return "", fmt.Errorf("🤔 Could not find the zone of the domain %s.", domain)
	}

	zoneIDOfDomain.SetDefault(domain, zoneIDs[0])
	zoneNameOfID.SetDefault(zoneIDs[0], zoneName)
	return zoneIDs[0], nil
}

// updateRecordsArgs is the type of (named) arguments to updateRecords
type updateRecordsArgs = struct {
	context    context.Context
	quiet      quiet.Quiet
	target     Target
	recordType string
	ip         net.IP
	ttl        int
	proxied    bool
}

func (h *Handle) updateRecords(args *updateRecordsArgs) (net.IP, error) {
	domain, err := args.target.domain(args.context, h)
	if err != nil {
		return nil, err
	}

	zoneID, err := args.target.zoneID(args.context, h)
	if err != nil {
		return nil, err
	}

	var (
		matchedIDs   []string // all the record IDs with the same IP
		unmatchedIDs []string // all the record IDs with a different IP
	)

	{
		rs, err := h.cf.DNSRecords(args.context, zoneID, cloudflare.DNSRecord{
			Name: domain,
			Type: args.recordType,
		})
		if err != nil {
			return nil, fmt.Errorf("🤔 Could not retrieve DNS records for the domain %s: %v", domain, err)
		}
		for i := range rs {
			if args.ip.Equal(net.ParseIP(rs[i].Content)) {
				matchedIDs = append(matchedIDs, rs[i].ID)
			} else {
				unmatchedIDs = append(unmatchedIDs, rs[i].ID)
			}
		}
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

	for _, id := range matchedIDs {
		log.Printf("👻 Removing a duplicate %s record (ID: %s) . . .", args.recordType, id)
		if err := h.cf.DeleteDNSRecord(args.context, zoneID, id); err != nil {
			log.Printf("😡 Could not remove the record: %v", err)
		}
	}

	// the data for updating or creating a record
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
			if err := h.cf.UpdateDNSRecord(args.context, zoneID, id, payload); err != nil {
				log.Printf("😡 Could not update the record: %v", err)
				log.Printf("🧟 Deleting the record instead . . .")
				if err := h.cf.DeleteDNSRecord(args.context, zoneID, id); err != nil {
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

	for _, id := range unmatchedIDs {
		log.Printf("🧟 Deleting a stale %s record (ID: %s) . . .", args.recordType, id)
		if err := h.cf.DeleteDNSRecord(args.context, zoneID, id); err != nil {
			log.Printf("😡 Could not delete the record: %v", err)
		}
	}

	if !uptodate && args.ip != nil {
		log.Printf("👶 Adding a new %s record for the domain %s.", args.recordType, domain)
		if _, err := h.cf.CreateDNSRecord(args.context, zoneID, payload); err != nil {
			log.Printf("😡 Could not add the record: %v", err)
		} else {
			uptodate = true
		}
	}

	if !uptodate {
		return nil, fmt.Errorf("😡 Failed to update %s records for the domain %s.", args.recordType, domain)
	}

	return args.ip, nil
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

func (h *Handle) Update(args *UpdateArgs) error {
	domain, err := args.Target.domain(args.Context, h)
	if err != nil {
		return err
	}

	checkingIP4 := false
	if args.IP4Managed {
		savedIP4, saved := savedIP4s.Get(domain)
		checkingIP4 = !(saved && savedIP4.(*net.IP).Equal(args.IP4))
	}

	checkingIP6 := false
	if args.IP6Managed {
		savedIP6, saved := savedIP6s.Get(domain)
		checkingIP6 = !(saved && savedIP6.(*net.IP).Equal(args.IP6))
	}

	if !checkingIP4 && !checkingIP6 {
		if !args.Quiet {
			var readableRecordType string
			switch {
			case args.IP4Managed && args.IP6Managed:
				readableRecordType = "A or AAAA"
			case args.IP4Managed:
				readableRecordType = "A"
			case args.IP6Managed:
				readableRecordType = "AAAA"
			default:
				return fmt.Errorf("😱 The impossible happened!")
			}
			log.Printf("🤷 IP addresses remain the same; no need to check %s records for %s.", readableRecordType, domain)
		}
		return nil
	}

	zoneName, err := args.Target.zoneName(args.Context, h)
	if err != nil {
		return err
	}

	if !args.Quiet {
		log.Printf("🧐 Found the zone of the domain %s: %s.", domain, zoneName)
	}

	var (
		err4, err6 error
	)

	if checkingIP4 {
		ip, err4 := h.updateRecords(&updateRecordsArgs{
			context:    args.Context,
			quiet:      args.Quiet,
			target:     args.Target,
			recordType: "A",
			ip:         args.IP4.To4(),
			ttl:        args.TTL,
			proxied:    args.Proxied,
		})
		if err4 != nil {
			log.Print(err4)
			savedIP4s.Delete(domain)
		} else {
			savedIP4s.SetDefault(domain, (*net.IP)(&ip))
		}
	}

	if checkingIP6 {
		ip, err6 := h.updateRecords(&updateRecordsArgs{
			context:    args.Context,
			quiet:      args.Quiet,
			target:     args.Target,
			recordType: "AAAA",
			ip:         args.IP6.To16(),
			ttl:        args.TTL,
			proxied:    args.Proxied,
		})
		if err6 != nil {
			log.Print(err6)
			savedIP6s.Delete(domain)
		} else {
			savedIP6s.SetDefault(domain, (*net.IP)(&ip))
		}
	}

	if err4 != nil || err6 != nil {
		return fmt.Errorf("😡 Failed to update records for the domain %s.", domain)
	}

	return nil
}
