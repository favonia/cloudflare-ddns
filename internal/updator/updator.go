package updator

import (
	"context"
	"net"
	"sort"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Args is the type of (named) arguments to updateRecords.
type Args struct {
	Handle    api.Handle
	IPNetwork ipnet.Type
	IP        net.IP
	Domain    api.FQDN
	TTL       api.TTL
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

	// This is to make Do deterministic so that this package is easier to test.
	// Otherwise, sorting is not needed. The performance penality should be small
	// because in most cases the total number of (matched and unmached) records
	// would be zero or one.
	sort.Strings(matchedIDs)
	sort.Strings(unmatchedIDs)

	return matchedIDs, unmatchedIDs
}

func Do(ctx context.Context, ppfmt pp.PP, args *Args) bool { //nolint:funlen,cyclop,gocognit
	recordType := args.IPNetwork.RecordType()
	domainDescription := args.Domain.Describe()

	rs, ok := args.Handle.ListRecords(ctx, ppfmt, args.Domain, args.IPNetwork)
	if !ok {
		ppfmt.Errorf(pp.EmojiError, "Failed to (fully) update %s records of %q", recordType, domainDescription)
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
		ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", recordType, domainDescription)
		return true
	}

	if !uptodate && args.IP != nil {
		var unhandled []string

		for i, id := range unmatchedIDs {
			if args.Handle.UpdateRecord(ctx, ppfmt, args.Domain, args.IPNetwork, id, args.IP) {
				ppfmt.Noticef(pp.EmojiUpdateRecord,
					"Updated a stale %s record of %q (ID: %s)", recordType, domainDescription, id)

				uptodate = true
				numUnmatched--
				unhandled = unmatchedIDs[i+1:]

				break
			} else {
				if args.Handle.DeleteRecord(ctx, ppfmt, args.Domain, args.IPNetwork, id) {
					ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)",
						recordType, domainDescription, id)
					numUnmatched--
				}
				continue
			}
		}

		unmatchedIDs = unhandled
	}

	if !uptodate && args.IP != nil {
		if id, ok := args.Handle.CreateRecord(ctx, ppfmt,
			args.Domain, args.IPNetwork, args.IP, args.TTL, args.Proxied); ok {
			ppfmt.Noticef(pp.EmojiAddRecord, "Added a new %s record of %q (ID: %s)", recordType, domainDescription, id)
			uptodate = true
		}
	}

	for _, id := range unmatchedIDs {
		if args.Handle.DeleteRecord(ctx, ppfmt, args.Domain, args.IPNetwork, id) {
			ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", recordType, domainDescription, id)
			numUnmatched--
		}
	}

	for _, id := range matchedIDs {
		if args.Handle.DeleteRecord(ctx, ppfmt, args.Domain, args.IPNetwork, id) {
			ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a duplicate %s record of %q (ID: %s)",
				recordType, domainDescription, id)
		}
	}

	if !uptodate || numUnmatched > 0 {
		ppfmt.Errorf(pp.EmojiError, "Failed to (fully) update %s records of %q", recordType, domainDescription)
		return false
	}

	return true
}
