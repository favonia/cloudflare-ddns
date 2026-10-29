package main

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func notificationWAFCleanup(code setter.ResponseCode, whole bool) setter.WAFResult {
	status := api.WAFBatchUnconfirmed
	cleanupCode := api.WAFListCleanupFailed
	switch code {
	case setter.ResponseUpdated:
		status, cleanupCode = api.WAFBatchCompleted, api.WAFListCleanupUpdated
	case setter.ResponseUpdating:
		status, cleanupCode = api.WAFBatchAccepted, api.WAFListCleanupUpdating
	case setter.ResponseNoop:
		cleanupCode = api.WAFListCleanupNoop
	case setter.ResponseFailed:
	}
	result := setter.WAFResult{Code: code, Before: nil, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: cleanupCode, Before: nil, Stages: nil}}
	if code == setter.ResponseNoop {
		result.Cleanup.Before = &api.WAFSnapshot{Exists: false, Cached: false, Items: nil}
	} else {
		var items []api.WAFListItem
		if !whole {
			items = []api.WAFListItem{notificationWAFItem("cleanup", "192.0.2.1/32")}
			result.Cleanup.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: items}
		}
		result.Cleanup.Stages = []api.WAFCleanupStage{{WholeList: whole, Items: items, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: code != setter.ResponseFailed, Status: status, Returned: nil, ReturnedItems: nil}}}
	}
	result.Before = result.Cleanup.Before
	return result
}

// Each content witness uses distinct records and a coherent static provider.
// The shape grid varies retained, removed and added sets independently.
func notificationContentScenarios() []notificationScenario {
	scenarios := make([]notificationScenario, 0, 26)
	for _, cached := range []bool{false, true} {
		for _, shape := range [][3]int{{0, 0, 0}, {0, 0, 1}, {0, 0, 2}, {0, 0, 3}, {0, 1, 0}, {0, 2, 0}, {0, 3, 0}, {0, 1, 1}, {1, 1, 1}, {2, 1, 1}, {3, 2, 3}, {1, 3, 2}, {3, 0, 0}} {
			old := make([]string, 0, shape[0]+shape[1])
			targets := make([]string, 0, shape[0]+shape[2])
			satisfied := make([]netip.Addr, 0, shape[0])
			removed := make([]string, 0, shape[1])
			added := make([]string, 0, shape[2])
			for i := range shape[0] {
				value := fmt.Sprintf("192.0.2.%d", 10+i)
				old = append(old, value)
				targets = append(targets, value)
				satisfied = append(satisfied, netip.MustParseAddr(value))
			}
			for i := range shape[1] {
				value := fmt.Sprintf("192.0.2.%d", 30+i)
				old = append(old, value)
				removed = append(removed, value)
			}
			for i := range shape[2] {
				value := fmt.Sprintf("192.0.2.%d", 50+i)
				targets = append(targets, value)
				added = append(added, value)
			}
			before := notificationSnapshot(old...)
			before.Cached = cached
			var operations []setter.DNSOperation
			for i := range min(len(removed), len(added)) {
				operations = append(operations, notificationOperation(api.RecordUpdate, setter.DNSOperationCompleted, api.ID("record-"+removed[i]), added[i], api.MembershipManaged))
			}
			for i := min(len(removed), len(added)); i < len(added); i++ {
				operations = append(operations, notificationOperation(api.RecordCreate, setter.DNSOperationCompleted, api.ID("new-"+added[i]), added[i], api.MembershipManaged))
			}
			for i := min(len(removed), len(added)); i < len(removed); i++ {
				operations = append(operations, notificationOperation(api.RecordDelete, setter.DNSOperationCompleted, api.ID("record-"+removed[i]), removed[i], api.MembershipUnknown))
			}
			code := setter.ResponseUpdated
			if len(operations) == 0 {
				code = setter.ResponseNoop
			}
			raw := strings.Join(targets, ",")
			if raw == "" {
				raw = "empty"
			}
			//nolint:exhaustruct_v5 // Scenario options default to ordinary scheduled execution.
			scenarios = append(scenarios, notificationScenario{name: fmt.Sprintf("content-r%d-d%d-a%d-cached-%t", shape[0], shape[1], shape[2], cached), targets: raw, rounds: []notificationRound{{results: []setter.DNSResult{notificationProgressResult(code, before, satisfied, operations...)}}}, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}})
		}
	}
	return scenarios
}

