package setter

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	apitags "github.com/favonia/cloudflare-ddns/internal/api/tags"
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

func (w ambiguityWarnings) warn(ppfmt pp.PP, count int, unit, field string, fallback string) {
	key := warningKey{unit: unit, field: field, reason: "ambiguous"}
	if w.emitted[key] {
		return
	}
	w.emitted[key] = true
	ppfmt.Noticef(pp.EmojiWarning,
		"The %d outdated %s disagree on %s; will use %s",
		count, unit, field, fallback,
	)
}

func (w ambiguityWarnings) warnDuplicateCanonicalTags(ppfmt pp.PP, unit string) {
	ppfmt.Noticef(pp.EmojiImpossible,
		"The tags for %s contain duplicates that differ only by letter case; "+
			"this should not happen; please report it at %s",
		unit, pp.IssueReportingURL,
	)
}

// New creates a new Setter against one handle-bound ownership scope.
func New(_ppfmt pp.PP, handle api.Handle) Setter {
	return setter{Handle: handle}
}

// record represents a DNS record in this package.
type record struct {
	api.ID
	api.RecordParams

	// originalIP accompanies each outdated record through metadata sorting and
	// recycling. IDs may be empty or conflicting, so they cannot recover this
	// association from a map. Matching records need only identity and metadata.
	originalIP netip.Addr
}

// partitionRecords partitions records into desired-target buckets and outdated ones.
// matched contains only keys that have at least one matched record.
// unmatched keeps the original target order.
func partitionRecords(
	targets []netip.Addr, rs []api.Record,
) (matched map[netip.Addr][]record, unmatched []netip.Addr, outdated []record) {
	targetSet := make(map[netip.Addr]bool, len(targets))
	for _, target := range targets {
		targetSet[target] = true
	}
	matched = make(map[netip.Addr][]record, len(targetSet))
	outdated = make([]record, 0, len(rs))
	for _, r := range rs {
		// Unmap so IPv4-mapped IPv6 records match canonical IPv4 targets.
		// Invalid or non-target records are intentionally treated as outdated.
		ip := r.IP.Unmap()
		if ip.IsValid() {
			if targetSet[ip] {
				matched[ip] = append(matched[ip], record{ID: r.ID, RecordParams: r.RecordParams})
				continue
			}
		}
		outdated = append(outdated, record{ID: r.ID, RecordParams: r.RecordParams, originalIP: r.IP})
	}

	unmatched = make([]netip.Addr, 0, len(targets))
	for _, target := range targets {
		if len(matched[target]) == 0 {
			unmatched = append(unmatched, target)
		}
	}

	return matched, unmatched, outdated
}

func recordsAlreadyUpToDate(targets []netip.Addr, matched map[netip.Addr][]record, outdated []record) bool {
	if len(outdated) != 0 {
		return false
	}
	for _, target := range targets {
		// Already-satisfying managed records are soft residue, even when more than
		// one record covers the same desired target or their metadata differs.
		if len(matched[target]) == 0 {
			return false
		}
	}
	return true
}

func sameDNSRecordParams(left, right api.RecordParams) bool {
	return left.TTL == right.TTL &&
		left.Proxied == right.Proxied &&
		left.Comment == right.Comment &&
		apitags.Equal(left.Tags, right.Tags)
}

func sortRecordsByID(records []record) {
	slices.SortFunc(records, func(left, right record) int {
		return cmp.Compare(left.ID, right.ID)
	})
}

// resolveScalarValue resolves a scalar value from outdated sources.
// The returned bool is true when outdated values are non-empty and disagree.
func resolveScalarValue[T comparable](fallback T, outdatedValues []T) (T, bool) {
	if len(outdatedValues) == 0 {
		return fallback, false
	}

	candidate := outdatedValues[0]
	for _, value := range outdatedValues[1:] {
		if value != candidate {
			return fallback, true
		}
	}
	return candidate, false
}

