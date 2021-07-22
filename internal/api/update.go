package api

import (
	"context"
	"log"
	"net"

	"github.com/cloudflare/cloudflare-go"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

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

	matchedIDs, unmatchedIDs, ok := h.listRecordIDs(ctx, domain, args.IPNetwork, args.IP)
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
			if err := h.cf.UpdateDNSRecord(ctx, zone, id, payload); err != nil {
				log.Printf("😡 Failed to update a stale %s record of %s (ID: %s): %v",
					args.IPNetwork.RecordType(), domain, id, err)
				if err = h.cf.DeleteDNSRecord(ctx, zone, id); err != nil {
					log.Printf("😡 Failed to delete the same record (ID: %s): %v", id, err)
					continue
				} else {
					log.Printf("🧟 Deleted the record instead (ID: %s).", id)
					continue
				}
			}

			log.Printf("📝 Updated a stale %s record of %s (ID: %s).", args.IPNetwork.RecordType(), domain, id)

			uptodate = true
			unhandled = unmatchedIDs[i+1:]

			break
		}

		unmatchedIDs = unhandled
	}

	if !uptodate && args.IP != nil {
		if r, err := h.cf.CreateDNSRecord(ctx, zone, payload); err != nil {
			log.Printf("😡 Failed to add a new %s record of %s.", err, domain)
		} else {
			log.Printf("🐣 Added a new %s record of %s (ID: %s).", args.IPNetwork.RecordType(), domain, r.Result.ID)
			uptodate = true
		}
	}

	for _, id := range unmatchedIDs {
		if err := h.cf.DeleteDNSRecord(ctx, zone, id); err != nil {
			log.Printf("😡 Failed to delete a stale %s record of %s (ID: %s): %v", args.IPNetwork.RecordType(), domain, id, err)
		} else {
			log.Printf("🧟 Deleted a stale %s record of %s (ID: %s).", args.IPNetwork.RecordType(), domain, id)
		}
	}

	for _, id := range matchedIDs {
		if err := h.cf.DeleteDNSRecord(ctx, zone, id); err != nil {
			log.Printf("😡 Failed to remove a duplicate %s record of %s (ID: %s): %v", args.IPNetwork.RecordType(), domain, id, err)
		} else {
			log.Printf("👻 Removed a duplicate %s record of %s (ID: %s).", args.IPNetwork.RecordType(), domain, id)
		}
	}

	if !uptodate {
		log.Printf("😡 Failed to update %s records of %s.", args.IPNetwork.RecordType(), domain)
		return nil, false
	}

	return args.IP, true
}

func (h *Handle) Update(ctx context.Context, args *UpdateArgs) bool {
	domain, ok := args.Target.domain(ctx, h)
	if !ok {
		return false
	}

	savedIP, saved := apiCache.savedIP[args.IPNetwork].Get(domain)

	if saved && savedIP.(net.IP).Equal(args.IP) {
		if !args.Quiet {
			log.Printf("🤷 No need to update %s records of %s.", args.IPNetwork.RecordType(), domain)
		}

		return true
	}

	ip, ok := h.updateNoCache(ctx, args)
	if !ok {
		apiCache.savedIP[args.IPNetwork].Delete(domain)

		log.Printf("😡 Failed to update %s records of %s.", args.IPNetwork.RecordType(), domain)
		return false
	}

	apiCache.savedIP[args.IPNetwork].SetDefault(domain, ip)
	return true
}
