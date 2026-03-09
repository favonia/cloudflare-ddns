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

type warningKey struct {
	unit   string
	field  string
	reason string
}

type ambiguityWarnings struct {
	emitted map[warningKey]bool
}

func newAmbiguityWarnings() ambiguityWarnings {
	return ambiguityWarnings{emitted: make(map[warningKey]bool)}
}

func (w ambiguityWarnings) warn(ppfmt pp.PP, unit, field string, count int, fallback string) {
	key := warningKey{unit: unit, field: field, reason: "ambiguous"}
	if w.emitted[key] {
		return
	}
	w.emitted[key] = true
	ppfmt.Noticef(pp.EmojiWarning,
		"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
		unit, field, count, fallback,
	)
}

func (w ambiguityWarnings) warnDuplicateCanonicalTags(ppfmt pp.PP, unit, field string) {
	ppfmt.Noticef(pp.EmojiImpossible,
		"Found duplicate canonical tags in metadata reconciliation for %s field %q; "+
			"this should not happen and please report it at %s",
		unit, field, pp.IssueReportingURL,
	)
}

// New creates a new Setter against one handle-bound ownership scope.
func New(_ppfmt pp.PP, handle api.Handle) (Setter, bool) {
	return setter{Handle: handle}, true
}

// Record represents a DNS record in this package.
type Record struct {
	api.ID
	api.RecordParams
}

