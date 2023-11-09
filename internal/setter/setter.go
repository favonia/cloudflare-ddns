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
//
// The target ip must be non-zero.
func partitionRecords(rmap map[string]netip.Addr, target netip.Addr) (matchedIDs, unmatchedIDs []string) {
	for id, ip := range rmap {
		if ip == target {
			matchedIDs = append(matchedIDs, id)
		} else {
			unmatchedIDs = append(unmatchedIDs, id)
		}
	}

	// This is to make Set deterministic so that this package is easier to test.
	// Otherwise, sorting is not needed. The performance penalty is small because in most cases
	// the total number of (matched and unmatched) records would be zero or one.
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

// Set updates the IP address of one domain to the given ip. The ip must be non-zero.
//
//nolint:funlen
func (s *setter) Set(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipnet ipnet.Type, ip netip.Addr, ttl api.TTL, proxied bool,
) ResponseCode {
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rs, ok := s.Handle.ListRecords(ctx, ppfmt, domain, ipnet)
	if !ok {
		ppfmt.Errorf(pp.EmojiError, "Failed to retrieve the current %s records of %q", recordType, domainDescription)
		return ResponseUpdatesFailed
	}

	// The intention of these two lists is to find or create a good record and then delete everything else.
	// We prefer recycling existing records (if possible) so that existing TTL and proxy can be preserved.
	// However, when ip is not valid, we will delete all DNS records.
	unprocessedMatched, unprocessedUnmatched := partitionRecords(rs, ip)

	// foundMatched remembers whether the correct DNS record is already present.
	// Stale records may still exist even when foundMatched is true.
	foundMatched := false

	// If it's still not up to date, and there's a matched ID, use it and delete everything else.
	// Note that this implies ip is valid, for otherwise foundMatched would have been true.
	if !foundMatched && len(unprocessedMatched) > 0 {
		foundMatched = true
		unprocessedMatched = unprocessedMatched[1:]
	}

	// If it's up to date and there are no other records, we are done!
	if foundMatched && len(unprocessedMatched) == 0 && len(unprocessedUnmatched) == 0 {
		ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", recordType, domainDescription)
		return ResponseNoUpdatesNeeded
	}

	// This counts the stale records that have not being deleted yet.
	//
	// We need a different counter (instead of using len(unprocessedUnmatched) all the times)
	// because when we fail to delete a record, we will give up and remove that record from
	// unprocessedUnmatched, but the stale record that could not be deleted should still be
	// counted in numUndeletedUnmatched  so that we know we have failed to complete the updating.
	numUndeletedUnmatched := len(unprocessedUnmatched)

	// If somehow it's still not up to date, it means there are no matched records but ip is valid.
	// This means we should update one stale record or create a new one with the desired ip.
	//
	// Again, we prefer updating stale records instead of creating new ones so that we can
	// preserve the current TTL and proxy setting.
	if !foundMatched {
		// Temporary local variable for the new unprocessedUnmatched
		var newUnprocessedUnmatched []string

		// Let's go through all stale records
		for i, id := range unprocessedUnmatched {
			// Let's try to update it first.
			if ok := s.Handle.UpdateRecord(ctx, ppfmt, domain, ipnet, id, ip); !ok {
				// If the updating fails, we will delete it.
				if ok := s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id); ok {
					ppfmt.Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)",
						recordType, domainDescription, id)

					// Only when the deletion succeeds, we decrease the counter of remaining stale records.
					numUndeletedUnmatched--
				}

				// No matter whether the deletion succeeds, move on.
				continue
			}

			// If the updating succeeds, we can move on to the next stage!
			//
			// Note that there can still be stale records at this point.
			ppfmt.Noticef(pp.EmojiUpdateRecord,
				"Updated a stale %s record of %q (ID: %q)", recordType, domainDescription, id)

			// Now it's up to date! Note that unprocessedMatched must be empty
			// otherwise foundMatched would have been true.
			foundMatched = true
			numUndeletedUnmatched--
			newUnprocessedUnmatched = unprocessedUnmatched[i+1:]

			break
		}

		unprocessedUnmatched = newUnprocessedUnmatched
	}

	// If it's still not up to date at this point, it means there are no stale records or that we failed to update
	// any one of them. This leaves us no choices---we have to create a new record with the correct IP.
	if !foundMatched {
		if id, ok := s.Handle.CreateRecord(ctx, ppfmt,
			domain, ipnet, ip, ttl, proxied); ok {
			ppfmt.Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", recordType, domainDescription, id)

			// Now it's up to date! unprocessedMatched and unprocessedUnmatched must both be empty at this point
			foundMatched = true
		}
	}

	// Now, we should try to delete all remaining stale records.
	for _, id := range unprocessedUnmatched {
		if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			ppfmt.Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", recordType, domainDescription, id)
			numUndeletedUnmatched--
		}
	}

	// We should also delete all duplicate records even if they are up to date.
	// This has lower priority than deleting the stale records.
	for _, id := range unprocessedMatched {
		if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			ppfmt.Noticef(pp.EmojiDeleteRecord, "Deleted a duplicate %s record of %q (ID: %q)",
				recordType, domainDescription, id)
		}
	}

	// Check whether we are done. It is okay to have duplicates, but it is not okay to have remaining stale records.
	if !foundMatched || numUndeletedUnmatched > 0 {
		ppfmt.Errorf(pp.EmojiError,
			"Failed to complete updating of %s records of %q; records might be inconsistent",
			recordType, domainDescription)
		return ResponseUpdatesFailed
	}

	return ResponseUpdatesApplied
}

// Delete deletes all managed DNS records.
func (s *setter) Delete(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipnet ipnet.Type) ResponseCode {
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rmap, ok := s.Handle.ListRecords(ctx, ppfmt, domain, ipnet)
	if !ok {
		ppfmt.Errorf(pp.EmojiError, "Failed to retrieve the current %s records of %q", recordType, domainDescription)
		return ResponseUpdatesFailed
	}

	// Sorting is not needed for correctness, but it will make the function deterministic.
	unmatchedIDs := make([]string, 0, len(rmap))
	for id := range rmap {
		unmatchedIDs = append(unmatchedIDs, id)
	}
	sort.Strings(unmatchedIDs)

	if len(unmatchedIDs) == 0 {
		ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted", recordType, domainDescription)
		return ResponseNoUpdatesNeeded
	}

	allOk := true
	for _, id := range unmatchedIDs {
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id); !ok {
			allOk = false
			continue
		}

		ppfmt.Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", recordType, domainDescription, id)
	}
	if !allOk {
		ppfmt.Errorf(pp.EmojiError,
			"Failed to complete deleting of %s records of %q; records might be inconsistent",
			recordType, domainDescription)
		return ResponseUpdatesFailed
	}

	return ResponseUpdatesApplied
}
