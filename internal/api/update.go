package api

import (
	"context"
	"log"
	"net"

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

func splitRecords(rmap map[string]net.IP, target net.IP) (matchedIDs, unmatchedIDs []string) {
	for id, ip := range rmap {
		if ip.Equal(target) {
			matchedIDs = append(matchedIDs, id)
		} else {
			unmatchedIDs = append(unmatchedIDs, id)
		}
	}

	return matchedIDs, unmatchedIDs
}

func (h *Handle) Update(ctx context.Context, args *UpdateArgs) bool { //nolint:funlen,cyclop,gocognit
	domain, ok := args.Target.domain(ctx, h)
	if !ok {
		log.Printf("😡 Failed to update %s records of %s.", args.IPNetwork.RecordType(), domain)
		return false
	}

	rs, ok := h.listRecords(ctx, domain, args.IPNetwork)
	if !ok {
		log.Printf("😡 Failed to update %s records of %s.", args.IPNetwork.RecordType(), domain)
		return false
	}

	matchedIDs, unmatchedIDs := splitRecords(rs, args.IP)

	// whether there was already an up-to-date record
	uptodate := false
	// whether everything works
	numUnmatched := len(unmatchedIDs)

	// delete every record if ip is `nil`
	if args.IP == nil {
		uptodate = true
	}

	if !uptodate && len(matchedIDs) > 0 {
		uptodate = true
		matchedIDs = matchedIDs[1:]
	}

	if uptodate && len(matchedIDs) == 0 && len(unmatchedIDs) == 0 {
		if !args.Quiet {
			log.Printf("🤷 %s records of %s are already up to date.", args.IPNetwork.RecordType(), domain)
		}

		return true
	}

	if !uptodate && args.IP != nil {
		var unhandled []string

		for i, id := range unmatchedIDs {
			if h.updateRecord(ctx, domain, args.IPNetwork, id, args.IP) {
				log.Printf("📡 Updated a stale %s record of %s (ID: %s).", args.IPNetwork.RecordType(), domain, id)

				uptodate = true
				numUnmatched--
				unhandled = unmatchedIDs[i+1:]

				break
			} else {
				if h.deleteRecord(ctx, domain, args.IPNetwork, id) {
					log.Printf("🧟 Deleted a stale %s record of %s instead (ID: %s).", args.IPNetwork.RecordType(), domain, id)
					numUnmatched--
				}
				continue
			}
		}

		unmatchedIDs = unhandled
	}

	if !uptodate && args.IP != nil {
		if id, ok := h.createRecord(ctx, domain, args.IPNetwork, args.IP, args.TTL, args.Proxied); ok {
			log.Printf("🐣 Added a new %s record of %s (ID: %s).", args.IPNetwork.RecordType(), domain, id)
			uptodate = true
		}
	}

	for _, id := range unmatchedIDs {
		if h.deleteRecord(ctx, domain, args.IPNetwork, id) {
			log.Printf("🧟 Deleted a stale %s record of %s (ID: %s).", args.IPNetwork.RecordType(), domain, id)
			numUnmatched--
		}
	}

	for _, id := range matchedIDs {
		if h.deleteRecord(ctx, domain, args.IPNetwork, id) {
			log.Printf("👻 Removed a duplicate %s record of %s (ID: %s).", args.IPNetwork.RecordType(), domain, id)
		}
	}

	if !uptodate || numUnmatched > 0 {
		log.Printf("😡 Failed to update %s records of %s.", args.IPNetwork.RecordType(), domain)
		return false
	}

	return true
}
