package main

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// These append-only witnesses exercise real updater/lifecycle composition with
// controlled setter evidence. They neither execute selectors nor claim that a
// remote service admitted the supplied snapshots. Separate flags distinguish
// comment restriction from configured-family projection.
func notificationFilteredScenarios() []notificationScenario {
	scenarios := make([]notificationScenario, 0, 3)
	addDNS := func(name, target string, result setter.DNSResult) {
		kind := notifier.KindStartup
		if result.Code == setter.ResponseFailed {
			kind = notifier.KindUpdateFailure
		}
		//nolint:exhaustruct_v5 // Capture options default to scheduled execution.
		scenarios = append(scenarios, notificationScenario{name: name, dnsFiltered: true, targets: target, rounds: []notificationRound{{results: []setter.DNSResult{result}}}, wantKinds: []notifier.Kind{kind, notifier.KindShutdown}})
	}
	// Reuse the same evidence builder, independently of baseline display labels.
	for _, tc := range []struct {
		name   string
		shape  [3]int
		cached bool
	}{
		{"filtered-dns-add-one", [3]int{0, 0, 1}, false},
		{"filtered-dns-add-many", [3]int{0, 0, 3}, false},
		{"filtered-dns-remove-all", [3]int{0, 2, 0}, false},
		{"filtered-dns-replace-cached", [3]int{0, 1, 1}, true},
	} {
		scenario := notificationContentScenario(tc.name, tc.shape, tc.cached)
		scenario.dnsFiltered = true
		scenarios = append(scenarios, scenario)
	}
	retained := netip.MustParseAddr("192.0.2.1")
	addDNS("filtered-dns-add-retain", "192.0.2.1,192.0.2.2", notificationProgressResult(setter.ResponseUpdated, notificationSnapshot("192.0.2.1"), []netip.Addr{retained}, notificationOperation(api.RecordCreate, setter.DNSOperationCompleted, "new", "192.0.2.2", api.MembershipManaged)))
	addDNS("filtered-dns-remove-retain", "192.0.2.1", notificationProgressResult(setter.ResponseUpdated, notificationSnapshot("192.0.2.1", "192.0.2.2"), []netip.Addr{retained}, notificationOperation(api.RecordDelete, setter.DNSOperationCompleted, "record-192.0.2.2", "192.0.2.2", api.MembershipUnknown)))
	addDNS("filtered-dns-mixed-many", "192.0.2.1,192.0.2.3,192.0.2.4", notificationProgressResult(setter.ResponseUpdated, notificationSnapshot("192.0.2.1", "192.0.2.2"), []netip.Addr{retained}, notificationOperation(api.RecordUpdate, setter.DNSOperationCompleted, "record-192.0.2.2", "192.0.2.3", api.MembershipManaged), notificationOperation(api.RecordCreate, setter.DNSOperationCompleted, "new", "192.0.2.4", api.MembershipManaged)))
	addDNS("filtered-dns-empty-ipv6-current", "", notificationNoop(notificationSnapshot()))
	last := &scenarios[len(scenarios)-1]
	last.families, last.targets6 = []ipnet.Family{ipnet.IP6}, "empty"
	cached := notificationSnapshot("192.0.2.1", "192.0.2.2")
	cached.Cached = true
	addDNS("filtered-dns-cached-current-many", "192.0.2.1,192.0.2.2", notificationNoop(cached, "192.0.2.1", "192.0.2.2"))
	addDNS("filtered-dns-cached-empty-current", "empty", notificationNoop(&setter.DNSSnapshot{Cached: true, Records: nil}))
	addDNS("filtered-dns-unknown-prewrite-blocked", "", setter.DNSResult{Code: setter.ResponseFailed, Before: nil, Progress: nil, Effects: nil})
	addDNS("filtered-dns-paired-blocked", "192.0.2.2", notificationProgressResult(setter.ResponseFailed, notificationSnapshot("192.0.2.1"), nil, notificationOperation(api.RecordUpdate, setter.DNSOperationBlocked, "record-192.0.2.1", "192.0.2.2", api.MembershipUnknown)))
	addDNS("filtered-dns-partial-unconfirmed", "192.0.2.2,192.0.2.3", notificationProgressResult(setter.ResponseFailed, notificationSnapshot("192.0.2.1"), nil, notificationOperation(api.RecordUpdate, setter.DNSOperationCompleted, "record-192.0.2.1", "192.0.2.2", api.MembershipManaged), notificationOperation(api.RecordCreate, setter.DNSOperationUnconfirmed, "", "192.0.2.3", api.MembershipUnknown)))
	cleanup := notificationFinalDeleteScenario()
	cleanup.name, cleanup.dnsFiltered = "filtered-dns-cleanup-partial", true
	scenarios = append(scenarios, cleanup)
	return append(scenarios, notificationFilteredWAFScenarios()...)
}

