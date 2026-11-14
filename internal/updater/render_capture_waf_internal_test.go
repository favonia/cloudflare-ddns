package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// Defensive snapshots exercise the consuming renderer's identity boundaries.
// They are deliberately outside claims about production setter reachability or service admission.
func defensiveWAFRenderingScenarios() []renderingScenario {
	item := func(id, prefix string) api.WAFListItem {
		return api.WAFListItem{ID: api.ID(id), Prefix: netip.MustParsePrefix(prefix), Comment: "owned"}
	}
	old, added := item("old", "192.0.2.1/32"), item("added", "192.0.2.2/32")
	scenarios := make([]renderingScenario, 0, 11)
	for _, tc := range []struct {
		name      string
		configure func(*setter.WAFResult)
		wantKind  notifier.Kind
		check     func(*testing.T, Message)
	}{
		{name: "prefix-only-created-identity", configure: func(result *setter.WAFResult) {
			result.Before.Items = nil
			result.Batches[0].Result.ReturnedManaged.Items = nil
			outside := added
			outside.Comment = "outside-selector"
			result.Batches[0].Result.ReturnedAllItems = []api.WAFListItem{outside}
		}, wantKind: notifier.KindUpdate, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.Empty(t, content.exclusions)
			require.NotNil(t, content.observed)
			require.Empty(t, content.observed.Items)
			require.False(t, content.deltaKnown)
			require.Equal(t, api.WAFBatchCompleted, o.result.Batches[0].Result.Status)
		}},
		{name: "returned-identity-unreliable", configure: func(result *setter.WAFResult) {
			result.Batches[0].Result.ReturnedManaged.Items[1].ID = old.ID
			result.Batches[0].Result.ReturnedAllItems = result.Batches[0].Result.ReturnedManaged.Items
		}, wantKind: notifier.KindUpdate, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.Contains(t, content.issues, wafReturnedIdentityInvalid)
			require.NotContains(t, content.issues, wafReturnedContentsUnavailable)
			require.Nil(t, content.observed)
			require.Equal(t, api.WAFBatchCompleted, o.result.Batches[0].Result.Status)
		}},
		{name: "returned-identity-missing", configure: func(result *setter.WAFResult) {
			result.Batches[0].Result.ReturnedManaged.Items[1].ID = ""
			result.Batches[0].Result.ReturnedAllItems = result.Batches[0].Result.ReturnedManaged.Items
		}, wantKind: notifier.KindUpdate, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.Contains(t, content.issues, wafReturnedIdentityInvalid)
			require.NotContains(t, content.issues, wafReturnedContentsUnavailable)
			require.Nil(t, content.observed)
			require.Equal(t, api.WAFBatchCompleted, o.result.Batches[0].Result.Status)
		}},
		{name: "returned-identity-conflicting", configure: func(result *setter.WAFResult) {
			result.Batches[0].Result.ReturnedAllItems[1].Comment = "conflicting-view"
		}, wantKind: notifier.KindUpdate, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.Contains(t, content.issues, wafReturnedIdentityInvalid)
			require.NotContains(t, content.issues, wafReturnedContentsUnavailable)
			require.Nil(t, content.observed)
			require.Equal(t, api.WAFBatchCompleted, o.result.Batches[0].Result.Status)
		}},
		{name: "duplicate-prefix-addition", configure: func(result *setter.WAFResult) {
			result.Before.Items = nil
			duplicate := added
			duplicate.ID = "duplicate"
			result.Batches[0].Result.ReturnedManaged.Items = []api.WAFListItem{added, duplicate}
			result.Batches[0].Result.ReturnedAllItems = result.Batches[0].Result.ReturnedManaged.Items
		}, wantKind: notifier.KindUpdate, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.True(t, content.deltaKnown)
			require.Len(t, content.observed.Items, 2)
			require.Len(t, content.added, 1)
		}},
		{name: "duplicate-prefix-cleanup", configure: func(result *setter.WAFResult) {
			duplicate := old
			duplicate.ID = "duplicate"
			result.Before.Items = append(result.Before.Items, duplicate)

			status, cleanupCode := api.WAFBatchCompleted, api.WAFListCleanupUpdated

			result.Batches = nil
			result.Cleanup = &api.WAFCleanupResult{Code: cleanupCode, Before: result.Before, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{old}, Result: api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: status, ReturnedManaged: nil, ReturnedAllItems: nil}}}}
		}, wantKind: notifier.KindCleanup, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.True(t, content.deltaKnown)
			require.Empty(t, content.removed)
			require.Len(t, content.unchanged, 1)
		}},
		{name: "cleanup-untrusted-identity", configure: func(result *setter.WAFResult) {
			duplicate := old
			duplicate.ID = "duplicate"
			result.Before.Items = append(result.Before.Items, duplicate)

			result.Before.Items[1].ID = old.ID
			result.Before.Items[1].Prefix = added.Prefix
			result.Code = setter.ResponseUpdating
			status, cleanupCode := api.WAFBatchAccepted, api.WAFListCleanupUpdating

			result.Batches = nil
			result.Cleanup = &api.WAFCleanupResult{Code: cleanupCode, Before: result.Before, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{old}, Result: api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: status, ReturnedManaged: nil, ReturnedAllItems: nil}}}}
		}, wantKind: notifier.KindCleanup, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.False(t, o.acceptedFullCleanup(o.configuredSnapshot(content.before)))
			require.False(t, content.deltaKnown)
		}},
		{name: "absent-baseline-response", configure: func(result *setter.WAFResult) {
			result.Before = nil
		}, wantKind: notifier.KindUpdate, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.Nil(t, content.before)
			require.NotNil(t, content.observed)
			require.False(t, content.deltaKnown)
		}},
		{name: "prefix-only-later-unconfirmed", configure: func(result *setter.WAFResult) {
			result.Before.Items = nil
			result.Batches[0].Result.ReturnedManaged.Items = nil
			outside := added
			outside.Comment = "outside-selector"
			result.Batches[0].Result.ReturnedAllItems = []api.WAFListItem{outside}
			result.Before.Items = []api.WAFListItem{old}
			result.Code = setter.ResponseFailed
			result.Batches[1] = setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{old}, Result: &api.WAFBatchResult{ListCreationDispatched: false, OK: false, Status: api.WAFBatchUnconfirmed, ReturnedManaged: nil, ReturnedAllItems: nil}}
		}, wantKind: notifier.KindUpdateFailure, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.Empty(t, content.exclusions)
			require.Nil(t, content.observed)
		}},
		{name: "cleanup-started-partial-target", configure: func(result *setter.WAFResult) {
			duplicate := old
			duplicate.ID = "duplicate"
			result.Before.Items = append(result.Before.Items, duplicate)
			result.Before.Items[1] = added

			result.Code = setter.ResponseUpdating
			status, cleanupCode := api.WAFBatchAccepted, api.WAFListCleanupUpdating

			result.Batches = nil
			result.Cleanup = &api.WAFCleanupResult{Code: cleanupCode, Before: result.Before, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{old}, Result: api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: status, ReturnedManaged: nil, ReturnedAllItems: nil}}}}
		}, wantKind: notifier.KindCleanup, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.False(t, o.acceptedFullCleanup(o.configuredSnapshot(content.before)))
			require.Nil(t, content.observed)
		}},
		{name: "cleanup-completed-partial-target", configure: func(result *setter.WAFResult) {
			duplicate := old
			duplicate.ID = "duplicate"
			result.Before.Items = append(result.Before.Items, duplicate)
			result.Before.Items[1] = added

			status, cleanupCode := api.WAFBatchCompleted, api.WAFListCleanupUpdated

			result.Batches = nil
			result.Cleanup = &api.WAFCleanupResult{Code: cleanupCode, Before: result.Before, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{old}, Result: api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: status, ReturnedManaged: nil, ReturnedAllItems: nil}}}}
		}, wantKind: notifier.KindCleanup, check: func(t *testing.T, message Message) {
			t.Helper()
			o := message.waf[0]
			content := o.content()
			require.True(t, content.deltaKnown)
			require.Len(t, content.removed, 1)
			require.Len(t, content.unchanged, 1)
		}},
	} {
		before := &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old}}
		returned := &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old, added}}
		result := setter.WAFResult{Code: setter.ResponseUpdated, Before: before, Cleanup: nil, Batches: []setter.WAFBatch{
			{Create: []api.WAFListCreateItem{{Prefix: added.Prefix, Comment: added.Comment}}, Delete: nil, Result: &api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: api.WAFBatchCompleted, ReturnedManaged: returned, ReturnedAllItems: []api.WAFListItem{old, added}}},
			{Create: nil, Delete: nil, Result: &api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: api.WAFBatchNoEffect, ReturnedManaged: nil, ReturnedAllItems: nil}},
		}}
		tc.configure(&result)
		o := wafOutcome{filtered: true, scope: wafScope{ipnet.IP4: true}, list: api.WAFList{AccountID: "account", Name: "list"}, cleanup: result.Cleanup != nil, result: result}
		message := newMessage()
		message.cleanup = o.cleanup
		message.waf = []wafOutcome{o}
		message = message.withHeartbeat()
		scenarios = append(scenarios, renderingScenario{name: "defensive-waf-" + tc.name, message: message, wantKind: tc.wantKind, captureHeartbeat: false, assertMessage: tc.check})
	}
	return scenarios
}

// Capture assertions target semantic evidence and cardinality, leaving the full
// human-facing strings available for exact-output review.
func TestNotificationDefensiveWAFCaptureContracts(t *testing.T) {
	t.Parallel()
	for _, scenario := range defensiveWAFRenderingScenarios() {
		scenario.assertMessage(t, scenario.message)
	}
}
