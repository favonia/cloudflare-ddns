package setter

import (
	"cmp"
	"context"
	"net/netip"
	"slices"

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

// partitionRecords partitions records into desired-target buckets and stale ones.
func partitionRecords(
	rs []api.Record, targetSet map[netip.Addr]struct{},
) (matched map[netip.Addr][]Record, stale []Record) {
	matched = make(map[netip.Addr][]Record, len(targetSet))
	stale = make([]Record, 0, len(rs))
	for _, r := range rs {
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

// Set updates the IP address of one domain to the given IP.
// This is the compatibility wrapper around [setter.SetIPs].
// IP is assumed to already satisfy SetIPs input invariants.
func (s setter) Set(ctx context.Context, ppfmt pp.PP,
	ipNetwork ipnet.Type, domain domain.Domain, ip netip.Addr,
	expectedParams api.RecordParams,
) ResponseCode {
	return s.SetIPs(ctx, ppfmt, ipNetwork, domain, []netip.Addr{ip}, expectedParams)
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

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, ipNetwork, domain, expectedParams)
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
			matchedByIP[target] = matched[1:]
			continue
		}

		if len(staleRecords) > 0 {
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
// For each IP family:
// - managed + targets: keep ranges covering any target, add smallest prefixes for uncovered targets
// - managed + empty target set: detection failed, preserve existing family ranges
// - unmanaged: remove all family ranges.
func (s setter) SetWAFList(ctx context.Context, ppfmt pp.PP,
	list api.WAFList, listDescription string, detectedIPs map[ipnet.Type][]netip.Addr, itemComment string,
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
		targets, managed := detectedIPs[ipNet]
		if managed && len(targets) == 0 {
			continue // detection was attempted but failed; do nothing
		}

		coveredTargets := make(map[netip.Addr]struct{}, len(targets))
		for _, item := range items {
			if !ipNet.Matches(item.Addr()) {
				continue
			}
			if !managed {
				itemsToDelete = append(itemsToDelete, item)
				continue
			}

			covered := false
			for _, target := range targets {
				if item.Contains(target) {
					coveredTargets[target] = struct{}{}
					covered = true
				}
			}
			if !covered {
				itemsToDelete = append(itemsToDelete, item)
			}
		}

		for _, target := range targets {
			if _, covered := coveredTargets[target]; !covered {
				itemsToCreate = append(itemsToCreate,
					netip.PrefixFrom(target, api.WAFListMaxBitLen[ipNet]).Masked())
			}
		}
	}

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
