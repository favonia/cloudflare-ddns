package api

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/patrickmn/go-cache"
)

type Handle struct {
	cf *cloudflare.API
}

const (
	// CacheExpirationBaseline specifies the time after which we should recheck the zone IDs,
	// zone names, and/or IP records on the server even if we believe nothing has changed.
	CacheExpirationBaseline = time.Hour
)

var (
	ip4Remembered  *cache.Cache
	ip6Remembered  *cache.Cache
	zoneNameOfID   *cache.Cache
	zoneIDOfDomain *cache.Cache
)

func init() {
	ip4Remembered = cache.New(CacheExpirationBaseline, CacheExpirationBaseline*4)
	ip6Remembered = cache.New(CacheExpirationBaseline, CacheExpirationBaseline*4)
	zoneNameOfID = cache.New(CacheExpirationBaseline, CacheExpirationBaseline*4)
	zoneIDOfDomain = cache.New(CacheExpirationBaseline, CacheExpirationBaseline*4)
}

func newWithKey(key, email string) (*Handle, error) {
	handle, err := cloudflare.New(key, email)
	if err != nil {
		return nil, fmt.Errorf("🤔 The token-based CloudFlare authentication failed: %v", err)
	}
	return &Handle{cf: handle}, nil
}

func newWithToken(token string) (*Handle, error) {
	handle, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		return nil, fmt.Errorf("🤔 The key-based CloudFlare authentication failed: %v", err)
	}
	return &Handle{cf: handle}, nil
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

func (h *Handle) zoneID(ctx context.Context, domain string) (string, error) {
	// try the whole domain as the zone
	var zone *cloudflare.Zone

	zones, err := h.cf.ListZones(ctx, domain)
	if err == nil && len(zones) > 0 {
		zone = &zones[0]
	}

	if zone == nil {
		// search for the closetest zone
		domainSlice := []byte(domain)
		for i, b := range domainSlice {
			if b != '.' {
				continue
			}
			zoneName := string(domainSlice[i+1:])
			zones, err = h.cf.ListZones(ctx, zoneName)
			if err == nil && len(zones) > 0 {
				zone = &zones[0]
				break
			}
		}
	}

	if zone == nil {
		return "", fmt.Errorf("🤔 Could not find the zone for the domain %s.", domain)
	} else {
		zoneNameOfID.SetDefault(zone.ID, zone.Name)
		zoneIDOfDomain.SetDefault(domain, zone.ID)
		return zone.ID, nil
	}
}

// updateRecordsArgs is the type of (named) arguments to updateRecords
type updateRecordsArgs = struct {
	context    context.Context
	quiet      bool
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

	query := cloudflare.DNSRecord{Name: domain, Type: args.recordType}
	rs, err := h.cf.DNSRecords(args.context, zoneID, query)
	if err != nil {
		return nil, err
	}
	uptodate := false

	// delete every record if the ip is `nil`
	if args.ip == nil {
		uptodate = true
	}

	for _, r := range rs {
		if r.Name != domain {
			return nil, fmt.Errorf("🤯 Unexpected domain %s when the domain %s: %+v", r.Name, domain, r)
		}
		if r.Type != args.recordType {
			return nil, fmt.Errorf("🤯 Unexpected %s records when handling %s records: %+v", r.Type, args.recordType, r)
		}
	}

	/*
		Find the entries that match, if any. If there is already a record that is up-to-date,
		then we will delete the first stale record instead of updating it.
	*/
	for _, r := range rs {
		if args.ip.Equal(net.ParseIP(r.Content)) {
			if !args.quiet {
				log.Printf("😃 Found an up-to-date %s record for the domain %s.", args.recordType, domain)
			}
			uptodate = true
			break
		}
	}

	payload := cloudflare.DNSRecord{
		Name:    domain,
		Type:    args.recordType,
		Content: args.ip.String(),
		TTL:     args.ttl,
		Proxied: &args.proxied,
	}

	num_matched := 0
	for _, r := range rs {
		if args.ip.Equal(net.ParseIP(r.Content)) {
			uptodate = true
			num_matched++
			if num_matched > 1 {
				log.Printf("👻 Removing a duplicate %s record (ID: %s) . . .", args.recordType, r.ID)
				err := h.cf.DeleteDNSRecord(args.context, zoneID, r.ID)
				if err != nil {
					log.Printf("😡 Could not remove the record: %v", err)
				}
			}
		} else {
			if uptodate {
				log.Printf("🧟 Deleting a stale %s record (ID: %s) that was pointing to %s . . .", args.recordType, r.ID, r.Content)
				err := h.cf.DeleteDNSRecord(args.context, zoneID, r.ID)
				if err != nil {
					log.Printf("😡 Could not delete the record: %v", err)
				}
			} else {
				log.Printf("📝 Updating a stale %s record (ID: %s) from %s to %v . . .", args.recordType, r.ID, r.Content, args.ip)
				h.cf.UpdateDNSRecord(args.context, zoneID, r.ID, payload)
				if err != nil {
					log.Printf("😡 Could not update the record: %v", err)
					log.Printf("🧟 Deleting the record instead . . .")
					err := h.cf.DeleteDNSRecord(args.context, zoneID, r.ID)
					if err != nil {
						log.Printf("😡 Could not delete the record, either: %v", err)
					}
				} else {
					uptodate = true
					num_matched++
				}
			}
		}
	}

	// The remaining case: there aren't any records to begin with!
	if !uptodate {
		log.Printf("👶 Adding a new %s record: %+v", args.recordType, payload)
		_, err := h.cf.CreateDNSRecord(args.context, zoneID, payload)
		if err != nil {
			log.Printf("😡 Could not add the record: %v", err)
		} else {
			uptodate = true
			num_matched++
		}
	}

	if !uptodate {
		return nil, fmt.Errorf("😡 Failed to update %s records for the domain %s.", args.recordType, domain)
	} else {
		return args.ip, nil
	}
}

