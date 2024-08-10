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

// SanityCheck calls [api.Handle.SanityCheck].
func (s setter) SanityCheck(ctx context.Context, ppfmt pp.PP) bool {
	return s.Handle.SanityCheck(ctx, ppfmt)
}

// partitionRecords partitions record maps into matched and unmatched ones.
//
// The target IP is assumed to be non-zero.
func partitionRecords(rs []api.Record, target netip.Addr) (matchedIDs, unmatchedIDs []string) {
	for _, r := range rs {
		if r.IP == target {
			matchedIDs = append(matchedIDs, r.ID)
		} else {
			unmatchedIDs = append(unmatchedIDs, r.ID)
		}
	}

	return matchedIDs, unmatchedIDs
}

// Set updates the IP address of one domain to the given ip. The IP address (ip) must be non-zero.
//
//nolint:funlen
func (s setter) Set(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipnet ipnet.Type, ip netip.Addr, ttl api.TTL, proxied bool, recordComment string,
) ResponseCode {
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, domain, ipnet)
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
				"The %s records of %q are already up to date (cached)",
				recordType, domainDescription)
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone,
				"The %s records of %q are already up to date",
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
		if ok := s.Handle.UpdateRecord(ctx, ppfmt, domain, ipnet, unprocessedUnmatched[0], ip); !ok {
			ppfmt.Warningf(pp.EmojiError,
				"Failed to properly update %s records of %q; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		// If the updating succeeds, we can move on to the next stage!
		//
		// Note that there can still be stale records at this point.
		ppfmt.Noticef(pp.EmojiUpdate,
			"Updated a stale %s record of %q (ID: %s)",
			recordType, domainDescription, unprocessedUnmatched[0])

		// Now it's up to date! Note that unprocessedMatched must be empty
		// otherwise foundMatched would have been true.
		foundMatched = true
		unprocessedUnmatched = unprocessedUnmatched[1:]
	}

	// If it's still not up to date at this point, it means there are no stale records to update.
	// This leaves us no choices---we have to create a new record with the correct IP.
	if !foundMatched {
		id, ok := s.Handle.CreateRecord(ctx, ppfmt, domain, ipnet, ip, ttl, proxied, recordComment)
		if !ok {
			ppfmt.Warningf(pp.EmojiError,
				"Failed to properly update %s records of %q; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		// Note that both unprocessedMatched and unprocessedUnmatched must be empty at this point
		// and the records are updated.
		ppfmt.Noticef(pp.EmojiCreation,
			"Added a new %s record of %q (ID: %s)", recordType, domainDescription, id)
	}

	// Now, we should try to delete all remaining stale records.
	for _, id := range unprocessedUnmatched {
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id); !ok {
			ppfmt.Warningf(pp.EmojiError,
				"Failed to properly update %s records of %q; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		ppfmt.Noticef(pp.EmojiDeletion,
			"Deleted a stale %s record of %q (ID: %s)", recordType, domainDescription, id)
	}

	// We should also delete all duplicate records even if they are up to date.
	// This has lower priority than deleting the stale records.
	for _, id := range unprocessedMatched {
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id); ok {
			ppfmt.Noticef(pp.EmojiDeletion,
				"Deleted a duplicate %s record of %q (ID: %s)", recordType, domainDescription, id)
		}
		if ctx.Err() != nil {
			return ResponseUpdated
		}
	}

	return ResponseUpdated
}

// Delete deletes all managed DNS records.
func (s setter) Delete(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipnet ipnet.Type) ResponseCode {
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, domain, ipnet)
	if !ok {
		return ResponseFailed
	}

	// Sorting is not needed for correctness, but it will make the function deterministic.
	unmatchedIDs := make([]string, 0, len(rs))
	for _, r := range rs {
		unmatchedIDs = append(unmatchedIDs, r.ID)
	}

	if len(unmatchedIDs) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted (cached)", recordType, domainDescription)
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted", recordType, domainDescription)
		}
		return ResponseNoop
	}

	allOk := true
	for _, id := range unmatchedIDs {
		if !s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			allOk = false

			if ctx.Err() != nil {
				ppfmt.Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", ctx.Err())
				return ResponseFailed
			}
			continue
		}

		ppfmt.Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", recordType, domainDescription, id)
	}
	if !allOk {
		ppfmt.Warningf(pp.EmojiError,
			"Failed to properly delete %s records of %q; records might be inconsistent",
			recordType, domainDescription)
		return ResponseFailed
	}

	return ResponseUpdated
}

