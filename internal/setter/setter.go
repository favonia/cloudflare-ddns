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

type ambiguityWarnings struct {
	emitted map[string]struct{}
}

func newAmbiguityWarnings() ambiguityWarnings {
	return ambiguityWarnings{emitted: make(map[string]struct{})}
}

func (w ambiguityWarnings) warn(ppfmt pp.PP, unit, field string, count int, fallback string) {
	key := unit + "|" + field
	if _, seen := w.emitted[key]; seen {
		return
	}
	w.emitted[key] = struct{}{}
	ppfmt.Noticef(pp.EmojiWarning,
		"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
		unit, field, count, fallback,
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

func sameDNSRecordParams(left, right api.RecordParams) bool {
	return left.TTL == right.TTL &&
		left.Proxied == right.Proxied &&
		left.Comment == right.Comment &&
		sameTagsByPolicy(left.Tags, right.Tags)
}

func selectLowestRecordID(records []Record) int {
	selected := 0
	for i := 1; i < len(records); i++ {
		if records[i].ID < records[selected].ID {
			selected = i
		}
	}
	return selected
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
	warnings := newAmbiguityWarnings()
	unit := recordType + " records of " + domainDescription
	duplicateMatchedRecords := make([]Record, 0)
	targetsToCreate := make([]netip.Addr, 0)
	for _, target := range targets {
		if matched := matchedByIP[target]; len(matched) > 0 {
			keepIndex := 0
			if len(matched) > 1 {
				ttlValues := make([]api.TTL, 0, len(matched))
				proxiedValues := make([]bool, 0, len(matched))
				commentValues := make([]string, 0, len(matched))
				tagSets := make([][]string, 0, len(matched))
				for _, candidate := range matched {
					ttlValues = append(ttlValues, candidate.TTL)
					proxiedValues = append(proxiedValues, candidate.Proxied)
					commentValues = append(commentValues, candidate.Comment)
					tagSets = append(tagSets, candidate.Tags)
				}

				resolvedTTL, ttlAmbiguous := resolveScalarValue(expectedParams.TTL, ttlValues)
				resolvedProxied, proxiedAmbiguous := resolveScalarValue(expectedParams.Proxied, proxiedValues)
				resolvedComment, commentAmbiguous := resolveScalarValue(expectedParams.Comment, commentValues)
				if ttlAmbiguous {
					warnings.warn(ppfmt, unit, "ttl", len(ttlValues), "configured value")
				}
				if proxiedAmbiguous {
					warnings.warn(ppfmt, unit, "proxied", len(proxiedValues), "configured value")
				}
				if commentAmbiguous {
					warnings.warn(ppfmt, unit, "comment", len(commentValues), "configured value")
				}

				resolvedParams := api.RecordParams{
					TTL:     resolvedTTL,
					Proxied: resolvedProxied,
					Comment: resolvedComment,
					Tags:    commonTags(tagSets),
				}

				matchingCandidates := make([]Record, 0, len(matched))
				for _, candidate := range matched {
					if sameDNSRecordParams(candidate.RecordParams, resolvedParams) {
						matchingCandidates = append(matchingCandidates, candidate)
					}
				}

				if len(matchingCandidates) > 0 {
					keepID := matchingCandidates[selectLowestRecordID(matchingCandidates)].ID
					for i, candidate := range matched {
						if candidate.ID == keepID {
							keepIndex = i
							break
						}
					}
				} else {
					keepIndex = selectLowestRecordID(matched)
				}
			}

			for i, record := range matched {
				if i == keepIndex {
					continue
				}
				duplicateMatchedRecords = append(duplicateMatchedRecords, record)
			}
			matchedByIP[target] = nil
			continue
		}

		if len(staleRecords) > 0 {
			// Recycle a stale record before creating a new one to preserve record metadata.
			recycled := staleRecords[0]
			if ok := s.Handle.UpdateRecord(ctx, ppfmt, ipNetwork, domain, recycled.ID, target,
				recycled.RecordParams, expectedParams,
			); !ok {
				ppfmt.Noticef(pp.EmojiError,
					"Could not confirm update of %s records of %s; records might be inconsistent",
					recordType, domainDescription)
				return ResponseFailed
			}
			ppfmt.Noticef(pp.EmojiUpdate,
				"Updated a stale %s record of %s (ID: %s)",
				recordType, domainDescription, recycled.ID)
			staleRecords = staleRecords[1:]
			continue
		}

		targetsToCreate = append(targetsToCreate, target)
	}

	sourceRecords := make([]Record, 0, len(staleRecords)+len(duplicateMatchedRecords))
	sourceRecords = append(sourceRecords, staleRecords...)
	sourceRecords = append(sourceRecords, duplicateMatchedRecords...)

	ttlValues := make([]api.TTL, 0, len(sourceRecords))
	proxiedValues := make([]bool, 0, len(sourceRecords))
	commentValues := make([]string, 0, len(sourceRecords))
	tagSets := make([][]string, 0, len(sourceRecords))
	for _, source := range sourceRecords {
		ttlValues = append(ttlValues, source.TTL)
		proxiedValues = append(proxiedValues, source.Proxied)
		commentValues = append(commentValues, source.Comment)
		tagSets = append(tagSets, source.Tags)
	}
	createTTL, ttlAmbiguous := resolveScalarValue(expectedParams.TTL, ttlValues)
	createProxied, proxiedAmbiguous := resolveScalarValue(expectedParams.Proxied, proxiedValues)
	createComment, commentAmbiguous := resolveScalarValue(expectedParams.Comment, commentValues)
	if ttlAmbiguous {
		warnings.warn(ppfmt, unit, "ttl", len(ttlValues), "configured value")
	}
	if proxiedAmbiguous {
		warnings.warn(ppfmt, unit, "proxied", len(proxiedValues), "configured value")
	}
	if commentAmbiguous {
		warnings.warn(ppfmt, unit, "comment", len(commentValues), "configured value")
	}
	createParams := api.RecordParams{
		TTL:     createTTL,
		Proxied: createProxied,
		Comment: createComment,
		Tags:    commonTags(tagSets),
	}

	for _, target := range targetsToCreate {
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

	// Delete all remaining stale/out-of-target records.
	for _, r := range staleRecords {
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipNetwork, domain, r.ID, api.RegularDelitionMode); !ok {
			ppfmt.Noticef(pp.EmojiError,
				"Could not confirm update of %s records of %s; records might be inconsistent",
				recordType, domainDescription)
			return ResponseFailed
		}

		ppfmt.Noticef(pp.EmojiDeletion,
			"Deleted a stale %s record of %s (ID: %s)", recordType, domainDescription, r.ID)
	}

	// Delete all duplicate matched records even if they are up to date.
	// This has lower priority than deleting the stale records.
	for _, r := range duplicateMatchedRecords {
		if ok := s.Handle.DeleteRecord(ctx, ppfmt, ipNetwork, domain, r.ID, api.RegularDelitionMode); ok {
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
	plans := map[ipnet.Type]wafFamilyPlan{
		ipnet.IP4: {},
		ipnet.IP6: {},
	}
	for ipNet := range ipnet.All {
		plan := plans[ipNet]
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
				plan.deleteItems = append(plan.deleteItems, item)
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
		return cmp.Or(i.Compare(j.Prefix), cmp.Compare(i.ID, j.ID))
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
	createsByComment := map[string][]netip.Prefix{}
	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		plan := plans[ipNet]
		createsByComment[plan.createComment] = append(createsByComment[plan.createComment], plan.createPrefixes...)
	}

	createComments := make([]string, 0, len(createsByComment))
	for comment, prefixes := range createsByComment {
		if len(prefixes) == 0 {
			continue
		}
		createComments = append(createComments, comment)
	}
	slices.Sort(createComments)
	for _, comment := range createComments {
		prefixes := createsByComment[comment]
		slices.SortFunc(prefixes, netip.Prefix.Compare)
		prefixes = slices.Compact(prefixes)
		if !s.Handle.CreateWAFListItems(ctx, ppfmt, list, listDescription, prefixes, comment) {
			ppfmt.Noticef(pp.EmojiError,
				"Could not confirm update of the list %s; its content may be inconsistent", list.Describe())
			return ResponseFailed
		}
		for _, item := range prefixes {
			ppfmt.Noticef(pp.EmojiCreation, "Added %s to the list %s",
				ipnet.DescribePrefixOrIP(item), list.Describe())
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
