package setter

import (
	"context"
	"net/netip"
	"sort"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type setter struct {
	Handle api.Handle
}

// partitionRecords partitions record maps into matched and unmatched ones.
func partitionRecords(rmap map[string]netip.Addr, target netip.Addr) (matchedIDs, unmatchedIDs []string) {
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

// New creates a new Setter.
func New(_ppfmt pp.PP, handle api.Handle) (Setter, bool) {
	return &setter{
		Handle: handle,
	}, true
}

// Set calls the DNS service API to update the API of one domain.
//
//nolint:funlen,cyclop,gocognit
func (s *setter) Set(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipnet ipnet.Type, ip netip.Addr, ttl api.TTL, proxied bool) bool { //nolint:lll
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rs, ok := s.Handle.ListRecords(ctx, ppfmt, domain, ipnet)
	if !ok {
		ppfmt.Errorf(pp.EmojiError, "Failed to retrieve the current %s records of %q", recordType, domainDescription)
		return false
	}

	// The intention of these two lists is to find or create a good record and then delete everything else.
	matchedIDs, unmatchedIDsToUpdate := partitionRecords(rs, ip)

	// Whether the correct DNS record is already present
	uptodate := false

	// If ip is not valid, it is considered "up to date", but we have to delete all existing records
	if !ip.IsValid() {
		// matchedIDs must be empty, due to how partitionRecords works
		uptodate = true
	}

	// duplicateMatchedIDs are to be deleted; this is set when uptodate becomes true
	var duplicateMatchedIDs []string

	// If it's still not up to date, and there's a matched ID, use it and delete everything else.
	// Note that this implies ip is valid, for otherwise uptodate would already be true.
	if !uptodate && len(matchedIDs) > 0 {
		uptodate = true
		duplicateMatchedIDs = matchedIDs[1:]
	}

	// If it's up to date and there are no other records, we are done!
	if uptodate && len(duplicateMatchedIDs) == 0 && len(unmatchedIDsToUpdate) == 0 {
		ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", recordType, domainDescription)
		return true
	}

	// This counts the stale records that have not being deleted yet.
	// We need a different variable (instead of using len(unmatchedIDsToUpdate) all the times)
	// because when we fail to delete a record, we will give up and remove that record from
	// unmatchedIDsToUpdate, but that stale record should still be counted in numUndeletedUnmatched
	// so that we know we have failed to complete the updating.
	numUndeletedUnmatched := len(unmatchedIDsToUpdate)

	// If somehow it's still not up to date, it means there are no matched records but ip is valid.
	// This means we have to change a record or create a new one with the desired ip.
	if !uptodate {
		// Temporary variable for the new unmatchedIDsToUpdate
		var unhandled []string

		// Let's go through all stale records
		for i, id := range unmatchedIDsToUpdate {
			// Let's try to update it first.
			if s.Handle.UpdateRecord(ctx, ppfmt, domain, ipnet, id, ip) {
				// If the updating succeeds, we can move on to the next stage!
				ppfmt.Noticef(pp.EmojiUpdateRecord,
					"Updated a stale %s record of %q (ID: %s)", recordType, domainDescription, id)

				// Now it's up to date! matchedIDs must be empty for otherwise uptodate would have been true
				uptodate = true
				numUndeletedUnmatched--
				unhandled = unmatchedIDsToUpdate[i+1:]

				break
			} else {
				// If the updating fails, we will delete it.
				if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
					ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)",
						recordType, domainDescription, id)

					// Only when the deletion succeeds, we decrease the counter for undeleted incorrect records.
					numUndeletedUnmatched--
				}

				// No matter whether the deletion succeeds, move on.
				continue
			}
		}

		unmatchedIDsToUpdate = unhandled
	}

	// If it's still not up to date at this point, it means there are no stale records or that all attempts to
	// update one of them failed. The last resort is to create a new record with the correct ip.
	if !uptodate {
		if id, ok := s.Handle.CreateRecord(ctx, ppfmt,
			domain, ipnet, ip, ttl, proxied); ok {
			ppfmt.Noticef(pp.EmojiAddRecord, "Added a new %s record of %q (ID: %s)", recordType, domainDescription, id)

			// Now it's up to date! matchedIDs and unmatchedIDsToUpdate must both be empty at this point
			uptodate = true
		}
	}

	// Now, we should try to delete all remaining stale records.
	for _, id := range unmatchedIDsToUpdate {
		if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", recordType, domainDescription, id)
			numUndeletedUnmatched--
		}
	}

	// We should also delete all duplicate records even if they are up to date.
	// This has lower priority than deleting the stale records.
	for _, id := range duplicateMatchedIDs {
		if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a duplicate %s record of %q (ID: %s)",
				recordType, domainDescription, id)
		}
	}

	// Check whether we are done. It is okay to have duplicates, but it is not okay to have (undeleted) stale records.
	if !uptodate || numUndeletedUnmatched > 0 {
		ppfmt.Errorf(pp.EmojiError,
			"Failed to complete updating of %s records of %q; records might be inconsistent",
			recordType, domainDescription)
		return false
	}

	return true
}