func notificationWAFItem(id, prefix string) api.WAFListItem {
	return api.WAFListItem{ID: api.ID(id), Prefix: netip.MustParsePrefix(prefix), Comment: "owned"}
}

func notificationWAFReturned(items ...api.WAFListItem) *api.WAFBatchResult {
	return &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: items}, ReturnedItems: items}
}

func notificationCurrentScenarios() []notificationScenario {
	scenarios := notificationContentScenarios()
	old := notificationSnapshot("192.0.2.1", "192.0.2.2", "192.0.2.3")
	for _, status := range []setter.DNSOperationStatus{setter.DNSOperationUnconfirmed, setter.DNSOperationBlocked} {
		operations := []setter.DNSOperation{
			notificationOperation(api.RecordUpdate, setter.DNSOperationCompleted, old.Records[0].ID, "192.0.2.4", api.MembershipManaged),
			notificationOperation(api.RecordUpdate, status, old.Records[1].ID, "192.0.2.5", api.MembershipUnknown),
			notificationOperation(api.RecordUpdate, setter.DNSOperationNotAttempted, old.Records[2].ID, "192.0.2.6", api.MembershipUnknown),
		}
		//nolint:exhaustruct_v5 // Only the varying evidence and expected lifecycle are relevant.
		scenarios = append(scenarios, notificationScenario{name: fmt.Sprintf("partial-update-pairs-%d", status), targets: "192.0.2.4,192.0.2.5,192.0.2.6", rounds: []notificationRound{{results: []setter.DNSResult{notificationProgressResult(setter.ResponseFailed, old, nil, operations...)}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}})
		//nolint:exhaustruct_v5 // Known-empty chronology uses the ordinary scheduled lifecycle.
		scenarios = append(scenarios, notificationScenario{name: fmt.Sprintf("known-empty-create-%d", status), rounds: []notificationRound{{results: []setter.DNSResult{notificationProgressResult(setter.ResponseFailed, notificationSnapshot(), nil, notificationOperation(api.RecordCreate, status, "", "192.0.2.1", api.MembershipUnknown))}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}})
	}
	//nolint:exhaustruct_v5 // Unknown input and explicit empty target are the only variations.
	scenarios = append(scenarios, notificationScenario{name: "unknown-empty-cleanup-blocked", targets: "empty", rounds: []notificationRound{{results: []setter.DNSResult{{Code: setter.ResponseFailed, Before: nil, Progress: nil, Effects: nil}}}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}})
	scenarios = append(scenarios, notificationWAFEmptyFallbackScenarios()...)
	return append(append(append(scenarios, notificationWAFScenarios()...), notificationWAFCleanupScenarios()...), notificationWAFWordingScenarios()...)
}

func notificationWAFCleanupScenarios() []notificationScenario {
	item := notificationWAFItem("cleanup", "192.0.2.1/32")
	scenarios := make([]notificationScenario, 0, 3)
	for _, status := range []api.WAFBatchStatus{api.WAFBatchCompleted, api.WAFBatchAccepted, api.WAFBatchUnconfirmed} {
		code := setter.ResponseUpdated
		cleanupCode := api.WAFListCleanupUpdated
		kind := notifier.KindShutdown
		exit := 0
		if status == api.WAFBatchAccepted {
			code = setter.ResponseUpdating
			cleanupCode = api.WAFListCleanupUpdating
		}
		if status == api.WAFBatchUnconfirmed {
			code = setter.ResponseFailed
			cleanupCode = api.WAFListCleanupFailed
			kind = notifier.KindCleanupFailure
			exit = 1
		}
		result := setter.WAFResult{Code: code, Before: nil, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: cleanupCode, Before: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item}}, Stages: []api.WAFCleanupStage{
			{WholeList: true, Items: nil, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchUnconfirmed, Returned: nil, ReturnedItems: nil}},
			{WholeList: false, Items: []api.WAFListItem{item}, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: status != api.WAFBatchUnconfirmed, Status: status, Returned: nil, ReturnedItems: nil}},
		}}}
		result.Before = result.Cleanup.Before
		// Whole-list eligibility models the configured handle's ownership policy.
		// Confirmed item fallback uses the one-shot wait path; scheduled cleanup
		// uses async acceptance or an unconfirmed attempt.
		once := status == api.WAFBatchCompleted
		kinds := []notifier.Kind{notifier.KindStartup, kind}
		if once {
			kinds = []notifier.Kind{notifier.KindCleanup}
		}
		//nolint:exhaustruct_v5 // Only ownership, cleanup evidence and lifecycle vary.
		scenarios = append(scenarios, notificationScenario{name: fmt.Sprintf("waf-cleanup-fallback-%d", status), wholeWAF: true, families: []ipnet.Family{ipnet.IP4, ipnet.IP6}, once: once, deferred: !once, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{notificationResult(setter.ResponseUpdated, notificationSnapshot("192.0.2.1"), api.RecordDelete, api.MutationCompleted, "192.0.2.1"), notificationNoop(notificationSnapshot())}, wafCleanup: code, wafCleanupResult: &result, wantKinds: kinds, wantExit: exit})
	}
	return scenarios
}

