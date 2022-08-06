package setter

import (
	"context"
	"net/netip"
	"sort"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type setter struct {
	Handle  api.Handle
	TTL     api.TTL
	Proxied bool
}

func splitRecords(rmap map[string]netip.Addr, target netip.Addr) (matchedIDs, unmatchedIDs []string) {
	if target.IsValid() {
		for id, ip := range rmap {
			if ip == target {
				matchedIDs = append(matchedIDs, id)
			} else {
				unmatchedIDs = append(unmatchedIDs, id)
			}
		}
	} else {
		for id := range rmap {
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

func New(_ppfmt pp.PP, handle api.Handle, ttl api.TTL, proxied bool) (Setter, bool) {
	return &setter{
		Handle:  handle,
		TTL:     ttl,
		Proxied: proxied,
	}, true
}

//nolint:funlen,cyclop,gocognit
func (s *setter) Set(ctx context.Context, ppfmt pp.PP, domain api.Domain, ipnet ipnet.Type, ip netip.Addr) bool { //nolint:lll
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rs, ok := s.Handle.ListRecords(ctx, ppfmt, domain, ipnet)
	if !ok {
		ppfmt.Errorf(pp.EmojiError, "Failed to retrieve the current %s records of %q", recordType, domainDescription)
		return false
	}

	// The intention of these two lists is to find or create a good record and then delete everything else.
	matchedIDs, unmatchedIDs := splitRecords(rs, ip)

	// If ip is not valid, this should be vacuously true.
	// If ip is valid, then we will check matchedIDs and unmatchedIDs, but before that,
	// it is considered not up to date.
	uptodate := !ip.IsValid()

	// First, if there's a matched ID, use it and delete everything else.
	// Note that this implies ip is valid, for otherwise uptodate would be true.
	if !uptodate && len(matchedIDs) > 0 {
		uptodate = true
		matchedIDs = matchedIDs[1:]
	}

	// If it's up to date and there's nothing else, we are done!
	if uptodate && len(matchedIDs) == 0 && len(unmatchedIDs) == 0 {
		ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", recordType, domainDescription)
		return true
	}

	// This counts the stale records that have not being deleted yet.
	// We need a different variable (instead of checking len(unmatchedIDs) all the times)
	// because when we fail to delete a record, we might remove it from unmatchedIDs,
	// but that stale record should still be counted in numUndeletedUnmatched
	numUndeletedUnmatched := len(unmatchedIDs)

	// If somehow it's still not up to date, it means there are no matched records but ip is valid.
	// This means we have to change a record or create a new one with the desired ip.
	if !uptodate && ip.IsValid() {
		var unhandled []string

		// Let's go through all stale records
		for i, id := range unmatchedIDs {
			// Let's try to update it first.
			if s.Handle.UpdateRecord(ctx, ppfmt, domain, ipnet, id, ip) {
				ppfmt.Noticef(pp.EmojiUpdateRecord,
					"Updated a stale %s record of %q (ID: %s)", recordType, domainDescription, id)

				uptodate = true
				numUndeletedUnmatched--
				unhandled = unmatchedIDs[i+1:]

				break
			} else {
				// If the updating fails, we will delete it.
				if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
					ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)",
						recordType, domainDescription, id)
					numUndeletedUnmatched--
				}
				// No matter whether the deletion succeeds, move on.
				continue
			}
		}

		unmatchedIDs = unhandled
	}

	// If it's still not up to date, it means there are no stale records or that we fail to update one of them.
	// The last resort is to create a new record with the correct ip.
	// The checking "ip.IsValid()" is redundant but it does not hurt. (This function is too complicated.)
	if !uptodate && ip.IsValid() {
		if id, ok := s.Handle.CreateRecord(ctx, ppfmt,
			domain, ipnet, ip, s.TTL, s.Proxied); ok {
			ppfmt.Noticef(pp.EmojiAddRecord, "Added a new %s record of %q (ID: %s)", recordType, domainDescription, id)
			uptodate = true
		}
	}

	// Now, we should try to delete all remaining stale records.
	for _, id := range unmatchedIDs {
		if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", recordType, domainDescription, id)
			numUndeletedUnmatched--
		}
	}

	// We should also delete all duplicate records even if they are up to date.
	for _, id := range matchedIDs {
		if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a duplicate %s record of %q (ID: %s)",
				recordType, domainDescription, id)
		}
	}

	// It is okay to have duplicates, but it is not okay to have stale records.
	if !uptodate || numUndeletedUnmatched > 0 {
		ppfmt.Errorf(pp.EmojiError,
			"Failed to complete updating of %s records of %q; records might be inconsistent",
			recordType, domainDescription)
		return false
	}

	return true
}
