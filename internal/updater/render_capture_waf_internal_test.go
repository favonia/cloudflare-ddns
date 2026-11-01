package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// Defensive snapshots exercise the consuming renderer's identity boundaries.
// They are deliberately outside claims about ordinary setter/service admission.
func defensiveWAFRenderingScenarios() []renderingScenario {
	item := func(id, prefix string) api.WAFListItem {
		return api.WAFListItem{ID: api.ID(id), Prefix: netip.MustParsePrefix(prefix), Comment: "owned"}
	}
	old, added := item("old", "192.0.2.1/32"), item("added", "192.0.2.2/32")
	scenarios := make([]renderingScenario, 0, 11)
	for _, variant := range []string{"prefix-only-created-identity", "returned-identity-unreliable", "returned-identity-missing", "returned-identity-conflicting", "duplicate-prefix-addition", "duplicate-prefix-cleanup", "cleanup-untrusted-identity", "absent-baseline-response", "prefix-only-later-unconfirmed", "cleanup-started-partial-target", "cleanup-completed-partial-target"} {
		before := &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old}}
		returned := &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old, added}}
		result := setter.WAFResult{Code: setter.ResponseUpdated, Before: before, Cleanup: nil, Batches: []setter.WAFBatch{
			{Create: []api.WAFListCreateItem{{Prefix: added.Prefix, Comment: added.Comment}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: returned, ReturnedItems: []api.WAFListItem{old, added}}},
			{Create: nil, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchNoEffect, Returned: nil, ReturnedItems: nil}},
		}}
		switch variant {
		case "prefix-only-created-identity", "prefix-only-later-unconfirmed":
			before.Items = nil
			returned.Items = nil
			outside := added
			outside.Comment = "outside-selector"
			result.Batches[0].Result.ReturnedItems = []api.WAFListItem{outside}
			if variant == "prefix-only-later-unconfirmed" {
				before.Items = []api.WAFListItem{old}
				result.Code = setter.ResponseFailed
				result.Batches[1] = setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{old}, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchUnconfirmed, Returned: nil, ReturnedItems: nil}}
			}
		case "returned-identity-unreliable":
			returned.Items[1].ID = old.ID
			result.Batches[0].Result.ReturnedItems = returned.Items
		case "returned-identity-missing":
			returned.Items[1].ID = ""
			result.Batches[0].Result.ReturnedItems = returned.Items
		case "returned-identity-conflicting":
			result.Batches[0].Result.ReturnedItems[1].Comment = "conflicting-view"
		case "duplicate-prefix-addition":
			before.Items = nil
			duplicate := added
			duplicate.ID = "duplicate"
			returned.Items = []api.WAFListItem{added, duplicate}
			result.Batches[0].Result.ReturnedItems = returned.Items
		case "duplicate-prefix-cleanup", "cleanup-untrusted-identity", "cleanup-started-partial-target", "cleanup-completed-partial-target":
			duplicate := old
			duplicate.ID = "duplicate"
			before.Items = append(before.Items, duplicate)
			if variant == "cleanup-started-partial-target" || variant == "cleanup-completed-partial-target" {
				before.Items[1] = added
			}
			if variant == "cleanup-untrusted-identity" {
				before.Items[1].ID = old.ID
				before.Items[1].Prefix = added.Prefix
				result.Code = setter.ResponseUpdating
			}
			status, cleanupCode := api.WAFBatchCompleted, api.WAFListCleanupUpdated
			if variant == "cleanup-untrusted-identity" || variant == "cleanup-started-partial-target" {
				result.Code = setter.ResponseUpdating
				status, cleanupCode = api.WAFBatchAccepted, api.WAFListCleanupUpdating
			}
			result.Batches = nil
			result.Cleanup = &api.WAFCleanupResult{Code: cleanupCode, Before: before, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{old}, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: status, Returned: nil, ReturnedItems: nil}}}}
		case "absent-baseline-response":
			result.Before = nil
		}
		o := wafOutcome{filtered: true, scope: wafScope{ipnet.IP4: true}, list: api.WAFList{AccountID: "account", Name: "list"}, cleanup: result.Cleanup != nil, result: result}
		responses := emptySetterWAFListResponses()
		responses.register(o.list.Name, result.Code)
		message := generateUpdateWAFListsMessage(responses)
		if o.cleanup {
			message = generateFinalClearWAFListsMessage(responses)
		}
		message.waf = []wafOutcome{o}
		scenarios = append(scenarios, renderingScenario{name: "defensive-waf-" + variant, message: message})
	}
	return scenarios
}

// Capture assertions target semantic evidence and cardinality, leaving the full
// human-facing strings available for exact-output review.
func TestNotificationDefensiveWAFCaptureContracts(t *testing.T) {
	t.Parallel()
	for _, scenario := range defensiveWAFRenderingScenarios() {
		o := scenario.message.waf[0]
		content := o.content()
		switch scenario.name {
		case "defensive-waf-prefix-only-created-identity":
			require.Empty(t, content.exclusions)
			require.NotNil(t, content.observed)
			require.Empty(t, content.observed.Items)
			require.False(t, content.deltaKnown)
			require.Equal(t, api.WAFBatchCompleted, o.result.Batches[0].Result.Status)
		case "defensive-waf-prefix-only-later-unconfirmed":
			require.Empty(t, content.exclusions)
			require.Nil(t, content.observed)
		case "defensive-waf-returned-identity-unreliable", "defensive-waf-returned-identity-missing", "defensive-waf-returned-identity-conflicting":
			require.Contains(t, content.issues, wafReturnedIdentityInvalid)
			require.NotContains(t, content.issues, wafReturnedContentsUnavailable)
			require.Nil(t, content.observed)
			require.Equal(t, api.WAFBatchCompleted, o.result.Batches[0].Result.Status)
		case "defensive-waf-duplicate-prefix-addition":
			require.True(t, content.deltaKnown)
			require.Len(t, content.observed.Items, 2)
			require.Len(t, content.added, 1)
		case "defensive-waf-duplicate-prefix-cleanup":
			require.True(t, content.deltaKnown)
			require.Empty(t, content.removed)
			require.Len(t, content.unchanged, 1)
		case "defensive-waf-cleanup-untrusted-identity":
			require.False(t, o.acceptedFullCleanup(o.configuredSnapshot(content.before)))
			require.False(t, content.deltaKnown)
		case "defensive-waf-cleanup-started-partial-target":
			require.False(t, o.acceptedFullCleanup(o.configuredSnapshot(content.before)))
			require.Nil(t, content.observed)
		case "defensive-waf-cleanup-completed-partial-target":
			require.True(t, content.deltaKnown)
			require.Len(t, content.removed, 1)
			require.Len(t, content.unchanged, 1)
		case "defensive-waf-absent-baseline-response":
			require.Nil(t, content.before)
			require.NotNil(t, content.observed)
			require.False(t, content.deltaKnown)
		}
	}
}