func notificationWAFEmptyDelete() setter.WAFBatch {
	return setter.WAFBatch{Create: nil, Delete: nil, Result: &api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: api.WAFBatchNoEffect, ReturnedManaged: nil, ReturnedAllItems: nil}}
}

func notificationWAFCreate(items []api.WAFListItem, result *api.WAFBatchResult) setter.WAFBatch {
	creates := make([]api.WAFListCreateItem, 0, len(items))
	for _, item := range items {
		creates = append(creates, api.WAFListCreateItem{Prefix: item.Prefix, Comment: item.Comment})
	}
	return setter.WAFBatch{Create: creates, Delete: nil, Result: result}
}

func notificationFilteredWAFScenarios() []notificationScenario {
	scenarios := make([]notificationScenario, 0, 3)
	added := notificationWAFItem("added", "192.0.2.1/32")
	old := notificationWAFItem("old", "198.51.100.0/24")
	six := notificationWAFItem("six", "2001:db8::/32")
	cover := notificationWAFItem("cover", "192.0.2.0/24")
	add := func(name string, result setter.WAFResult) {
		kind := notifier.KindStartup
		if result.Code == setter.ResponseFailed {
			kind = notifier.KindUpdateFailure
		}
		//nolint:exhaustruct_v5 // DNS and WAF share a real IPv4 provider with controlled setter outcomes.
		scenarios = append(scenarios, notificationScenario{name: name, waf: true, wafFiltered: true, rounds: []notificationRound{{results: []setter.DNSResult{notificationNoop(notificationSnapshot("192.0.2.1"), "192.0.2.1")}, waf: &result}}, wantKinds: []notifier.Kind{kind, notifier.KindShutdown}})
	}
	before := func(cached bool, items ...api.WAFListItem) *api.WAFSnapshot {
		return &api.WAFSnapshot{Exists: true, Cached: cached, Items: items}
	}
	add("filtered-waf-add-one", setter.WAFResult{Code: setter.ResponseUpdated, Before: before(false), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, notificationWAFReturned(added)), notificationWAFEmptyDelete()}, Cleanup: nil})
	add("filtered-waf-empty-current", setter.WAFResult{Code: setter.ResponseNoop, Before: before(false), Batches: nil, Cleanup: nil})
	scenarios[len(scenarios)-1].targets = "empty"
	scenarios[len(scenarios)-1].rounds[0].results = []setter.DNSResult{notificationNoop(notificationSnapshot())}
	add("filtered-waf-configured-ipv4-projection", setter.WAFResult{Code: setter.ResponseNoop, Before: before(false, cover, six), Batches: nil, Cleanup: nil})
	// Family-only restriction: no comment selector. Other cases combine both.
	scenarios[len(scenarios)-1].wafFiltered = false
	inactive := func() {
		last := &scenarios[len(scenarios)-1]
		last.families = []ipnet.Family{ipnet.IP4, ipnet.IP6}
		last.rounds[0].unavailable = map[ipnet.Family]bool{ipnet.IP6: true}
		last.wantKinds[0] = notifier.KindUpdateFailure
	}
	for _, nonempty := range []bool{true, false} {
		name := "filtered-waf-inactive-ipv6-empty"
		var items []api.WAFListItem
		if nonempty {
			name, items = "filtered-waf-inactive-ipv6-nonempty", []api.WAFListItem{six}
		}
		returned := append(slices.Clone(items), added)
		add(name, setter.WAFResult{Code: setter.ResponseUpdated, Before: before(false, items...), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, notificationWAFReturned(returned...)), notificationWAFEmptyDelete()}, Cleanup: nil})
		inactive()
	}
	unconfirmed := func() *api.WAFBatchResult {
		return &api.WAFBatchResult{ListCreationDispatched: false, OK: false, Status: api.WAFBatchUnconfirmed, ReturnedManaged: nil, ReturnedAllItems: nil}
	}
	for _, nonempty := range []bool{false, true} {
		name := "filtered-waf-inactive-empty-later-unconfirmed"
		items := []api.WAFListItem{old}
		if nonempty {
			name = "filtered-waf-inactive-nonempty-later-unconfirmed"
			items = append(items, six)
		}
		add(name, setter.WAFResult{Code: setter.ResponseFailed, Before: before(false, items...), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, notificationWAFReturned(append(slices.Clone(items), added)...)), {Create: nil, Delete: []api.WAFListItem{old}, Result: unconfirmed()}}, Cleanup: nil})
		inactive()
	}
	add("filtered-waf-inactive-history-unconfirmed", setter.WAFResult{Code: setter.ResponseFailed, Before: before(true, old, six), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, unconfirmed()), {Create: nil, Delete: []api.WAFListItem{old}, Result: nil}}, Cleanup: nil})
	inactive()
	add("filtered-waf-create-then-delete-unconfirmed", setter.WAFResult{Code: setter.ResponseFailed, Before: before(false, old), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, notificationWAFReturned(old, added)), {Create: nil, Delete: []api.WAFListItem{old}, Result: unconfirmed()}}, Cleanup: nil})
	// Completion without a readable response differs from an unconfirmed request.
	unreadable := func() *api.WAFBatchResult {
		return &api.WAFBatchResult{ListCreationDispatched: false, OK: false, Status: api.WAFBatchCompleted, ReturnedManaged: nil, ReturnedAllItems: nil}
	}
	add("filtered-waf-completed-unreadable-response", setter.WAFResult{Code: setter.ResponseFailed, Before: before(false, old), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, unreadable()), {Create: nil, Delete: []api.WAFListItem{old}, Result: nil}}, Cleanup: nil})
	add("filtered-waf-create-then-delete-unreadable", setter.WAFResult{Code: setter.ResponseFailed, Before: before(false, old), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, notificationWAFReturned(old, added)), {Create: nil, Delete: []api.WAFListItem{old}, Result: unreadable()}}, Cleanup: nil})
	outside := cover
	outside.Comment = "outside-selector"
	terminal := notificationWAFReturned(added)
	terminal.ReturnedAllItems = []api.WAFListItem{outside, added}
	add("filtered-waf-existing-item-excluded-terminal-current", setter.WAFResult{Code: setter.ResponseUpdated, Before: before(false, cover), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, terminal), notificationWAFEmptyDelete()}, Cleanup: nil})
	// Keep the old /24 and request a disjoint new address; .1 alone is covered.
	last := &scenarios[len(scenarios)-1]
	last.targets = "192.0.2.1,203.0.113.1"
	last.rounds[0].results = []setter.DNSResult{notificationNoop(notificationSnapshot("192.0.2.1", "203.0.113.1"), "192.0.2.1", "203.0.113.1")}
	newItem := notificationWAFItem("added", "203.0.113.1/32")
	last.rounds[0].waf.Batches[0].Create = []api.WAFListCreateItem{{Prefix: newItem.Prefix, Comment: newItem.Comment}}
	last.rounds[0].waf.Batches[0].Result = notificationWAFReturned(newItem)
	last.rounds[0].waf.Batches[0].Result.ReturnedAllItems = []api.WAFListItem{outside, newItem}
	// A later uncertain mutation invalidates that terminal observation, while the
	// exclusion warning remains attached to the previously owned identity.
	extraOld := notificationWAFItem("remove", "198.51.100.0/24")
	add("filtered-waf-existing-item-excluded-later-unconfirmed", setter.WAFResult{Code: setter.ResponseFailed, Before: before(false, cover, extraOld), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{newItem}, &api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: api.WAFBatchCompleted, ReturnedManaged: before(false, newItem, extraOld), ReturnedAllItems: []api.WAFListItem{outside, newItem, extraOld}}), {Create: nil, Delete: []api.WAFListItem{extraOld}, Result: unconfirmed()}}, Cleanup: nil})
	last = &scenarios[len(scenarios)-1]
	last.targets = "192.0.2.1,203.0.113.1"
	last.rounds[0].results = []setter.DNSResult{notificationNoop(notificationSnapshot("192.0.2.1", "203.0.113.1"), "192.0.2.1", "203.0.113.1")}
	add("filtered-waf-inactive-ipv6-noop-current", setter.WAFResult{Code: setter.ResponseNoop, Before: before(true, cover, six), Batches: nil, Cleanup: nil})
	inactive()
	for _, structural := range []bool{false, true} {
		name := "filtered-waf-prewrite-blocked-cached-current"
		if structural {
			name = "filtered-waf-list-creation-unconfirmed"
		}
		input := before(true, old)
		if structural {
			input = &api.WAFSnapshot{Exists: false, Cached: true, Items: nil}
		}
		batches := []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, &api.WAFBatchResult{ListCreationDispatched: structural, OK: false, Status: api.WAFBatchNoEffect, ReturnedManaged: nil, ReturnedAllItems: nil})}
		if !structural {
			batches = append(batches, setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{old}, Result: nil})
		}
		add(name, setter.WAFResult{Code: setter.ResponseFailed, Before: input, Batches: batches, Cleanup: nil})
	}
	// Comment-only selection configures both families with usable targets.
	add("filtered-waf-comment-only-both-families", setter.WAFResult{Code: setter.ResponseUpdated, Before: before(false, six), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, notificationWAFReturned(added, six)), notificationWAFEmptyDelete()}, Cleanup: nil})
	last = &scenarios[len(scenarios)-1]
	last.families = []ipnet.Family{ipnet.IP4, ipnet.IP6}
	last.rounds[0].results = append(last.rounds[0].results, notificationNoop(notificationSnapshot("2001:db8::1"), "2001:db8::1"))
	// One startup envelope, one caller's coherent shared provider inputs.
	add("filtered-dns-dual-family-waf-startup", setter.WAFResult{Code: setter.ResponseUpdated, Before: before(false, six), Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{added}, notificationWAFReturned(added, six)), notificationWAFEmptyDelete()}, Cleanup: nil})
	last = &scenarios[len(scenarios)-1]
	last.families, last.dnsFiltered = []ipnet.Family{ipnet.IP4, ipnet.IP6}, true
	last.rounds[0].results = []setter.DNSResult{notificationResult(setter.ResponseUpdated, notificationSnapshot(), api.RecordCreate, api.MutationCompleted, "192.0.2.1"), notificationNoop(notificationSnapshot("2001:db8::1"), "2001:db8::1")}
	return append(scenarios, notificationFilteredCleanupScenarios()...)
}

