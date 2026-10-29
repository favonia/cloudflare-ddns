package updater

import (
	"fmt"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func wafPrefixes(items []api.WAFListItem) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.Prefix.String())
	}
	slices.Sort(values)
	return slices.Compact(values)
}

// wafInventoryValues preserves item cardinality when several trustworthy
// identities share one prefix. Prefix-set deltas alone cannot express that fact.
func wafInventoryValues(items []api.WAFListItem) []string {
	values := wafPrefixes(items)
	for i, value := range values {
		count := 0
		for _, item := range items {
			if item.Prefix.String() == value {
				count++
			}
		}
		if count > 1 {
			values[i] = fmt.Sprintf("%s (%d items)", value, count)
		}
	}
	return values
}

func wafBatchStatus(result *api.WAFBatchResult) string {
	if result == nil {
		return "not attempted"
	}
	switch result.Status {
	case api.WAFBatchNoEffect:
		return "blocked"
	case api.WAFBatchUnconfirmed:
		return "unconfirmed"
	case api.WAFBatchCompleted:
		return "completed"
	case api.WAFBatchAccepted:
		return "started"
	}
	return "unconfirmed"
}

// renderWAF consumes classified evidence without reconstructing reconciliation.
// Inventory and delta eligibility are independent. Whole-input context covers
// configured scope; only successful deltas and supplemental family rows split it.
func renderWAF(o wafOutcome) string {
	content := o.content()
	before := o.configuredSnapshot(content.before)
	cached := before != nil && before.Cached
	var lines []string
	if o.result.Cleanup != nil {
		for _, stage := range o.result.Cleanup.Stages {
			if stage.WholeList {
				label := "List deletion "
				if stage.Result.Status == api.WAFBatchCompleted || stage.Result.Status == api.WAFBatchAccepted {
					label = "Cleanup "
				}
				lines = append(lines, label+wafBatchStatus(&stage.Result))
			}
		}
	}
	switch {
	case o.code == setter.ResponseNoop:
		lines = append(lines, renderWAFContext(o, content.observed, false, "")...)
		if content.observed == nil {
			lines = append(lines, "No update needed")
		}
	case content.deltaKnown:
		if before != nil && !before.Exists {
			lines = append(lines, renderWAFContext(o, before, true, "")...)
		}
		lines = append(lines, renderDelta(wafPrefixes(content.added), wafPrefixes(content.removed),
			wafPrefixes(content.unchanged), cached)...)
		family := o.activeFamilyLabel()
		singular, plural := "item", "items"
		if family != "" {
			singular, plural = family+" item", family+" items"
		}
		// Unique-prefix cardinality does not establish a unique item. Use the full
		// supported result identities to qualify only/all conclusions.
		afterItems := o.activeItems(content.observed)
		pureAddition := len(content.added) > 0 && len(content.removed) == 0 && len(content.unchanged) == 0
		if pureAddition && len(afterItems) != len(wafPrefixes(afterItems)) {
			lines = append(lines, renderWAFContext(o, content.observed, false, "")...)
		} else {
			lines = append(lines, completenessEnding(singular, plural,
				len(content.added), len(content.removed), len(content.unchanged))...)
			if len(content.added)+len(content.removed)+len(content.unchanged) == 0 {
				lines = append(lines, renderWAFContext(o, content.observed, false, "")...)
			} else {
				lines = append(lines, o.renderInactiveWAF(content)...)
			}
		}
	default:
		lines = append(lines, o.renderWAFProgress(content, cached)...)
		fullCleanup := o.acceptedFullCleanup(before)
		if content.observed != nil {
			timing := ""
			if content.inputHistorical && content.observed.Exists &&
				len(content.observed.Items) == 0 && o.hasCompletedAddition() {
				timing = " after the addition"
			}
			lines = append(lines, renderWAFContext(o, content.observed, false, timing)...)
		} else if !fullCleanup {
			lines = append(lines, renderWAFContext(o, before, content.inputHistorical, "")...)
		}
	}
	if len(lines) == 0 {
		action := "Update blocked"
		if o.cleanup {
			action = "Cleanup blocked"
			if o.code == setter.ResponseUpdated {
				action = "Cleanup completed"
			}
			if o.code == setter.ResponseUpdating {
				action = "Cleanup started"
			}
		}
		lines = append(lines, action)
	}
	return "WAF list " + o.list.Describe() + "\n" + strings.Join(lines, "\n")
}

func (o wafOutcome) hasCompletedAddition() bool {
	return slices.ContainsFunc(o.result.Batches, func(batch setter.WAFBatch) bool {
		return len(batch.Create) > 0 && batch.Result != nil && batch.Result.Status == api.WAFBatchCompleted
	})
}

// activeFamilyLabel scopes conclusions only when configured families were
// unavailable. A full configured observation needs no supplemental family row.
func (o wafOutcome) activeFamilyLabel() string {
	inactive := false
	var active []string
	for _, family := range []ipnet.Family{ipnet.IP4, ipnet.IP6} {
		if o.configured[family] && !o.active[family] {
			inactive = true
		}
		if o.configured[family] && o.active[family] {
			active = append(active, family.Describe())
		}
	}
	if inactive {
		return englishValues(active)
	}
	return ""
}

