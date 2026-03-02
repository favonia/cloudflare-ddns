package setter

import (
	"cmp"
	"context"
	"net/netip"
	"regexp"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type setter struct {
	Handle       api.Handle
	RecordFilter api.ManagedRecordFilter
}

// New creates a new Setter and binds one managed-record filter for its lifetime.
// The underlying [api.Handle] is expected to use this stable filter consistently.
//
// A nil regex is allowed and means match-all, consistent with [api.ManagedRecordFilter].
// The normal runtime path still passes a compiled regex from [config.Config.Normalize].
func New(_ppfmt pp.PP, handle api.Handle, managedRecordsCommentRegex *regexp.Regexp) (Setter, bool) {
	return setter{
		Handle:       handle,
		RecordFilter: api.ManagedRecordFilter{CommentRegex: managedRecordsCommentRegex},
	}, true
}

// Record represents a DNS record in this package.
type Record struct {
	api.ID
	api.RecordParams
}

// partitionRecords partitions records into desired-target buckets and stale ones.
func partitionRecords(
	rs []api.Record, targetSet map[netip.Addr]struct{},
) (matched map[netip.Addr][]Record, stale []Record) {
	matched = make(map[netip.Addr][]Record, len(targetSet))
	stale = make([]Record, 0, len(rs))
	for _, r := range rs {
		// Unmap so IPv4-mapped IPv6 records match canonical IPv4 targets.
		// Invalid or non-target records are intentionally treated as stale.
		ip := r.IP.Unmap()
		if ip.IsValid() {
			if _, ok := targetSet[ip]; ok {
				matched[ip] = append(matched[ip], Record{ID: r.ID, RecordParams: r.RecordParams})
				continue
			}
		}
		stale = append(stale, Record{ID: r.ID, RecordParams: r.RecordParams})
	}

	return matched, stale
}

func recordsAlreadyUpToDate(targets []netip.Addr, matched map[netip.Addr][]Record, stale []Record) bool {
	if len(stale) != 0 {
		return false
	}
	for _, target := range targets {
		if len(matched[target]) != 1 {
			return false
		}
	}
	return true
}

// SetIPs updates the IP addresses of one domain to the given target set.
// The inputs are assumed to satisfy [Setter.SetIPs] invariants.
func (s setter) SetIPs(ctx context.Context, ppfmt pp.PP,
	ipNetwork ipnet.Type, domain domain.Domain, ips []netip.Addr,
	expectedParams api.RecordParams,
) ResponseCode {
	recordType := ipNetwork.RecordType()
	domainDescription := domain.Describe()
	targets := ips

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, ipNetwork, domain, s.RecordFilter, expectedParams)
	if !ok {
		return ResponseFailed
	}

	targetSet := make(map[netip.Addr]struct{}, len(targets))
	for _, target := range targets {
		targetSet[target] = struct{}{}
	}
	matchedByIP, staleRecords := partitionRecords(rs, targetSet)

	// If records already match all desired targets (one record per target, with no
	// stale or duplicate leftovers), we are done.
	if recordsAlreadyUpToDate(targets, matchedByIP, staleRecords) {
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

	// Satisfy each target deterministically:
	// 1. keep one matched record if available,
	// 2. otherwise recycle one stale record via update,
	// 3. otherwise create a new record.
	for _, target := range targets {
		if matched := matchedByIP[target]; len(matched) > 0 {
			// Reserve one matching record; remaining matches are duplicates cleaned up later.
			matchedByIP[target] = matched[1:]
			continue
		}

		if len(staleRecords) > 0 {
			// Recycle a stale record before creating a new one to preserve record metadata.
			recycled := staleRecords[0]
			if ok := s.Handle.UpdateRecord(ctx, ppfmt, ipNetwork, domain, recycled.ID, target,
				recycled.RecordParams, expectedParams,
			); !ok {
				ppfmt.Noticef(pp.EmojiError,
					"Failed to properly update %s records of %s; records might be inconsistent",
					recordType, domainDescription)
				return ResponseFailed
			}
			ppfmt.Noticef(pp.EmojiUpdate,
				"Updated a stale %s record of %s (ID: %s)",
				recordType, domainDescription, recycled.ID)
			staleRecords = staleRecords[1:]
			continue
		}

		id, ok := s.Handle.CreateRecord(ctx, ppfmt, ipNetwork, domain, target, expectedParams)
		if !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to properly update %s records of %s; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}
		ppfmt.Noticef(pp.EmojiCreation,
			"Added a new %s record of %s (ID: %s)", recordType, domainDescription, id)
	}

	// Delete all remaining stale/out-of-target records.
	for _, r := range staleRecords {
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipNetwork, domain, r.ID, api.RegularDelitionMode); !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to properly update %s records of %s; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		ppfmt.Noticef(pp.EmojiDeletion,
			"Deleted a stale %s record of %s (ID: %s)", recordType, domainDescription, r.ID)
	}

	// Delete all duplicate matched records even if they are up to date.
	// This has lower priority than deleting the stale records.
	for _, target := range targets {
		for _, r := range matchedByIP[target] {
			if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipNetwork, domain, r.ID, api.RegularDelitionMode); ok {
				ppfmt.Noticef(pp.EmojiDeletion,
					"Deleted a duplicate %s record of %s (ID: %s)", recordType, domainDescription, r.ID)
			}
			if ctx.Err() != nil {
				return ResponseUpdated
			}
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

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, ipnet, domain, s.RecordFilter, expectedParams)
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
// The reconciliation operates only on the WAF items returned by the API handle.
//
// For each IP family:
// - managed + targets: keep ranges covering any target, add smallest prefixes for uncovered targets
// - managed + empty target set: detection failed, preserve existing family ranges
// - unmanaged: remove all family ranges.
func (s setter) SetWAFList(ctx context.Context, ppfmt pp.PP,
	list api.WAFList, listDescription string, detectedIPs map[ipnet.Type][]netip.Addr, itemComment string,
) ResponseCode {
	items, alreadyExisting, cached, ok := s.Handle.ListWAFListItems(
		ctx, ppfmt, list, api.ManagedWAFListItemFilter{CommentRegex: nil}, listDescription,
	)
	if !ok {
		return ResponseFailed
	}
	if !alreadyExisting {
		ppfmt.Noticef(pp.EmojiCreation, "Created a new list %s", list.Describe())
	}

	var itemsToDelete []api.WAFListItem
	var itemsToCreate []netip.Prefix
	for ipNet := range ipnet.All {
		targets, managed := detectedIPs[ipNet]
		if managed && len(targets) == 0 {
			continue // detection was attempted but failed; do nothing
		}

		// Track targets already covered by at least one kept item.
		coveredTargets := make(map[netip.Addr]bool, len(targets))
		for _, item := range items {
			if !ipNet.Matches(item.Addr()) {
				continue
			}

			// Unmanaged family: remove all existing items of this family.
			if !managed {
				itemsToDelete = append(itemsToDelete, item)
				continue
			}

			// Managed family with targets: keep items that cover at least one
			// target and remember which targets are already covered.
			covered := false
			for _, target := range targets {
				if item.Contains(target) {
					coveredTargets[target] = true
					covered = true
				}
			}
			if !covered {
				itemsToDelete = append(itemsToDelete, item)
			}
		}

		// Add the smallest allowed prefix for each uncovered target so every
		// managed target ends up covered by at least one list item.
		for _, target := range targets {
			if !coveredTargets[target] {
				itemsToCreate = append(itemsToCreate,
					netip.PrefixFrom(target, api.WAFListMaxBitLen[ipNet]).Masked())
			}
		}
	}

	// Canonicalize mutation order so WAF updates are deterministic across runs and tests.
	slices.SortFunc(itemsToCreate, netip.Prefix.Compare)
	itemsToCreate = slices.Compact(itemsToCreate)
	slices.SortFunc(itemsToDelete, func(i, j api.WAFListItem) int {
		return cmp.Or(i.Compare(j.Prefix), cmp.Compare(i.ID, j.ID))
	})

	if len(itemsToCreate) == 0 && len(itemsToDelete) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The list %s is already up to date (cached)", list.Describe())
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The list %s is already up to date", list.Describe())
		}
		return ResponseNoop
	}

	// Create first, then delete, to avoid temporary coverage gaps on partial failures.
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
// This is the whole-list shutdown path: it does not try to preserve foreign
// items in a shared WAF list.
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
