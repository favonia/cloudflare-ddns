package setter

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type setter struct {
	Handle api.Handle
}

// New creates a new Setter.
func New(_ppfmt pp.PP, handle api.Handle) (Setter, bool) {
	return setter{
		Handle: handle,
	}, true
}

// Record represents a DNS record in this package.
type Record struct {
	api.ID
	api.RecordParams
}

// partitionRecords partitions record maps into matched and unmatched ones.
//
// The target IP is assumed to be non-zero.
func partitionRecords(rs []api.Record, target netip.Addr) (matchedIDs, unmatchedIDs []Record) {
	for _, r := range rs {
		if r.IP == target {
			matchedIDs = append(matchedIDs, Record{ID: r.ID, RecordParams: r.RecordParams})
		} else {
			unmatchedIDs = append(unmatchedIDs, Record{ID: r.ID, RecordParams: r.RecordParams})
		}
	}

	return matchedIDs, unmatchedIDs
}

// Set updates the IP address of one domain to the given ip. The IP address (ip) must be non-zero.
func (s setter) Set(ctx context.Context, ppfmt pp.PP,
	ipnet ipnet.Type, domain domain.Domain, ip netip.Addr,
	expectedParams api.RecordParams,
) ResponseCode {
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, ipnet, domain, expectedParams)
	if !ok {
		return ResponseFailed
	}

	// The intention of these two lists is to find or create a good record and then delete everything else.
	// We prefer recycling existing records (if possible) so that existing record attributes can be preserved.
	unprocessedMatched, unprocessedUnmatched := partitionRecords(rs, ip)

	// foundMatched remembers whether the correct DNS record is already present.
	// Stale records may still exist even when foundMatched is true.
	foundMatched := false

	// If it's still not up to date, and there's a matched ID, use it and delete everything else.
	if !foundMatched && len(unprocessedMatched) > 0 {
		foundMatched = true
		unprocessedMatched = unprocessedMatched[1:]
	}

	// If it's up to date and there are no other records, we are done!
	if foundMatched && len(unprocessedMatched) == 0 && len(unprocessedUnmatched) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone,
				"The %s records of %s are already up to date (cached)",
				recordType, domainDescription)
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone,
				"The %s records of %s are already up to date",
				recordType, domainDescription)
		}
		return ResponseNoop
	}

	// If somehow it's still not up to date, it means there are no matching records.
	// This means we should update one stale record or create a new one with the desired IP address.
	//
	// Again, we prefer updating stale records instead of creating new ones so that we can
	// preserve the current TTL and proxy setting.
	if !foundMatched && len(unprocessedUnmatched) > 0 {
		if ok := s.Handle.UpdateRecord(ctx, ppfmt, ipnet, domain, unprocessedUnmatched[0].ID, ip,
			unprocessedUnmatched[0].RecordParams, expectedParams,
		); !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to properly update %s records of %s; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		// If the updating succeeds, we can move on to the next stage!
		//
		// Note that there can still be stale records at this point.
		ppfmt.Noticef(pp.EmojiUpdate,
			"Updated a stale %s record of %s (ID: %s)",
			recordType, domainDescription, unprocessedUnmatched[0].ID)

		// Now it's up to date! Note that unprocessedMatched must be empty
		// otherwise foundMatched would have been true.
		foundMatched = true
		unprocessedUnmatched = unprocessedUnmatched[1:]
	}

	// If it's still not up to date at this point, it means there are no stale records to update.
	// This leaves us no choices---we have to create a new record with the correct IP.
	if !foundMatched {
		id, ok := s.Handle.CreateRecord(ctx, ppfmt, ipnet, domain, ip, expectedParams)
		if !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to properly update %s records of %s; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		// Note that both unprocessedMatched and unprocessedUnmatched must be empty at this point
		// and the records are updated.
		ppfmt.Noticef(pp.EmojiCreation,
			"Added a new %s record of %s (ID: %s)", recordType, domainDescription, id)
	}

	// Now, we should try to delete all remaining stale records.
	for _, r := range unprocessedUnmatched {
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipnet, domain, r.ID, api.RegularDelitionMode); !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to properly update %s records of %s; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		ppfmt.Noticef(pp.EmojiDeletion,
			"Deleted a stale %s record of %s (ID: %s)", recordType, domainDescription, r.ID)
	}

	// We should also delete all duplicate records even if they are up to date.
	// This has lower priority than deleting the stale records.
	for _, r := range unprocessedMatched {
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipnet, domain, r.ID, api.RegularDelitionMode); ok {
			ppfmt.Noticef(pp.EmojiDeletion,
				"Deleted a duplicate %s record of %s (ID: %s)", recordType, domainDescription, r.ID)
		}
		if ctx.Err() != nil {
			return ResponseUpdated
		}
	}

	return ResponseUpdated
}

