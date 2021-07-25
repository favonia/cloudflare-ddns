package api

import (
	"context"
	"fmt"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

// UpdateArgs is the type of (named) arguments to updateRecords.
type UpdateArgs struct {
	Quiet     quiet.Quiet
	IPNetwork ipnet.Type
	IP        net.IP
	Domain    FQDN
	TTL       TTL
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
	domain := args.Domain.String()
	recordType := args.IPNetwork.RecordType()

	rs, ok := h.listRecords(ctx, domain, args.IPNetwork)
	if !ok {
		fmt.Printf("😡 Failed to update %s records of %s.\n", recordType, domain)
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
			fmt.Printf("🤷 The %s records of %s are already up to date.\n", recordType, domain)
		}

		return true
	}

	if !uptodate && args.IP != nil {
		var unhandled []string

		for i, id := range unmatchedIDs {
			if h.updateRecord(ctx, domain, args.IPNetwork, id, args.IP) {
				fmt.Printf("📡 Updated a stale %s record of %s (ID: %s).\n", recordType, domain, id)

				uptodate = true
				numUnmatched--
				unhandled = unmatchedIDs[i+1:]

				break
			} else {
				if h.deleteRecord(ctx, domain, args.IPNetwork, id) {
					fmt.Printf("💀 Deleted a stale %s record of %s instead (ID: %s).\n", recordType, domain, id)
					numUnmatched--
				}
				continue
			}
		}

		unmatchedIDs = unhandled
	}

	if !uptodate && args.IP != nil {
		if id, ok := h.createRecord(ctx, domain, args.IPNetwork, args.IP, args.TTL.Int(), args.Proxied); ok {
			fmt.Printf("🐣 Added a new %s record of %s (ID: %s).\n", recordType, domain, id)
			uptodate = true
		}
	}

	for _, id := range unmatchedIDs {
		if h.deleteRecord(ctx, domain, args.IPNetwork, id) {
			fmt.Printf("💀 Deleted a stale %s record of %s (ID: %s).\n", recordType, domain, id)
			numUnmatched--
		}
	}

	for _, id := range matchedIDs {
		if h.deleteRecord(ctx, domain, args.IPNetwork, id) {
			fmt.Printf("👻 Removed a duplicate %s record of %s (ID: %s).\n", recordType, domain, id)
		}
	}

	if !uptodate || numUnmatched > 0 {
		fmt.Printf("😡 Failed to update %s records of %s.\n", recordType, domain)
		return false
	}

	return true
}