func reconcileAndPartitionRecords(
	fallbackParams api.RecordParams,
	records []record,
	ppfmt pp.PP,
	warnings ambiguityWarnings,
	unit string,
) (resolvedParams api.RecordParams, matching []record, nonMatching []record) {
	// Collect all the attributes we want to resolve.
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

	// Actually resolve them.
	// Note: the fallback tag set mentioned in design documents is always empty.
	resolvedTTL, ttlAmbiguous := resolveScalarValue(fallbackParams.TTL, ttlValues)
	resolvedProxied, proxiedAmbiguous := resolveScalarValue(fallbackParams.Proxied, proxiedValues)
	resolvedComment, commentAmbiguous := resolveScalarValue(fallbackParams.Comment, commentValues)
	resolvedTags := apitags.Resolve(tagSets)
	if ttlAmbiguous {
		warnings.warn(ppfmt, len(records), unit, "TTL values",
			fmt.Sprintf("fallback TTL %s", fallbackParams.TTL.Describe()))
	}
	if proxiedAmbiguous {
		warnings.warn(ppfmt, len(records), unit, "proxy statuses",
			fmt.Sprintf(`fallback proxy setting %q`, api.DescribeProxyStatus(fallbackParams.Proxied)))
	}
	if commentAmbiguous {
		warnings.warn(ppfmt, len(records), unit, "comments",
			fmt.Sprintf("fallback comment %s",
				pp.QuotePreviewOrEmptyLabel(fallbackParams.Comment, pp.AdvisoryPreviewLimit, "(empty)")))
	}
	if resolvedTags.HasDuplicateCanonical {
		warnings.warnDuplicateCanonicalTags(ppfmt, unit)
	}
	if resolvedTags.HasAmbiguousCanonical {
		warnings.warn(ppfmt, len(records), unit, "tags",
			fmt.Sprintf("common set (%s), dropping %s",
				pp.EnglishJoinMapOrEmptyLabel(pp.QuoteIfUnsafeInSentence, resolvedTags.Inherited, "no tags"),
				pp.EnglishJoinMapOrEmptyLabel(pp.QuoteIfUnsafeInSentence, resolvedTags.Dropped, "none")))
	}

	// Tags differ from scalar fields: the current config surface has no non-empty
	// fallback tag value, so reconciliation preserves only the canonical tags that
	// every recyclable managed record already has. With today's effective
	// fallback Tags=nil, this is exactly the canonical intersection/common subset.
	resolvedParams = api.RecordParams{
		TTL:     resolvedTTL,
		Proxied: resolvedProxied,
		Comment: resolvedComment,
		Tags:    resolvedTags.Inherited,
	}
	matching = make([]record, 0, len(records))
	nonMatching = make([]record, 0, len(records))
	for _, record := range records {
		if sameDNSRecordParams(record.RecordParams, resolvedParams) {
			matching = append(matching, record)
			continue
		}
		nonMatching = append(nonMatching, record)
	}
	sortRecordsByID(matching)
	sortRecordsByID(nonMatching)
	return resolvedParams, matching, nonMatching
}

func reconcileAndSortRecords(
	fallbackParams api.RecordParams,
	records []record,
	ppfmt pp.PP,
	warnings ambiguityWarnings,
	unit string,
) (resolvedParams api.RecordParams, sorted []record) {
	resolvedParams, matching, nonMatching := reconcileAndPartitionRecords(
		fallbackParams, records, ppfmt, warnings, unit,
	)
	return resolvedParams, slices.Concat(matching, nonMatching)
}