func notificationFilteredCleanupScenarios() []notificationScenario {
	scenarios := make([]notificationScenario, 0, 3)
	for _, tc := range []struct {
		name         string
		cached, once bool
	}{
		{"filtered-waf-cleanup-started-all", false, false},
		{"filtered-waf-cleanup-started-all-cached", true, false},
		{"filtered-waf-cleanup-completed-all", false, true},
	} {
		items := []api.WAFListItem{notificationWAFItem("one", "192.0.2.1/32"), notificationWAFItem("two", "192.0.2.2/32")}
		before := &api.WAFSnapshot{Exists: true, Cached: tc.cached, Items: items}
		once := tc.once
		targets := items
		code, cleanupCode, status := setter.ResponseUpdating, api.WAFListCleanupUpdating, api.WAFBatchAccepted
		kinds := []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}
		if once {
			code, cleanupCode, status = setter.ResponseUpdated, api.WAFListCleanupUpdated, api.WAFBatchCompleted
			kinds = []notifier.Kind{notifier.KindCleanup}
		}
		result := setter.WAFResult{Code: code, Before: before, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: cleanupCode, Before: before, Stages: []api.WAFCleanupStage{{WholeList: false, Items: targets, Result: api.WAFBatchResult{ListCreationDispatched: false, OK: true, Status: status, ReturnedManaged: nil, ReturnedAllItems: nil}}}}}
		//nolint:exhaustruct_v5 // Waited once cleanup and async shutdown share selective item evidence.
		scenarios = append(scenarios, notificationScenario{name: tc.name, wafFiltered: true, deferred: !once, once: once, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{notificationNoop(notificationSnapshot())}, wafCleanup: code, wafCleanupResult: &result, wantKinds: kinds})
	}
	return scenarios
}

