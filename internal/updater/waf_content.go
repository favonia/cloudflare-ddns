package updater

import (
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// wafContent retains admitted input in before separately from terminal evidence.
// observed is the latest trustworthy configured-scope inventory, independent of
// successful reconciliation; nil means unavailable, not empty. inputHistorical
// records a dispatched mutation after admission, including structural creation.
// Issues describe unusable completed responses still not superseded by a usable
// response; later uncertainty alone adds no issue. Exclusions retain supported
// configured-scope ownership transitions at the revealing batch even if later
// work is uncertain.
// Deltas compare actual active-family prefixes only and require every required
// batch's successful completion and consistent membership, never target coverage.
type wafContent struct {
	before          *api.WAFSnapshot
	observed        *api.WAFSnapshot
	inputHistorical bool
	issues          []wafContentIssue
	exclusions      []wafExclusion
	added           []api.WAFListItem
	removed         []api.WAFListItem
	unchanged       []api.WAFListItem
	deltaKnown      bool
}

type wafContentIssue uint8

const (
	wafReturnedContentsUnavailable wafContentIssue = iota
	wafReturnedIdentityInvalid
)

// wafExclusion associates a known-owned identity's newly observed exclusion with
// its response. Prefix equality with a create request never establishes identity.
type wafExclusion struct {
	batchIndex int
	item       api.WAFListItem
}

// configuredSnapshot projects comment-selected API input to configured families.
// Missing configuration means no configured families, never all families.
func (o wafOutcome) configuredSnapshot(snapshot *api.WAFSnapshot) *api.WAFSnapshot {
	if snapshot == nil || !trustworthyWAFItems(snapshot.Items) {
		return nil
	}
	result := cloneWAFSnapshot(snapshot)
	result.Items = nil
	for _, item := range snapshot.Items {
		family := ipnet.IP6
		if item.Prefix.Addr().Is4() {
			family = ipnet.IP4
		}
		if o.configured[family] {
			result.Items = append(result.Items, item)
		}
	}
	return result
}

func (o wafOutcome) activeItems(snapshot *api.WAFSnapshot) []api.WAFListItem {
	if snapshot == nil {
		return nil
	}
	var items []api.WAFListItem
	for _, item := range snapshot.Items {
		family := ipnet.IP6
		if item.Prefix.Addr().Is4() {
			family = ipnet.IP4
		}
		if o.active[family] {
			items = append(items, item)
		}
	}
	return items
}

func (o wafOutcome) content() wafContent {
	input := o.result.Before
	if o.result.Cleanup != nil {
		input = o.result.Cleanup.Before
	}
	content := wafContent{
		before: cloneWAFSnapshot(input), observed: o.configuredSnapshot(input),
		inputHistorical: false, issues: nil, exclusions: nil,
		added: nil, removed: nil, unchanged: nil, deltaKnown: false,
	}
	if o.result.Cleanup != nil {
		o.cleanupContent(&content, input)
		return content
	}
	beforeKnown := input != nil && trustworthyWAFItems(input.Items)
	// Membership guards and exclusions cover configured ownership, including
	// temporarily unavailable families. Raw identity validation remains broader.
	ownedIDs := make(map[api.ID]bool)
	if content.observed != nil {
		for _, item := range content.observed.Items {
			ownedIDs[item.ID] = true
		}
	}
	complete := beforeKnown
	var after *api.WAFSnapshot
	required := false
	for index, batch := range o.result.Batches {
		result := batch.Result
		if result != nil && result.PrerequisiteMutation {
			content.inputHistorical = true
			content.observed = nil
		}
		if len(batch.Create) == 0 && len(batch.Delete) == 0 {
			continue
		}
		required = true
		if result == nil {
			complete = false
			continue
		}
		if result.Status == api.WAFBatchNoEffect {
			complete = false
			continue
		}
		content.inputHistorical = true
		content.observed = nil
		if result.Status != api.WAFBatchCompleted {
			complete = false
			continue
		}
		after = result.Returned
		if after == nil {
			content.issues = []wafContentIssue{wafReturnedContentsUnavailable}
			complete = false
			continue
		}
		if !trustworthyWAFResponse(*result) {
			content.issues = []wafContentIssue{wafReturnedIdentityInvalid}
			complete = false
			continue
		}
		content.observed = o.configuredSnapshot(after)
		content.issues = nil
		if !result.OK {
			complete = false
		}
		if !batchMembershipConsistent(ownedIDs, batch) {
			complete = false
		}
		for _, item := range result.ReturnedItems {
			selected := slices.ContainsFunc(after.Items, func(owned api.WAFListItem) bool { return owned.ID == item.ID })
			if ownedIDs[item.ID] && !selected {
				content.exclusions = append(content.exclusions, wafExclusion{batchIndex: index, item: item})
				delete(ownedIDs, item.ID)
			}
		}
		for _, item := range after.Items {
			delete(ownedIDs, item.ID)
		}
		for _, item := range content.observed.Items {
			ownedIDs[item.ID] = true
		}
	}
	if !required {
		if beforeKnown {
			content.unchanged = o.activeItems(input)
		}
		return content
	}
	if !complete || after == nil {
		return content
	}
	content.setDelta(o.activeItems(input), o.activeItems(after))
	return content
}

// cleanupContent subtracts confirmed deleted identities before prefix projection.
// Successful cleanup effects establish a delta, never a new observed inventory.
// A fallback's input was admitted after any whole-list failure; only subsequent
// item stages can make that admitted input historical.
func (o wafOutcome) cleanupContent(content *wafContent, input *api.WAFSnapshot) {
	known := input != nil && trustworthyWAFItems(input.Items)
	deleted := make(map[api.ID]bool)
	for _, stage := range o.result.Cleanup.Stages {
		if stage.WholeList {
			if input == nil && stage.Result.Status != api.WAFBatchNoEffect {
				content.inputHistorical = true
			}
			continue
		}
		if stage.Result.Status != api.WAFBatchNoEffect || stage.Result.PrerequisiteMutation {
			content.inputHistorical = true
			content.observed = nil
		}
		if !stage.Result.OK || stage.Result.Status != api.WAFBatchCompleted || !trustworthyWAFItems(stage.Items) {
			known = false
			continue
		}
		for _, item := range stage.Items {
			if input == nil || !slices.Contains(input.Items, item) || deleted[item.ID] {
				known = false
			}
			deleted[item.ID] = true
		}
	}
	if !known {
		return
	}
	remaining := cloneWAFSnapshot(input)
	remaining.Items = slices.DeleteFunc(remaining.Items, func(item api.WAFListItem) bool { return deleted[item.ID] })
	content.setDelta(o.activeItems(input), o.activeItems(remaining))
}

func (content *wafContent) setDelta(before, after []api.WAFListItem) {
	content.deltaKnown = true
	before = wafPrefixRepresentatives(before)
	after = wafPrefixRepresentatives(after)
	for _, item := range before {
		if slices.ContainsFunc(after, func(other api.WAFListItem) bool { return other.Prefix == item.Prefix }) {
			content.unchanged = append(content.unchanged, item)
		} else {
			content.removed = append(content.removed, item)
		}
	}
	for _, item := range after {
		if !slices.ContainsFunc(before, func(other api.WAFListItem) bool { return other.Prefix == item.Prefix }) {
			content.added = append(content.added, item)
		}
	}
}

// Identity/cardinality anomalies leave operation evidence intact but cannot
// support a whole-content delta. This does not add a duplicate cleanup policy.
func trustworthyWAFItems(items []api.WAFListItem) bool {
	seen := make(map[api.ID]bool)
	for _, item := range items {
		if item.ID == "" || !item.Prefix.IsValid() || seen[item.ID] {
			return false
		}
		seen[item.ID] = true
	}
	return true
}

// Both returned views must identify items consistently. A supplied full view
// contains the selected items verbatim; legacy omitted full views cannot add
// membership evidence, but do not invalidate an independently selected snapshot.
func trustworthyWAFResponse(result api.WAFBatchResult) bool {
	if !trustworthyWAFItems(result.Returned.Items) || !trustworthyWAFItems(result.ReturnedItems) {
		return false
	}
	if result.ReturnedItems == nil {
		return true
	}
	for _, item := range result.Returned.Items {
		if !slices.Contains(result.ReturnedItems, item) {
			return false
		}
	}
	return true
}

// An actual returned item that left ownership is not evidence of deletion.
// An unselected requested prefix also prevents a complete delta: prefix equality
// cannot identify a created item or establish exclusion, but leaves requested
// managed creation unresolved. Other unowned items remain outside this guard.
func batchMembershipConsistent(ownedIDs map[api.ID]bool, batch setter.WAFBatch) bool {
	for _, item := range batch.Result.ReturnedItems {
		selected := slices.ContainsFunc(batch.Result.Returned.Items, func(owned api.WAFListItem) bool {
			return owned.ID == item.ID
		})
		if selected {
			continue
		}
		if ownedIDs[item.ID] {
			return false
		}
		if slices.ContainsFunc(batch.Create, func(request api.WAFListCreateItem) bool {
			return request.Prefix == item.Prefix
		}) {
			return false
		}
	}
	return true
}

// Content deltas are prefix sets. Retain one original item as each prefix's
// display representative; all identity/comment facts remain in the raw result.
// Unchanged/Removed use input representatives, Added uses returned representatives.
func wafPrefixRepresentatives(items []api.WAFListItem) []api.WAFListItem {
	seen := make(map[netip.Prefix]bool)
	var representatives []api.WAFListItem
	for _, item := range items {
		if !seen[item.Prefix] {
			representatives = append(representatives, item)
			seen[item.Prefix] = true
		}
	}
	return representatives
}