// SetIPs updates the IP addresses of one domain to the given target set.
// Provider output currently reaches this function through an address-only
// specialization of the raw-data model.
// The inputs are assumed to satisfy [Setter.SetIPs] invariants.
func (s setter) SetIPs(ctx context.Context, ppfmt pp.PP,
	ipFamily ipnet.Family, domain domain.Domain, ips []netip.Addr,
	fallbackParams api.RecordParams,
) (result DNSResult) {
	recordType := ipFamily.RecordType()
	domainDescription := domain.Describe()
	targets := ips

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, ipFamily, domain, fallbackParams)
	if !ok {
		result.Code = ResponseFailed
		return result
	}

	result.Before = snapshotRecords(rs, cached)
	result.Progress = &DNSProgress{Satisfied: nil, Operations: nil}

	matchedByIP, unmatchedTargets, outdatedRecords := partitionRecords(targets, rs)
	for _, target := range targets {
		if len(matchedByIP[target]) > 0 {
			result.Progress.Satisfied = append(result.Progress.Satisfied, target)
		}
	}

	// If records already satisfy all desired targets and no outdated managed records
	// remain, we are done. Matching duplicates are tolerated residue.
	if recordsAlreadyUpToDate(targets, matchedByIP, outdatedRecords) {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone,
				"The %s records for %s are already up to date (cached)",
				recordType, domainDescription)
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone,
				"The %s records for %s are already up to date",
				recordType, domainDescription)
		}
		result.Code = ResponseNoop
		return result
	}

	// Satisfy each uncovered target deterministically:
	// 1. recycle one outdated record via update,
	// 2. otherwise create a new record.
	warnings := newAmbiguityWarnings()
	unit := fmt.Sprintf("%s records for %s", recordType, domainDescription)

	// Stage 1: outdated-first operations for unmatched targets.
	// Metadata for these new targets is inherited from recyclable outdated records
	// when they agree; effective fallback values are used only when those records
	// are absent or disagree.
	resolvedParamsForNewTargets, outdatedRecords := reconcileAndSortRecords(
		fallbackParams, outdatedRecords, ppfmt, warnings, unit,
	)

	// Build the ordered work once, then execute that same list. Each outdated
	// record already owns its original address before API-borrowed records mutate.
	for _, target := range unmatchedTargets {
		operation := DNSOperation{
			Kind: api.RecordCreate, Record: api.RecordFact{ID: "", IP: target},
			Status: DNSOperationNotAttempted, Membership: api.MembershipUnknown,
		}
		if len(outdatedRecords) > 0 {
			operation.Kind = api.RecordUpdate
			operation.Record.ID = outdatedRecords[0].ID
			outdatedRecords = outdatedRecords[1:]
		}
		result.Progress.Operations = append(result.Progress.Operations, operation)
	}
	for _, r := range outdatedRecords {
		result.Progress.Operations = append(result.Progress.Operations, DNSOperation{
			Kind: api.RecordDelete, Record: api.RecordFact{ID: r.ID, IP: r.originalIP},
			Status: DNSOperationNotAttempted, Membership: api.MembershipUnknown,
		})
	}

	for i, operation := range result.Progress.Operations {
		var effect api.RecordMutation
		switch operation.Kind {
		case api.RecordUpdate:
			effect = s.Handle.UpdateRecord(ctx, ppfmt, ipFamily, domain, operation.Record.ID,
				operation.Record.IP, resolvedParamsForNewTargets)
		case api.RecordCreate:
			effect = s.Handle.CreateRecord(ctx, ppfmt, ipFamily, domain, operation.Record.IP, resolvedParamsForNewTargets)
		case api.RecordDelete:
			effect = s.Handle.DeleteRecord(ctx, ppfmt, ipFamily, domain, operation.Record.ID, api.RegularDeletionMode)
		}
		if !result.admit(i, effect) {
			result.Code = ResponseFailed
			return result
		}
		switch operation.Kind {
		case api.RecordUpdate:
			ppfmt.Noticef(pp.EmojiUpdate,
				"Updated an outdated %s record for %s (ID: %s)", recordType, domainDescription, operation.Record.ID)
		case api.RecordCreate:
			ppfmt.Noticef(pp.EmojiCreation,
				"Added a new %s record for %s (ID: %s)", recordType, domainDescription, effect.Record.ID)
		case api.RecordDelete:
			ppfmt.Noticef(pp.EmojiDeletion,
				"Deleted an outdated %s record for %s (ID: %s)", recordType, domainDescription, operation.Record.ID)
		}
	}

	if len(result.Progress.Operations) == 0 {
		result.Code = ResponseNoop
		return result
	}

	result.Code = ResponseUpdated
	return result
}