// The real API retains the failed whole-list attempt when a fresh fallback
// observes an empty list, so no item deletion is needed.
func notificationWAFEmptyFallback() setter.WAFResult {
	before := &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}
	return setter.WAFResult{Code: setter.ResponseNoop, Before: before, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: api.WAFListCleanupNoop, Before: before, Stages: []api.WAFCleanupStage{{WholeList: true, Items: nil, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchUnconfirmed, Returned: nil, ReturnedItems: nil}}}}}
}

func notificationWAFEmptyFallbackScenarios() []notificationScenario {
	scenarios := make([]notificationScenario, 0, 3)
	for _, name := range []string{"once", "shutdown", "scheduling-failure"} {
		result := notificationWAFEmptyFallback()
		once, schedulingFailure := name == "once", name == "scheduling-failure"
		kinds := []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}
		exit := 0
		if once {
			kinds = []notifier.Kind{notifier.KindCleanup}
		}
		if schedulingFailure {
			kinds[1], exit = notifier.KindSchedulingFailure, 1
		}
		//nolint:exhaustruct_v5 // Only cleanup, ownership and lifecycle options vary.
		scenarios = append(scenarios, notificationScenario{name: "waf-cleanup-empty-fallback-" + name, wholeWAF: true, families: []ipnet.Family{ipnet.IP4, ipnet.IP6}, once: once, deferred: !once, schedulingFailure: schedulingFailure, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{notificationNoop(notificationSnapshot()), notificationNoop(notificationSnapshot())}, wafCleanup: setter.ResponseNoop, wafCleanupResult: &result, wantKinds: kinds, wantExit: exit})
	}
	return scenarios
}