// Reject contradictions in the controlled update-round evidence. This guard
// checks the setter's batch obligations, not remote response admissibility or
// completeness of a hand-built plan. Defensive anomalies use the direct harness.
func notificationWAFEvidenceProblem(result setter.WAFResult, targets map[ipnet.Family]setter.WAFTargets) string {
	if result.Cleanup != nil {
		return "update round carries cleanup evidence"
	}
	if result.Before == nil {
		if result.Code != setter.ResponseFailed || len(result.Batches) != 0 {
			return "failed initial read carries a plan"
		}
		return ""
	}
	if result.Code == setter.ResponseNoop && len(result.Batches) != 0 {
		return "no-op carries a mutation plan"
	}
	stopped, deleting := false, false
	for _, batch := range result.Batches {
		if len(batch.Create) > 0 && (deleting || len(batch.Delete) > 0) {
			return "create work follows or shares delete work"
		}
		deleting = deleting || len(batch.Delete) > 0
		for _, create := range batch.Create {
			family := ipnet.IP6
			if create.Prefix.Addr().Is4() {
				family = ipnet.IP4
			}
			target, configured := targets[family]
			if !configured || !target.HasUsableTargets() || !slices.Contains(target.Prefixes, create.Prefix) {
				return "create is outside active targets"
			}
		}
		for _, item := range batch.Delete {
			family := ipnet.IP6
			if item.Prefix.Addr().Is4() {
				family = ipnet.IP4
			}
			target, configured := targets[family]
			if !configured || !target.HasUsableTargets() || !slices.Contains(result.Before.Items, item) {
				return "delete is outside active admitted items"
			}
		}
		if batch.Result == nil {
			stopped = true
			continue
		}
		if stopped {
			return "reached work follows a failed or unvisited batch"
		}
		if batch.Result.Status == api.WAFBatchAccepted {
			return "update-round mutations must wait for completion"
		}
		if batch.Result.OK {
			if batch.Result.Status != api.WAFBatchCompleted && (batch.Result.Status != api.WAFBatchNoEffect || len(batch.Create)+len(batch.Delete) != 0) {
				return "successful batch has an impossible status"
			}
		} else {
			stopped = true
		}
		if batch.Result.ListCreationDispatched && len(batch.Create) == 0 {
			return "structural prerequisite belongs to create work"
		}
	}
	if stopped && result.Code != setter.ResponseFailed {
		return "failed or unvisited plan claims success"
	}
	return ""
}