// FinalDelete deletes all managed DNS records.
func (s setter) FinalDelete(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, domain domain.Domain,
	fallbackParams api.RecordParams,
) (result DNSResult) {
	recordType := ipFamily.RecordType()
	domainDescription := domain.Describe()

	rs, cached, ok := s.Handle.ListRecords(ctx, ppfmt, ipFamily, domain, fallbackParams)
	if !ok {
		result.Code = ResponseFailed
		return result
	}

	result.Before = snapshotRecords(rs, cached)
	result.Progress = &DNSProgress{Satisfied: nil, Operations: nil}

	// Freeze every deletion, including record/address association, before the first
	// request can mutate the API's borrowed cache slice. Preserve listing order.
	for _, fact := range result.Before.Records {
		result.Progress.Operations = append(result.Progress.Operations, DNSOperation{
			Kind: api.RecordDelete, Record: fact,
			Status: DNSOperationNotAttempted, Membership: api.MembershipUnknown,
		})
	}

	if len(result.Progress.Operations) == 0 {
		if cached {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records for %s were already deleted (cached)", recordType, domainDescription) //nolint:lll
		} else {
			ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records for %s were already deleted", recordType, domainDescription)
		}
		result.Code = ResponseNoop
		return result
	}

	allOK := true
	for i, operation := range result.Progress.Operations {
		id := operation.Record.ID
		if !result.admit(i, s.Handle.DeleteRecord(ctx, ppfmt, ipFamily, domain, id, api.FinalDeletionMode)) {
			allOK = false

			if ctx.Err() != nil {
				ppfmt.Infof(pp.EmojiTimeout,
					"Deletion of %s records for %s was aborted by a timeout or signal",
					recordType, domainDescription)
				result.Code = ResponseFailed
				return result
			}
			continue
		}

		ppfmt.Noticef(pp.EmojiDeletion, "Deleted an outdated %s record for %s (ID: %s)", recordType, domainDescription, id)
	}
	if !allOK {
		result.Code = ResponseFailed
		return result
	}

	result.Code = ResponseUpdated
	return result
}