func TestNotificationCleanupGuardEmptyFallback(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"fresh-empty", "cached", "missing", "nonempty", "absent-list", "completed", "accepted", "unattempted", "item-stage"} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			result := notificationWAFEmptyFallback()
			switch variant {
			case "cached":
				result.Cleanup.Before.Cached = true
			case "missing":
				result.Cleanup.Before = nil
			case "nonempty":
				result.Cleanup.Before.Items = []api.WAFListItem{notificationWAFItem("old", "192.0.2.1/32")}
			case "absent-list":
				result.Cleanup.Before.Exists = false
			case "completed":
				result.Cleanup.Stages[0].Result.Status, result.Cleanup.Stages[0].Result.OK = api.WAFBatchCompleted, true
			case "accepted":
				result.Cleanup.Stages[0].Result.Status, result.Cleanup.Stages[0].Result.OK = api.WAFBatchAccepted, true
			case "unattempted":
				result.Cleanup.Stages[0].Result.Status = api.WAFBatchNoEffect
			case "item-stage":
				result.Cleanup.Stages[0].WholeList = false
			}
			problem := notificationCleanupProblem(result, map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}, true, api.CleanupWait)
			if variant == "fresh-empty" {
				require.Empty(t, problem)
			} else {
				require.NotEmpty(t, problem)
			}
		})
	}
}