func (o wafOutcome) renderInactiveWAF(content wafContent) []string {
	var lines []string
	snapshot := content.observed
	historical := false
	if snapshot == nil {
		snapshot = o.configuredSnapshot(content.before)
		historical = true
	}
	if snapshot == nil || !snapshot.Exists {
		return nil
	}
	for _, family := range []ipnet.Family{ipnet.IP4, ipnet.IP6} {
		if !o.configured[family] || o.active[family] {
			continue
		}
		subset := cloneWAFSnapshot(snapshot)
		subset.Items = slices.DeleteFunc(subset.Items, func(item api.WAFListItem) bool {
			return !family.Matches(item.Prefix.Addr())
		})
		noun := "Managed " + family.Describe() + " items"
		if historical {
			lines = append(lines, historicalInventory(noun, wafInventoryValues(subset.Items), subset.Cached, o.cleanup))
		} else {
			values := wafInventoryValues(subset.Items)
			text := "none"
			if len(values) > 0 {
				text = englishValues(values)
			}
			lines = append(lines, sourceLabel("Current managed "+family.Describe()+" items", subset.Cached)+": "+text)
		}
	}
	return lines
}

func renderWAFContext(o wafOutcome, snapshot *api.WAFSnapshot, historical bool, timing string) []string {
	if snapshot == nil {
		return nil
	}
	if !snapshot.Exists {
		if historical {
			when := "before the update"
			if o.cleanup {
				when = "before cleanup"
			}
			return []string{sourceLabel("The list did not exist "+when, snapshot.Cached) + "."}
		}
		return []string{sourceLabel("Current", snapshot.Cached) + ": list does not exist"}
	}
	noun, label, empty := "Items", "Current", "no items"
	if o.filtered {
		noun, label, empty = "Managed items", "Current managed items", "none"
	}
	values := wafInventoryValues(snapshot.Items)
	if historical {
		return []string{historicalInventory(noun, values, snapshot.Cached, o.cleanup)}
	}
	text := empty
	if len(values) > 0 {
		text = englishValues(values)
	}
	return []string{sourceLabel(label+timing, snapshot.Cached) + ": " + text}
}

func (o wafOutcome) renderWAFProgress(content wafContent, cached bool) []string {
	var lines []string
	lastCompleted := -1
	for i, batch := range o.result.Batches {
		if batch.Result != nil && batch.Result.Status == api.WAFBatchCompleted && len(batch.Create)+len(batch.Delete) > 0 {
			lastCompleted = i
		}
	}
	for index, batch := range o.result.Batches {
		if batch.Result != nil && batch.Result.PrerequisiteMutation && batch.Result.Status == api.WAFBatchNoEffect {
			lines = append(lines, "List creation unconfirmed")
		}
		if len(batch.Create) > 0 {
			var values []string
			for _, item := range batch.Create {
				values = append(values, item.Prefix.String())
			}
			lines = append(lines, "Addition "+wafBatchStatus(batch.Result)+": "+englishValues(values))
		}
		if len(batch.Delete) > 0 {
			lines = append(lines, sourceLabel("Removal "+wafBatchStatus(batch.Result), cached)+
				": "+englishValues(wafPrefixes(batch.Delete)))
		}
		for _, exclusion := range content.exclusions {
			if exclusion.batchIndex == index {
				lines = append(lines, "Item for "+exclusion.item.Prefix.String()+
					" excluded from management: its comment unexpectedly changed and no longer matches the filter.")
			}
		}
		if index == lastCompleted {
			for _, issue := range content.issues {
				switch issue {
				case wafReturnedContentsUnavailable:
					lines = append(lines, "Could not read the returned list contents.")
				case wafReturnedIdentityInvalid:
					lines = append(lines, "Returned list items could not be identified reliably.")
				}
			}
		}
	}
	if o.result.Cleanup != nil {
		before := o.configuredSnapshot(content.before)
		for _, stage := range o.result.Cleanup.Stages {
			if stage.WholeList {
				continue
			}
			if stage.Result.Status == api.WAFBatchAccepted {
				label := "Cleanup started"
				full := o.acceptedFullCleanup(before)
				if full && !cached {
					label += " for all managed items"
				}
				if len(stage.Items) > 0 {
					label += ": " + englishValues(wafPrefixes(stage.Items))
				}
				lines = append(lines, label)
				if full && cached {
					lines = append(lines, "These were all the managed items (cached).")
				}
			} else {
				lines = append(lines, sourceLabel("Removal "+wafBatchStatus(&stage.Result), cached)+
					": "+englishValues(wafPrefixes(stage.Items)))
			}
		}
	}
	return lines
}

// acceptedFullCleanup proves targets equal the entire trustworthy configured
// input by identity, before allowing operands to replace historical inventory.
func (o wafOutcome) acceptedFullCleanup(before *api.WAFSnapshot) bool {
	if before == nil || !before.Exists || len(before.Items) == 0 || o.result.Cleanup == nil {
		return false
	}
	var targets []api.WAFListItem
	for _, stage := range o.result.Cleanup.Stages {
		if stage.WholeList {
			continue
		}
		if stage.Result.Status != api.WAFBatchAccepted || !trustworthyWAFItems(stage.Items) {
			return false
		}
		targets = append(targets, stage.Items...)
	}
	if !trustworthyWAFItems(targets) || len(targets) != len(before.Items) {
		return false
	}
	for _, item := range before.Items {
		if !slices.Contains(targets, item) {
			return false
		}
	}
	return true
}