type UpdateArgs struct {
	Context    context.Context
	Target     Target
	IP4Managed bool
	IP4        net.IP
	IP6Managed bool
	IP6        net.IP
	TTL        int
	Proxied    bool
	Quiet      bool
}

func (h *Handle) Update(args *UpdateArgs) error {
	domain, err := args.Target.domain(args.Context, h)
	if err != nil {
		return err
	}

	checkingIP4 := false
	if args.IP4Managed {
		previousIP4, remembered := ip4Remembered.Get(domain)
		checkingIP4 = !(remembered && previousIP4.(*net.IP).Equal(args.IP4))
	}

	checkingIP6 := false
	if args.IP6Managed {
		previousIP6, remembered := ip6Remembered.Get(domain)
		checkingIP6 = !(remembered && previousIP6.(*net.IP).Equal(args.IP6))
	}

	if !checkingIP4 && !checkingIP6 {
		if !args.Quiet {
			log.Printf("🤷 Nothing to do for the domain %s; skipping the updating.", domain)
		}
		return nil
	}

	zoneName, err := args.Target.zoneName(args.Context, h)
	if err != nil {
		return err
	}

	if !args.Quiet {
		log.Printf("🧐 Found the zone rooted at %s for the domain %s.", zoneName, domain)
	}

	var (
		err4, err6 error
		ip         net.IP
	)

	if checkingIP4 {
		ip, err4 = h.updateRecords(&updateRecordsArgs{
			context:    args.Context,
			quiet:      args.Quiet,
			target:     args.Target,
			recordType: "A",
			ip:         args.IP4.To4(),
			ttl:        args.TTL,
			proxied:    args.Proxied,
		})
		if err4 != nil {
			ip4Remembered.Delete(domain)
		} else {
			ip4Remembered.SetDefault(domain, (*net.IP)(&ip))
		}
	}

	if checkingIP6 {
		ip, err6 = h.updateRecords(&updateRecordsArgs{
			context:    args.Context,
			quiet:      args.Quiet,
			target:     args.Target,
			recordType: "AAAA",
			ip:         args.IP6.To16(),
			ttl:        args.TTL,
			proxied:    args.Proxied,
		})
		if err6 != nil {
			ip6Remembered.Delete(domain)
		} else {
			ip6Remembered.SetDefault(domain, (*net.IP)(&ip))
		}
	}

	if err4 != nil || err6 != nil {
		return fmt.Errorf("😡 Failed to update records for the domain %s.", domain)
	} else {
		return nil
	}
}