func notificationWAFScenarios() []notificationScenario {
	old := notificationWAFItem("old", "198.51.100.0/24")
	added := notificationWAFItem("added", "192.0.2.1/32")
	cover := notificationWAFItem("cover", "192.0.2.0/24")
	six := notificationWAFItem("six", "2001:db8::/32")
	before := &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old}}
	create := setter.WAFBatch{Create: []api.WAFListCreateItem{{Prefix: added.Prefix, Comment: added.Comment}}, Delete: nil, Result: notificationWAFReturned(old, added)}
	remove := setter.WAFBatch{Create: nil, Delete: []api.WAFListItem{old}, Result: notificationWAFReturned(added)}
	var scenarios []notificationScenario
	add := func(name string, result setter.WAFResult) {
		kind := notifier.KindStartup
		if result.Code == setter.ResponseFailed {
			kind = notifier.KindUpdateFailure
		}
		//nolint:exhaustruct_v5 // WAF evidence shares the configured IPv4 provider with a DNS no-op.
		scenarios = append(scenarios, notificationScenario{name: name, waf: true, rounds: []notificationRound{{results: []setter.DNSResult{notificationNoop(notificationSnapshot("192.0.2.1"), "192.0.2.1")}, waf: &result}}, wantKinds: []notifier.Kind{kind, notifier.KindShutdown}})
	}
	add("waf-covering-prefix", setter.WAFResult{Code: setter.ResponseNoop, Before: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{cover}}, Batches: nil, Cleanup: nil})
	add("waf-shared-provider-replacement", setter.WAFResult{Code: setter.ResponseUpdated, Before: before, Batches: []setter.WAFBatch{create, remove}, Cleanup: nil})
	for _, exists := range []bool{false, true} {
		add(fmt.Sprintf("waf-empty-input-exists-%t", exists), setter.WAFResult{Code: setter.ResponseUpdated, Before: &api.WAFSnapshot{Exists: exists, Cached: false, Items: nil}, Batches: []setter.WAFBatch{{Create: create.Create, Delete: nil, Result: notificationWAFReturned(added)}}, Cleanup: nil})
	}
	for _, status := range []api.WAFBatchStatus{api.WAFBatchNoEffect, api.WAFBatchUnconfirmed, api.WAFBatchCompleted} {
		failed := create
		failed.Result = &api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: status, Returned: nil, ReturnedItems: nil}
		unvisited := remove
		unvisited.Result = nil
		add(fmt.Sprintf("waf-create-failure-%d", status), setter.WAFResult{Code: setter.ResponseFailed, Before: before, Batches: []setter.WAFBatch{failed, unvisited}, Cleanup: nil})
	}
	failedDelete := remove
	failedDelete.Result = &api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchUnconfirmed, Returned: nil, ReturnedItems: nil}
	add("waf-delete-batch-unconfirmed", setter.WAFResult{Code: setter.ResponseFailed, Before: before, Batches: []setter.WAFBatch{create, failedDelete}, Cleanup: nil})
	add("waf-unknown-input", setter.WAFResult{Code: setter.ResponseFailed, Before: nil, Batches: nil, Cleanup: nil})
	add("waf-unavailable-family-current", setter.WAFResult{Code: setter.ResponseNoop, Before: &api.WAFSnapshot{Exists: true, Cached: true, Items: []api.WAFListItem{cover, six}}, Batches: nil, Cleanup: nil})
	scenarios[len(scenarios)-1].families = []ipnet.Family{ipnet.IP4, ipnet.IP6}
	scenarios[len(scenarios)-1].rounds[0].unavailable = map[ipnet.Family]bool{ipnet.IP6: true}
	scenarios[len(scenarios)-1].wantKinds[0] = notifier.KindUpdateFailure
	activeCreate := create
	activeCreate.Result = notificationWAFReturned(old, six, added)
	activeDelete := remove
	activeDelete.Result = notificationWAFReturned(six, added)
	add("waf-unavailable-family-delta", setter.WAFResult{Code: setter.ResponseUpdated, Before: &api.WAFSnapshot{Exists: true, Cached: true, Items: []api.WAFListItem{old, six}}, Batches: []setter.WAFBatch{activeCreate, activeDelete}, Cleanup: nil})
	scenarios[len(scenarios)-1].families = []ipnet.Family{ipnet.IP4, ipnet.IP6}
	scenarios[len(scenarios)-1].rounds[0].unavailable = map[ipnet.Family]bool{ipnet.IP6: true}
	scenarios[len(scenarios)-1].wantKinds[0] = notifier.KindUpdateFailure
	add("waf-all-families-unavailable", setter.WAFResult{Code: setter.ResponseFailed, Before: nil, Batches: nil, Cleanup: nil})
	scenarios[len(scenarios)-1].rounds[0].results = nil
	scenarios[len(scenarios)-1].rounds[0].unavailable = map[ipnet.Family]bool{ipnet.IP4: true}
	// Controlled membership/metadata response witnesses, not live API observations.
	outside := added
	outside.Comment = "outside-selector"
	anomalousDelete := remove
	anomalousDelete.Result = notificationWAFReturned()
	anomalousDelete.Result.ReturnedItems = []api.WAFListItem{outside}
	add("waf-created-item-leaves-ownership", setter.WAFResult{Code: setter.ResponseUpdated, Before: before, Batches: []setter.WAFBatch{create, anomalousDelete}, Cleanup: nil})
	scenarios[len(scenarios)-1].filtered = true
	retained := cover
	retained.ID = "changed-id"
	retained.Comment = "owned-new"
	add("waf-retained-prefix-metadata-change", setter.WAFResult{Code: setter.ResponseUpdated, Before: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{cover}}, Batches: []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: netip.MustParsePrefix("192.0.2.200/32"), Comment: "owned"}}, Delete: nil, Result: notificationWAFReturned(retained, notificationWAFItem("new", "192.0.2.200/32"))}}, Cleanup: nil})
	// A covering prefix would satisfy .200, so use a disjoint target for this witness.
	last := &scenarios[len(scenarios)-1]
	last.targets = "192.0.2.1,203.0.113.1"
	last.rounds[0].results = []setter.DNSResult{notificationProgressResult(setter.ResponseUpdated, notificationSnapshot("192.0.2.1"), []netip.Addr{netip.MustParseAddr("192.0.2.1")}, notificationOperation(api.RecordCreate, setter.DNSOperationCompleted, "new", "203.0.113.1", api.MembershipManaged))}
	last.rounds[0].waf.Batches[0].Create[0].Prefix = netip.MustParsePrefix("203.0.113.1/32")
	last.rounds[0].waf.Batches[0].Result = notificationWAFReturned(retained, notificationWAFItem("new", "203.0.113.1/32"))
	last.filtered = true
	return scenarios
}

// These narrow guards reject local producer/configuration contradictions; they
// do not attempt to establish remote admissibility or replace setter tests.
func notificationDNSOwnershipProblem(result setter.DNSResult, filtered bool) string {
	if result.Progress != nil {
		for _, op := range result.Progress.Operations {
			if op.Membership == api.MembershipUnmanaged {
				if !filtered {
					return "unmanaged membership requires selective DNS ownership"
				}
				if op.Status != setter.DNSOperationCompleted || op.Kind == api.RecordDelete {
					return "unmanaged membership requires a completed create or update"
				}
			}
		}
	}
	return ""
}