// FinalDelete deletes all managed DNS records.
func (s setter) FinalDelete(ctx context.Context, ppfmt pp.PP, ipnet ipnet.Type, domain domain.Domain,
	expectedParams api.RecordParams,
) ResponseCode {
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, ipnet, domain, expectedParams)
	if !ok {
		return ResponseFailed
	}

	// Sorting is not needed for correctness, but it will make the function deterministic.
	unmatchedIDs := make([]api.ID, 0, len(rs))
	for _, r := range rs {
		unmatchedIDs = append(unmatchedIDs, r.ID)
	}

	if len(unmatchedIDs) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %s were already deleted (cached)", recordType, domainDescription)
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %s were already deleted", recordType, domainDescription)
		}
		return ResponseNoop
	}

	allOK := true
	for _, id := range unmatchedIDs {
		if !s.Handle.DeleteRecord(ctx, ppfmt, ipnet, domain, id, api.FinalDeletionMode) {
			allOK = false

			if ctx.Err() != nil {
				ppfmt.Infof(pp.EmojiTimeout,
					"Deletion of %s records of %s aborted by timeout or signals; records might be inconsistent",
					recordType, domainDescription)
				return ResponseFailed
			}
			continue
		}

		ppfmt.Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", recordType, domainDescription, id)
	}
	if !allOK {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to properly delete %s records of %s; records might be inconsistent",
			recordType, domainDescription)
		return ResponseFailed
	}

	return ResponseUpdated
}

// SetWAFList updates a WAF list.
//
// If detectedRange contains an invalid prefix, it means the detection is attempted but failed.
// If an item is missing from detectedRange, it means it is not managed.
//
// and all matching ranges should be preserved.
func (s setter) SetWAFList(ctx context.Context, ppfmt pp.PP,
	list api.WAFList, listDescription string, detectedRange map[ipnet.Type]netip.Prefix, itemComment string,
) ResponseCode {
	items, alreadyExisting, cached, ok := s.Handle.ListWAFListItems(ctx, ppfmt, list, listDescription)
	if !ok {
		return ResponseFailed
	}
	if !alreadyExisting {
		ppfmt.Noticef(pp.EmojiCreation, "Created a new list %s", list.Describe())
	}

	var itemsToDelete []api.WAFListItem
	var itemsToCreate []netip.Prefix
	for ipNet := range ipnet.All {
		detectedRange, managed := detectedRange[ipNet]
		covered := false
		for _, item := range items {
			if ipNet.Matches(item.Prefix.Addr()) {
				switch {
				case ipnet.ContainsPrefix(item.Prefix, detectedRange):
					covered = true
				case ipnet.ContainsPrefix(detectedRange, item.Prefix):
					// The range in the list is smaller; doesn't hurt to keep it
				case managed && !detectedRange.IsValid():
					// detection was attempted but failed; do nothing
				default:
					itemsToDelete = append(itemsToDelete, item)
				}
			}
		}
		if !covered && detectedRange.IsValid() {
			itemsToCreate = append(itemsToCreate, detectedRange.Masked())
		}
	}

	if len(itemsToCreate) == 0 && len(itemsToDelete) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The list %s is already up to date (cached)", list.Describe())
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The list %s is already up to date", list.Describe())
		}
		return ResponseNoop
	}

	if !s.Handle.CreateWAFListItems(ctx, ppfmt, list, listDescription, itemsToCreate, itemComment) {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to properly update the list %s; its content may be inconsistent", list.Describe())
		return ResponseFailed
	}
	for _, item := range itemsToCreate {
		ppfmt.Noticef(pp.EmojiCreation, "Added %s to the list %s",
			ipnet.DescribePrefixOrIP(item), list.Describe())
	}

	idsToDelete := make([]api.ID, 0, len(itemsToDelete))
	for _, item := range itemsToDelete {
		idsToDelete = append(idsToDelete, item.ID)
	}
	if !s.Handle.DeleteWAFListItems(ctx, ppfmt, list, listDescription, idsToDelete) {
		ppfmt.Noticef(pp.EmojiError, "Failed to properly update the list %s; its content may be inconsistent",
			list.Describe())
		return ResponseFailed
	}
	for _, item := range itemsToDelete {
		ppfmt.Noticef(pp.EmojiDeletion, "Deleted %s from the list %s",
			ipnet.DescribePrefixOrIP(item.Prefix), list.Describe())
	}

	return ResponseUpdated
}

// FinalClearWAFList calls [api.Handle.DeleteWAFList] or [api.Handle.ClearWAFList].
func (s setter) FinalClearWAFList(ctx context.Context, ppfmt pp.PP, list api.WAFList, listDescription string,
) ResponseCode {
	deleted, ok := s.Handle.FinalClearWAFListAsync(ctx, ppfmt, list, listDescription)
	switch {
	case ok && deleted:
		ppfmt.Noticef(pp.EmojiDeletion, "The list %s was deleted", list.Describe())
		return ResponseUpdated
	case ok && !deleted:
		ppfmt.Noticef(pp.EmojiClear, "The list %s is being cleared (asynchronously)", list.Describe())
		return ResponseUpdating
	default:
		return ResponseFailed
	}
}