// SetWAFList updates a WAF list.
//
// The handle returns only items managed by this updater under its bound
// ownership selector.
//
// For each IP family:
// - available targets: keep ranges covering any target, add smallest prefixes for uncovered targets
// - explicit empty (available + empty target set): delete managed family ranges
// - target unavailable: preserve managed family ranges
// - out of scope: preserve managed family ranges.
func (s setter) SetWAFList(ctx context.Context, ppfmt pp.PP,
	list api.WAFList, listDescription string,
	targetsByFamily map[ipnet.Family]WAFTargets, fallbackItemComment string,
) WAFResult {
	result := WAFResult{Before: nil, Batches: nil, Cleanup: nil, Code: ResponseFailed}
	type wafFamilyPlan struct {
		createPrefixes []netip.Prefix
		createComment  string
		deleteItems    []api.WAFListItem
	}

	items, exists, cached, ok := s.Handle.ListWAFListItems(
		ctx, ppfmt, list, listDescription, fallbackItemComment,
	)
	if !ok {
		return result
	}

	result.Before = &api.WAFSnapshot{Exists: exists, Cached: cached, Items: slices.Clone(items)}
	warnings := newAmbiguityWarnings()
	// Plan each family independently, then consume plans in a fixed family order.
	plans := make(map[ipnet.Family]wafFamilyPlan, ipnet.FamilyCount)
	for ipFamily := range ipnet.All {
		targets, managed := targetsByFamily[ipFamily]
		var plan wafFamilyPlan
		if !managed || !targets.HasUsableTargets() {
			continue
		}

		// Track targets already covered by at least one kept item.
		coveredTargets := make(map[netip.Prefix]bool, len(targets.Prefixes))
		for _, item := range items {
			if !ipFamily.Matches(item.Prefix.Addr()) {
				continue
			}

			// Managed family with known targets: keep items that cover at least one
			// target and remember which targets are already covered.
			covered := false
			for _, target := range targets.Prefixes {
				if prefixContainsPrefix(item.Prefix, target) {
					coveredTargets[target] = true
					covered = true
				}
			}
			if !covered {
				plan.deleteItems = append(plan.deleteItems, item)
			}
		}

		for _, target := range targets.Prefixes {
			if !coveredTargets[target] {
				plan.createPrefixes = append(plan.createPrefixes, target.Masked())
			}
		}

		slices.SortFunc(plan.createPrefixes, netip.Prefix.Compare)
		plan.createPrefixes = slices.Compact(plan.createPrefixes)

		commentValues := make([]string, 0, len(plan.deleteItems))
		for _, item := range plan.deleteItems {
			commentValues = append(commentValues, item.Comment)
		}
		resolvedComment, ambiguousComment := resolveScalarValue(fallbackItemComment, commentValues)
		if ambiguousComment {
			unit := fmt.Sprintf("%s items in the WAF list %s", ipFamily.Describe(), list.Describe())
			warnings.warn(ppfmt, len(plan.deleteItems), unit, "comments",
				fmt.Sprintf("fallback comment %s",
					pp.QuotePreviewOrEmptyLabel(fallbackItemComment, pp.AdvisoryPreviewLimit, "(empty)")))
		}
		plan.createComment = resolvedComment
		plans[ipFamily] = plan
	}

	var itemsToDelete []api.WAFListItem
	for _, ipFamily := range []ipnet.Family{ipnet.IP4, ipnet.IP6} {
		itemsToDelete = append(itemsToDelete, plans[ipFamily].deleteItems...)
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
		result.Code = ResponseNoop
		return result
	}

	// Create first, then delete, to avoid temporary coverage gaps on partial failures.
	itemsToCreate := make([]api.WAFListCreateItem, 0, itemsToCreateCount)
	for _, ipFamily := range []ipnet.Family{ipnet.IP4, ipnet.IP6} {
		plan := plans[ipFamily]
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
		result.Batches = append(result.Batches, WAFBatch{Delete: nil, Result: nil, Create: slices.Clone(itemsToCreate)})
	}
	if len(itemsToDelete) > 0 {
		result.Batches = append(result.Batches, WAFBatch{Create: nil, Delete: slices.Clone(itemsToDelete), Result: nil})
	}
	if len(itemsToCreate) > 0 {
		batch := s.Handle.CreateWAFListItems(ctx, ppfmt, list, listDescription, itemsToCreate)
		result.Batches[0].Result = cloneWAFBatchResult(batch)
		if !batch.OK {
			return result
		}
		for _, item := range itemsToCreate {
			ppfmt.Noticef(pp.EmojiCreation, "Added %s to the list %s",
				item.Prefix.Masked().String(), list.Describe())
		}
	}

	idsToDelete := make([]api.ID, 0, len(itemsToDelete))
	for _, item := range itemsToDelete {
		idsToDelete = append(idsToDelete, item.ID)
	}
	if len(itemsToDelete) == 0 {
		result.Batches = append(result.Batches, WAFBatch{Create: nil, Delete: nil, Result: nil})
	}
	batch := s.Handle.DeleteWAFListItems(ctx, ppfmt, list, listDescription, idsToDelete)
	result.Batches[len(result.Batches)-1].Result = cloneWAFBatchResult(batch)
	if !batch.OK {
		return result
	}
	for _, item := range itemsToDelete {
		ppfmt.Noticef(pp.EmojiDeletion, "Deleted %s from the list %s",
			item.Prefix.Masked().String(), list.Describe())
	}

	result.Code = ResponseUpdated
	return result
}

// prefixContainsPrefix reports whether container fully covers target.
// For valid network prefixes, containment is equivalent to containing target's
// base address plus having a prefix length no longer than target's.
func prefixContainsPrefix(container, target netip.Prefix) bool {
	return container.Contains(target.Addr()) && container.Bits() <= target.Bits()
}

// FinalClearWAFList removes managed WAF content during shutdown.
func (s setter) FinalClearWAFList(ctx context.Context, ppfmt pp.PP, list api.WAFList, listDescription string,
	managedFamilies map[ipnet.Family]bool, mode api.CleanupMode,
) WAFResult {
	cleanup := cloneWAFCleanupResult(s.Handle.FinalCleanWAFList(ctx, ppfmt, list, listDescription, managedFamilies, mode))
	result := WAFResult{Batches: nil, Code: ResponseFailed, Before: cleanup.Before, Cleanup: cleanup}
	switch cleanup.Code {
	case api.WAFListCleanupNoop:
		result.Code = ResponseNoop
	case api.WAFListCleanupUpdated:
		result.Code = ResponseUpdated
	case api.WAFListCleanupUpdating:
		result.Code = ResponseUpdating
	case api.WAFListCleanupFailed:
		result.Code = ResponseFailed
	default:
		result.Code = ResponseFailed
	}
	return result
}