func notificationCleanupProblem(result setter.WAFResult, families map[ipnet.Family]bool, wholeEligible bool, mode api.CleanupMode) string {
	c := result.Cleanup
	if c == nil {
		return "cleanup result lacks stage evidence"
	}
	if len(c.Stages) > 2 {
		return "cleanup has too many stages"
	}
	projection := map[api.WAFListCleanupCode]setter.ResponseCode{api.WAFListCleanupFailed: setter.ResponseFailed, api.WAFListCleanupNoop: setter.ResponseNoop, api.WAFListCleanupUpdated: setter.ResponseUpdated, api.WAFListCleanupUpdating: setter.ResponseUpdating}
	if projected, ok := projection[c.Code]; !ok || projected != result.Code {
		return "cleanup work-result projection disagrees"
	}
	if len(c.Stages) == 0 && result.Code != setter.ResponseNoop && result.Code != setter.ResponseFailed {
		return "completed cleanup lacks a reached stage"
	}
	for i, stage := range c.Stages {
		if stage.Result.OK != (stage.Result.Status == api.WAFBatchCompleted || stage.Result.Status == api.WAFBatchAccepted) {
			return "cleanup stage status and success disagree"
		}
		if i == len(c.Stages)-1 {
			expected := setter.ResponseFailed
			if stage.Result.Status == api.WAFBatchCompleted {
				expected = setter.ResponseUpdated
			}
			if stage.Result.Status == api.WAFBatchAccepted {
				expected = setter.ResponseUpdating
			}
			// Empty fallback needs no item request; the retained whole-list failure
			// still supplies operation evidence without making the work fail.
			if len(c.Stages) == 1 && stage.WholeList && stage.Result.Status == api.WAFBatchUnconfirmed &&
				c.Before != nil && c.Before.Exists && !c.Before.Cached && len(c.Before.Items) == 0 {
				expected = setter.ResponseNoop
			}
			if result.Code != expected {
				return "last cleanup stage and work result disagree"
			}
		}
		if stage.WholeList {
			if i != 0 || !wholeEligible || !families[ipnet.IP4] || !families[ipnet.IP6] {
				return "whole-list stage is outside eligible ownership"
			}
			if stage.Result.Status != api.WAFBatchCompleted && stage.Result.Status != api.WAFBatchUnconfirmed {
				return "whole-list stage has impossible status"
			}
			if stage.Result.Status == api.WAFBatchCompleted && (len(c.Stages) != 1 || c.Before != nil) {
				return "completed whole-list stage cannot have fallback input"
			}
			if len(stage.Items) != 0 {
				return "whole-list stage invents item input"
			}
			continue
		}
		if c.Before == nil || !c.Before.Exists || len(stage.Items) == 0 {
			return "item stage lacks admitted items"
		}
		if i > 0 && (!c.Stages[0].WholeList || c.Before.Cached) {
			return "fallback must follow whole-list failure with fresh input"
		}
		for _, item := range stage.Items {
			family := ipnet.IP6
			if item.Prefix.Addr().Is4() {
				family = ipnet.IP4
			}
			if !families[family] || !slices.Contains(c.Before.Items, item) {
				return "cleanup item is outside admitted managed families"
			}
		}
		switch stage.Result.Status {
		case api.WAFBatchCompleted:
			if mode != api.CleanupWait {
				return "completed item cleanup requires wait"
			}
		case api.WAFBatchAccepted:
			if mode != api.CleanupAllowAsync {
				return "accepted item cleanup requires async mode"
			}
		case api.WAFBatchUnconfirmed:
		case api.WAFBatchNoEffect:
			return "item cleanup never records an unattempted stage"
		}
	}
	if c.Before != nil && (result.Before == nil || c.Before.Exists != result.Before.Exists || c.Before.Cached != result.Before.Cached || !slices.Equal(c.Before.Items, result.Before.Items)) {
		return "setter cleanup input projection disagrees"
	}
	return ""
}