// SetWAFList updates a WAF list.
//
// If detectedIPs contains a zero (invalid) IP, it means the detection is attempted but failed
// and all matching IP addresses should be preserved.
func (s setter) SetWAFList(ctx context.Context, ppfmt pp.PP,
	listName, listDescription string, detectedIPs map[ipnet.Type]netip.Addr, itemComment string,
) ResponseCode {
	listID, alreadyExisting, ok := s.Handle.EnsureWAFList(ctx, ppfmt, listName, listDescription)
	if !ok {
		return ResponseFailed
	}
	if !alreadyExisting {
		ppfmt.Noticef(pp.EmojiCreation, "Created a new list named %q (ID: %s)", listName, listID)
	}

	items, cached, ok := s.Handle.ListWAFListItems(ctx, ppfmt, listName)
	if !ok {
		return ResponseFailed
	}

	var itemsToDelete []api.WAFListItem
	var itemsToCreate []netip.Prefix
	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		detectedIP, managed := detectedIPs[ipNet]
		covered := false
		for _, item := range items {
			if ipNet.Matches(item.Prefix.Addr()) {
				switch {
				case item.Prefix.Contains(detectedIP):
					covered = true
				case managed && !detectedIP.IsValid():
					// detection was attempted but failed; do nothing
				default:
					itemsToDelete = append(itemsToDelete, item)
				}
			}
		}
		if !covered && detectedIP.IsValid() {
			itemsToCreate = append(itemsToCreate,
				netip.PrefixFrom(detectedIP, api.WAFListMaxBitLen[ipNet]).Masked())
		}
	}

	if len(itemsToCreate) == 0 && len(itemsToDelete) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The list %q (ID: %s) is already up to date (cached)", listName, listID)
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The list %q (ID: %s) is already up to date", listName, listID)
		}
		return ResponseNoop
	}

	if !s.Handle.CreateWAFListItems(ctx, ppfmt, listName, itemsToCreate, itemComment) {
		ppfmt.Warningf(pp.EmojiError, "Failed to properly update the list %q (ID: %s); its content may be inconsistent",
			listName, listID)
		return ResponseFailed
	}
	for _, item := range itemsToCreate {
		ppfmt.Noticef(pp.EmojiCreation, "Added %s to the list %q (ID: %s)",
			ipnet.DescribePrefixOrIP(item), listName, listID)
	}

	idsToDelete := make([]string, 0, len(itemsToDelete))
	for _, item := range itemsToDelete {
		idsToDelete = append(idsToDelete, item.ID)
	}
	if !s.Handle.DeleteWAFListItems(ctx, ppfmt, listName, idsToDelete) {
		ppfmt.Warningf(pp.EmojiError, "Failed to properly update the list %q (ID: %s); its content may be inconsistent",
			listName, listID)
		return ResponseFailed
	}
	for _, item := range itemsToDelete {
		ppfmt.Noticef(pp.EmojiDeletion, "Deleted %s from the list %q (ID: %s)",
			ipnet.DescribePrefixOrIP(item.Prefix), listName, listID)
	}

	return ResponseUpdated
}

// DeleteWAFList deletes a WAF list.
func (s setter) DeleteWAFList(ctx context.Context, ppfmt pp.PP, listName string) ResponseCode {
	if !s.Handle.DeleteWAFList(ctx, ppfmt, listName) {
		return ResponseFailed
	}
	ppfmt.Noticef(pp.EmojiDeletion, "The list %q was deleted", listName)
	return ResponseUpdated
}