func TestNotificationWAFGuardRejectsContradictions(t *testing.T) {
	t.Parallel()
	item := notificationWAFItem("old", "192.0.2.1/32")
	for _, tc := range []struct {
		name   string
		mutate func(*setter.WAFResult, map[ipnet.Family]setter.WAFTargets)
	}{
		{name: "inactive-create", mutate: func(_ *setter.WAFResult, targets map[ipnet.Family]setter.WAFTargets) {
			targets[ipnet.IP4] = setter.NewUnavailableWAFTargets()
		}},
		{name: "inactive-delete", mutate: func(result *setter.WAFResult, targets map[ipnet.Family]setter.WAFTargets) {
			result.Batches = []setter.WAFBatch{{Create: nil, Delete: []api.WAFListItem{item}, Result: notificationWAFReturned()}}
			targets[ipnet.IP4] = setter.NewUnavailableWAFTargets()
		}},
		{name: "accepted", mutate: func(result *setter.WAFResult, _ map[ipnet.Family]setter.WAFTargets) {
			result.Batches[0].Result.Status = api.WAFBatchAccepted
		}},
		{name: "write-after-unreadable", mutate: func(result *setter.WAFResult, _ map[ipnet.Family]setter.WAFTargets) {
			result.Batches[0].Result.OK = false
			result.Batches[0].Result.ReturnedManaged = nil
		}},
		{name: "delete-structural", mutate: func(result *setter.WAFResult, _ map[ipnet.Family]setter.WAFTargets) {
			result.Batches = []setter.WAFBatch{{Create: nil, Delete: []api.WAFListItem{item}, Result: notificationWAFReturned()}}
			result.Batches[0].Result.ListCreationDispatched = true
		}},
		{name: "success-unconfirmed", mutate: func(result *setter.WAFResult, _ map[ipnet.Family]setter.WAFTargets) {
			result.Batches[0].Result.Status = api.WAFBatchUnconfirmed
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := setter.WAFResult{Code: setter.ResponseUpdated, Before: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item}}, Cleanup: nil, Batches: []setter.WAFBatch{notificationWAFCreate([]api.WAFListItem{item}, notificationWAFReturned(item)), notificationWAFEmptyDelete()}}
			targets := map[ipnet.Family]setter.WAFTargets{ipnet.IP4: setter.NewAvailableWAFTargets([]netip.Prefix{item.Prefix})}
			tc.mutate(&result, targets)
			require.NotEmpty(t, notificationWAFEvidenceProblem(result, targets))
		})
	}
}