func TestNotificationScenarioOwnershipAndCleanupContracts(t *testing.T) {
	t.Parallel()
	for _, scenario := range notificationScenarios() {
		for _, round := range scenario.rounds {
			for _, result := range round.results {
				require.Empty(t, notificationDNSOwnershipProblem(result, scenario.filtered || scenario.dnsFiltered), scenario.name)
			}
		}
		if !scenario.cleanup {
			continue
		}
		families := map[ipnet.Family]bool{ipnet.IP4: true}
		if len(scenario.families) > 0 {
			families = make(map[ipnet.Family]bool)
			for _, family := range scenario.families {
				families[family] = true
			}
		}
		mode := api.CleanupAllowAsync
		if scenario.once {
			mode = api.CleanupWait
		}
		result := scenario.wafCleanupResult
		if result == nil {
			value := notificationWAFCleanup(scenario.wafCleanup, scenario.wholeWAF)
			result = &value
		}
		require.Empty(t, notificationCleanupProblem(*result, families, scenario.wholeWAF && !scenario.filtered && !scenario.wafFiltered, mode), scenario.name)
	}
}

func TestNotificationCleanupGuardRejectsContradictions(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"partial-whole-list", "accepted-whole-list", "unattempted-whole-list", "completed-item-async", "accepted-item-wait", "item-outside-family", "missing-item-input", "wrong-result-projection"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			families := map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
			mode := api.CleanupWait
			whole := strings.Contains(name, "whole-list")
			result := notificationWAFCleanup(setter.ResponseUpdated, whole)
			switch name {
			case "partial-whole-list":
				delete(families, ipnet.IP6)
			case "accepted-whole-list":
				result = notificationWAFCleanup(setter.ResponseUpdating, true)
				mode = api.CleanupAllowAsync
			case "unattempted-whole-list":
				result = notificationWAFCleanup(setter.ResponseFailed, true)
				result.Cleanup.Stages[0].Result.Status = api.WAFBatchNoEffect
			case "completed-item-async":
				mode = api.CleanupAllowAsync
			case "accepted-item-wait":
				result = notificationWAFCleanup(setter.ResponseUpdating, false)
			case "item-outside-family":
				delete(families, ipnet.IP4)
			case "missing-item-input":
				result.Before = nil
				result.Cleanup.Before = nil
			case "wrong-result-projection":
				result.Code = setter.ResponseFailed
			}
			require.NotEmpty(t, notificationCleanupProblem(result, families, true, mode))
		})
	}
}