// partitionRecords partitions records into desired-target buckets and stale ones.
// matched contains only keys that have at least one matched record.
// unmatched keeps the original target order.
func partitionRecords(
	targets []netip.Addr, rs []api.Record,
) (matched map[netip.Addr][]Record, unmatched []netip.Addr, stale []Record) {
	targetSet := make(map[netip.Addr]struct{}, len(targets))
	for _, target := range targets {
		targetSet[target] = struct{}{}
	}
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

	unmatched = make([]netip.Addr, 0, len(targets))
	for _, target := range targets {
		if len(matched[target]) == 0 {
			unmatched = append(unmatched, target)
		}
	}

	return matched, unmatched, stale
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

func sameDNSRecordParams(left, right api.RecordParams) bool {
	return left.TTL == right.TTL &&
		left.Proxied == right.Proxied &&
		left.Comment == right.Comment &&
		sameTagsByPolicy(left.Tags, right.Tags)
}

func sortRecordsByID(records []Record) {
	slices.SortFunc(records, func(left, right Record) int {
		return cmp.Compare(left.ID, right.ID)
	})
}

func reconcileAndPartitionRecords(
	configured api.RecordParams,
	records []Record,
	ppfmt pp.PP,
	warnings ambiguityWarnings,
	unit string,
) (resolved api.RecordParams, matching []Record, nonMatching []Record) {
	ttlValues := make([]api.TTL, 0, len(records))
	proxiedValues := make([]bool, 0, len(records))
	commentValues := make([]string, 0, len(records))
	tagSets := make([][]string, 0, len(records))
	for _, record := range records {
		ttlValues = append(ttlValues, record.TTL)
		proxiedValues = append(proxiedValues, record.Proxied)
		commentValues = append(commentValues, record.Comment)
		tagSets = append(tagSets, record.Tags)
	}
	tagSummary := summarizeTagSets(tagSets)
	if tagSummary.hasDuplicateCanonical {
		warnings.warnDuplicateCanonicalTags(ppfmt, unit, "tags")
	}
	if tagSummary.hasAmbiguousCanonical {
		warnings.warn(ppfmt, unit, "tags", len(tagSets), "common subset")
	}

	resolvedTTL, ttlAmbiguous := resolveScalarValue(configured.TTL, ttlValues)
	resolvedProxied, proxiedAmbiguous := resolveScalarValue(configured.Proxied, proxiedValues)
	resolvedComment, commentAmbiguous := resolveScalarValue(configured.Comment, commentValues)
	if ttlAmbiguous {
		warnings.warn(ppfmt, unit, "ttl", len(ttlValues), "configured value")
	}
	if proxiedAmbiguous {
		warnings.warn(ppfmt, unit, "proxied", len(proxiedValues), "configured value")
	}
	if commentAmbiguous {
		warnings.warn(ppfmt, unit, "comment", len(commentValues), "configured value")
	}

	resolved = api.RecordParams{
		TTL:     resolvedTTL,
		Proxied: resolvedProxied,
		Comment: resolvedComment,
		Tags:    commonTags(tagSets),
	}
	matching = make([]Record, 0, len(records))
	nonMatching = make([]Record, 0, len(records))
	for _, record := range records {
		if sameDNSRecordParams(record.RecordParams, resolved) {
			matching = append(matching, record)
			continue
		}
		nonMatching = append(nonMatching, record)
	}
	sortRecordsByID(matching)
	sortRecordsByID(nonMatching)
	return resolved, matching, nonMatching
}

func reconcileAndSortRecords(
	configured api.RecordParams,
	records []Record,
	ppfmt pp.PP,
	warnings ambiguityWarnings,
	unit string,
) (resolved api.RecordParams, sorted []Record) {
	resolved, matching, nonMatching := reconcileAndPartitionRecords(
		configured, records, ppfmt, warnings, unit,
	)
	return resolved, slices.Concat(matching, nonMatching)
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

	matchedByIP, unmatchedTargets, staleRecords := partitionRecords(targets, rs)

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
	warnings := newAmbiguityWarnings()
	unit := recordType + " records of " + domainDescription
	matchedTargets := make([]netip.Addr, 0, len(targets)-len(unmatchedTargets))
	targetsToCreate := unmatchedTargets
	for _, target := range targets {
		if len(matchedByIP[target]) > 0 {
			matchedTargets = append(matchedTargets, target)
		}
	}

	// Stage 1: stale-first operations for unmatched targets.
	createParams, staleRecords := reconcileAndSortRecords(
		expectedParams, staleRecords, ppfmt, warnings, unit,
	)

	mutated := false
	for _, target := range targetsToCreate {
		if len(staleRecords) > 0 {
			// Recycle is an optimization of delete+create after metadata reconciliation.
			// UpdateRecord contract: apply target IP and createParams metadata.
			recycled := staleRecords[0]
			staleRecords = staleRecords[1:]
			mutated = true
			if ok := s.Handle.UpdateRecord(ctx, ppfmt, ipNetwork, domain, recycled.ID, target,
				recycled.RecordParams, createParams,
			); !ok {
				ppfmt.Noticef(pp.EmojiError,
					"Could not confirm update of %s records of %s; records might be inconsistent",
					recordType, domainDescription)
				return ResponseFailed
			}
			ppfmt.Noticef(pp.EmojiUpdate,
				"Updated a stale %s record of %s (ID: %s)",
				recordType, domainDescription, recycled.ID)
			continue
		}

		mutated = true
		id, ok := s.Handle.CreateRecord(ctx, ppfmt, ipNetwork, domain, target, createParams)
		if !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Could not confirm update of %s records of %s; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}
		ppfmt.Noticef(pp.EmojiCreation,
			"Added a new %s record of %s (ID: %s)", recordType, domainDescription, id)
	}

	// Stage 2: delete stale/out-of-target leftovers.
	for _, r := range staleRecords {
		mutated = true
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipNetwork, domain, r.ID, api.RegularDelitionMode); !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Could not confirm update of %s records of %s; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		ppfmt.Noticef(pp.EmojiDeletion,
			"Deleted a stale %s record of %s (ID: %s)", recordType, domainDescription, r.ID)
	}

	// Stage 3: update kept matched records if metadata reconciliation changes them.
	duplicateMatchedNonMatchingRecords := make([]Record, 0)
	duplicateMatchedMatchingRecords := make([]Record, 0)
	for _, target := range matchedTargets {
		matched := matchedByIP[target]
		// Consume this target bucket exactly once.
		matchedByIP[target] = nil

		resolvedParams, matchingCandidates, nonMatchingCandidates := reconcileAndPartitionRecords(
			expectedParams, matched, ppfmt, warnings, unit,
		)
		var keptRecord Record
		if len(matchingCandidates) > 0 {
			keptRecord = matchingCandidates[0]
			matchingCandidates = matchingCandidates[1:]
		} else {
			keptRecord = nonMatchingCandidates[0]
			nonMatchingCandidates = nonMatchingCandidates[1:]
		}

		collectDuplicates := func(candidates []Record, targetBucket *[]Record) {
			for _, record := range candidates {
				if record.ID == keptRecord.ID {
					ppfmt.Noticef(pp.EmojiImpossible,
						"Found repeated managed record ID %s among %s records of %s; skipping duplicate deletion for this impossible case",
						keptRecord.ID, recordType, domainDescription,
					)
					continue
				}
				*targetBucket = append(*targetBucket, record)
			}
		}
		collectDuplicates(matchingCandidates, &duplicateMatchedMatchingRecords)
		collectDuplicates(nonMatchingCandidates, &duplicateMatchedNonMatchingRecords)

		if !sameDNSRecordParams(keptRecord.RecordParams, resolvedParams) {
			mutated = true
			// Same-IP update is intentional here: reconcile metadata on the kept record.
			if ok := s.Handle.UpdateRecord(ctx, ppfmt, ipNetwork, domain, keptRecord.ID, target,
				keptRecord.RecordParams, resolvedParams,
			); !ok {
				ppfmt.Noticef(pp.EmojiError,
					"Could not confirm update of %s records of %s; records might be inconsistent",
					recordType, domainDescription)
				return ResponseFailed
			}
			ppfmt.Noticef(pp.EmojiUpdate,
				"Updated a matched %s record of %s (ID: %s)",
				recordType, domainDescription, keptRecord.ID)
		}
	}

	// Stage 4: delete duplicate matched records that do not match resolved metadata.
	for _, r := range duplicateMatchedNonMatchingRecords {
		mutated = true
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipNetwork, domain, r.ID, api.RegularDelitionMode); ok {
			ppfmt.Noticef(pp.EmojiDeletion,
				"Deleted a duplicate %s record of %s (ID: %s)", recordType, domainDescription, r.ID)
		}
		if ctx.Err() != nil {
			if !mutated {
				return ResponseNoop
			}
			return ResponseUpdated
		}
	}
	// Stage 5: delete duplicate matched records that already match resolved metadata.
	for _, r := range duplicateMatchedMatchingRecords {
		mutated = true
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipNetwork, domain, r.ID, api.RegularDelitionMode); ok {
			ppfmt.Noticef(pp.EmojiDeletion,
				"Deleted a duplicate %s record of %s (ID: %s)", recordType, domainDescription, r.ID)
		}
		if ctx.Err() != nil {
			if !mutated {
				return ResponseNoop
			}
			return ResponseUpdated
		}
	}

	if !mutated {
		return ResponseNoop
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
			"Could not confirm deletion of %s records of %s; records might be inconsistent",
			recordType, domainDescription)
		return ResponseFailed
	}

	return ResponseUpdated
}