// Controlled integration witnesses follow create-before-delete and stop on the
// first failed work result. Accepted work belongs only to async item cleanup.
func notificationWAFWordingScenarios() []notificationScenario {
	scenarios := make([]notificationScenario, 0, 7)
	for _, tc := range []struct {
		name               string
		count              int
		cached, failCreate bool
		status             api.WAFBatchStatus
	}{
		{"waf-grouped-addition-blocked", 2, false, true, api.WAFBatchNoEffect},
		{"waf-grouped-addition-unconfirmed", 2, false, true, api.WAFBatchUnconfirmed},
		{"waf-grouped-addition-completed-cached", 2, true, true, api.WAFBatchCompleted},
		{"waf-grouped-removal-unconfirmed", 3, false, false, api.WAFBatchUnconfirmed},
		{"waf-grouped-removal-completed-cached", 3, true, false, api.WAFBatchCompleted},
	} {
		old := make([]api.WAFListItem, 0, tc.count)
		added := make([]api.WAFListItem, 0, tc.count)
		creates := make([]api.WAFListCreateItem, 0, tc.count)
		addresses := make([]string, 0, tc.count)
		for i := range tc.count {
			old = append(old, notificationWAFItem(fmt.Sprintf("old-%d", i), fmt.Sprintf("198.51.%d.0/24", 100+i)))
			address := fmt.Sprintf("192.0.2.%d", i+1)
			addresses = append(addresses, address)
			item := notificationWAFItem(fmt.Sprintf("new-%d", i), address+"/32")
			added = append(added, item)
			creates = append(creates, api.WAFListCreateItem{Prefix: item.Prefix, Comment: item.Comment})
		}
		create := setter.WAFBatch{Create: creates, Delete: nil, Result: notificationWAFReturned(slices.Concat(old, added)...)}
		remove := setter.WAFBatch{Create: nil, Delete: old, Result: nil}
		failed := &api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: tc.status, Returned: nil, ReturnedItems: nil}
		if tc.failCreate {
			create.Result = failed
		} else {
			remove.Result = failed
		}
		result := setter.WAFResult{Code: setter.ResponseFailed, Before: &api.WAFSnapshot{Exists: true, Cached: tc.cached, Items: old}, Batches: []setter.WAFBatch{create, remove}, Cleanup: nil}
		//nolint:exhaustruct_v5 // Ordinary scheduled execution shares identical DNS and WAF targets.
		scenarios = append(scenarios, notificationScenario{name: tc.name, targets: strings.Join(addresses, ","), waf: true, rounds: []notificationRound{{results: []setter.DNSResult{notificationNoop(notificationSnapshot(addresses...), addresses...)}, waf: &result}}, wantKinds: []notifier.Kind{notifier.KindUpdateFailure, notifier.KindShutdown}})
	}
	for _, tc := range []struct {
		name   string
		count  int
		cached bool
	}{
		{"waf-grouped-cleanup-started", 2, false},
		{"waf-grouped-cleanup-started-cached", 3, true},
	} {
		items := make([]api.WAFListItem, 0, tc.count)
		for i := range tc.count {
			items = append(items, notificationWAFItem(fmt.Sprintf("cleanup-%d", i), fmt.Sprintf("192.0.2.%d/32", i+1)))
		}
		before := &api.WAFSnapshot{Exists: true, Cached: tc.cached, Items: items}
		result := setter.WAFResult{Code: setter.ResponseUpdating, Before: before, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: api.WAFListCleanupUpdating, Before: before, Stages: []api.WAFCleanupStage{{WholeList: false, Items: items, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchAccepted, Returned: nil, ReturnedItems: nil}}}}}
		//nolint:exhaustruct_v5 // Scheduled shutdown permits accepted item cleanup without whole-list ownership.
		scenarios = append(scenarios, notificationScenario{name: tc.name, deferred: true, rounds: []notificationRound{{}}, cleanup: true, cleanupResults: []setter.DNSResult{notificationNoop(notificationSnapshot())}, wafCleanup: setter.ResponseUpdating, wafCleanupResult: &result, wantKinds: []notifier.Kind{notifier.KindStartup, notifier.KindShutdown}})
	}
	return scenarios
}

func TestNotificationDNSOwnershipRejectsUnconfirmedExclusion(t *testing.T) {
	t.Parallel()
	for _, status := range []setter.DNSOperationStatus{setter.DNSOperationUnconfirmed, setter.DNSOperationBlocked, setter.DNSOperationNotAttempted} {
		result := notificationProgressResult(setter.ResponseFailed, notificationSnapshot(), nil, notificationOperation(api.RecordCreate, status, "", "192.0.2.1", api.MembershipUnmanaged))
		require.NotEmpty(t, notificationDNSOwnershipProblem(result, true))
	}
	result := notificationProgressResult(setter.ResponseUpdated, notificationSnapshot("192.0.2.1"), nil, notificationOperation(api.RecordDelete, setter.DNSOperationCompleted, "record", "192.0.2.1", api.MembershipUnmanaged))
	require.NotEmpty(t, notificationDNSOwnershipProblem(result, true))
}