// SetWAFList updates a WAF list.
//
// The handle returns only items managed by this updater under its bound
// ownership selector.
//
// For each IP family:
// - managed + targets: keep ranges covering any target, add smallest prefixes for uncovered targets
// - managed + empty target set: detection failed, preserve existing family ranges
// - unmanaged: remove all family ranges.
func (s setter) SetWAFList(ctx context.Context, ppfmt pp.PP,
	list api.WAFList, listDescription string, detectedIPs map[ipnet.Type][]netip.Addr, itemComment string,
) ResponseCode {
	type wafFamilyPlan struct {
		createPrefixes []netip.Prefix
		createComment  string
		deleteItems    []api.WAFListItem
	}

	items, alreadyExisting, cached, ok := s.Handle.ListWAFListItems(ctx, ppfmt, list, listDescription, itemComment)
	if !ok {
		return ResponseFailed
	}
	if !alreadyExisting {
		ppfmt.Noticef(pp.EmojiCreation, "Created a new list %s", list.Describe())
	}

	warnings := newAmbiguityWarnings()
	// Plan each family independently, then consume plans in a fixed family order.
	plans := make(map[ipnet.Type]wafFamilyPlan, ipnet.NetworkCount)
	for ipNet := range ipnet.All {
		var plan wafFamilyPlan
		targets, managed := detectedIPs[ipNet]
		if managed && len(targets) == 0 {
			continue // detection was attempted but failed; do nothing
		}

		// Track targets already covered by at least one kept item.
		coveredTargets := make(map[netip.Addr]bool, len(targets))
		for _, item := range items {
			if !ipNet.Matches(item.Prefix.Addr()) {
				continue
			}

			// Unmanaged family: remove all existing items of this family.
			if !managed {
				plan.deleteItems = append(plan.deleteItems, item)
				continue
			}

			// Managed family with targets: keep items that cover at least one
			// target and remember which targets are already covered.
			covered := false
			for _, target := range targets {
				if item.Prefix.Contains(target) {
					coveredTargets[target] = true
					covered = true
				}
			}
			if !covered {
				plan.deleteItems = append(plan.deleteItems, item)
			}
		}

		// Add the smallest allowed prefix for each uncovered target so every
		// managed target ends up covered by at least one list item.
		for _, target := range targets {
			if !coveredTargets[target] {
				plan.createPrefixes = append(plan.createPrefixes,
					netip.PrefixFrom(target, api.WAFListMaxBitLen[ipNet]).Masked())
			}
		}

		slices.SortFunc(plan.createPrefixes, netip.Prefix.Compare)
		plan.createPrefixes = slices.Compact(plan.createPrefixes)

		commentValues := make([]string, 0, len(plan.deleteItems))
		for _, item := range plan.deleteItems {
			commentValues = append(commentValues, item.Comment)
		}
		resolvedComment, ambiguousComment := resolveScalarValue(itemComment, commentValues)
		if ambiguousComment {
			unit := "WAF list " + list.Describe() + " " + ipNet.Describe()
			warnings.warn(ppfmt, unit, "comment", len(commentValues), "configured comment")
		}
		plan.createComment = resolvedComment
		plans[ipNet] = plan
	}

	var itemsToDelete []api.WAFListItem
	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		itemsToDelete = append(itemsToDelete, plans[ipNet].deleteItems...)
	}
	slices.SortFunc(itemsToDelete, func(i, j api.WAFListItem) int {
		return cmp.Or(i.Prefix.Compare(j.Prefix), cmp.Compare(i.ID, j.ID))
	})

	itemsToCreateCount := len(plans[ipnet.IP4].createPrefixes) + len(plans[ipnet.IP6].createPrefixes)

	if itemsToCreateCount == 0 && len(itemsToDelete) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The list %s is already up to date (cached)", list.Describe())
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The list %s is already up to date", list.Describe())
		}
		return ResponseNoop
	}

	// Create first, then delete, to avoid temporary coverage gaps on partial failures.
	itemsToCreate := make([]api.WAFListCreateItem, 0, itemsToCreateCount)
	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		plan := plans[ipNet]
		for _, prefix := range plan.createPrefixes {
			itemsToCreate = append(itemsToCreate, api.WAFListCreateItem{
				Prefix:  prefix,
				Comment: plan.createComment,
			})
		}
	}
	slices.SortFunc(itemsToCreate, func(left, right api.WAFListCreateItem) int {
		return cmp.Or(left.Prefix.Compare(right.Prefix), cmp.Compare(left.Comment, right.Comment))
	})

	if len(itemsToCreate) > 0 {
		if !s.Handle.CreateWAFListItems(ctx, ppfmt, list, listDescription, itemsToCreate) {
			ppfmt.Noticef(pp.EmojiError,
				"Could not confirm update of the list %s; its content may be inconsistent", list.Describe())
			return ResponseFailed
		}
		for _, item := range itemsToCreate {
			ppfmt.Noticef(pp.EmojiCreation, "Added %s to the list %s",
				ipnet.DescribePrefixOrIP(item.Prefix), list.Describe())
		}
	}

	idsToDelete := make([]api.ID, 0, len(itemsToDelete))
	for _, item := range itemsToDelete {
		idsToDelete = append(idsToDelete, item.ID)
	}
	if !s.Handle.DeleteWAFListItems(ctx, ppfmt, list, listDescription, itemComment, idsToDelete) {
		ppfmt.Noticef(pp.EmojiError, "Could not confirm update of the list %s; its content may be inconsistent",
			list.Describe())
		return ResponseFailed
	}
	for _, item := range itemsToDelete {
		ppfmt.Noticef(pp.EmojiDeletion, "Deleted %s from the list %s",
			ipnet.DescribePrefixOrIP(item.Prefix), list.Describe())
	}

	return ResponseUpdated
}

// FinalClearWAFList removes managed WAF content during shutdown.
func (s setter) FinalClearWAFList(ctx context.Context, ppfmt pp.PP, list api.WAFList, listDescription string,
) ResponseCode {
	switch s.Handle.FinalCleanWAFList(ctx, ppfmt, list, listDescription) {
	case api.WAFListCleanupNoop:
		return ResponseNoop
	case api.WAFListCleanupUpdated:
		return ResponseUpdated
	case api.WAFListCleanupUpdating:
		return ResponseUpdating
	case api.WAFListCleanupFailed:
		return ResponseFailed
	default:
		return ResponseFailed
	}
}
