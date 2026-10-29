package updater

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// Bounded formatter coverage complements the lifecycle witnesses; it does not
// establish producer or remote reachability for arbitrary combinations.
func TestRenderContentCardinalities(t *testing.T) {
	t.Parallel()
	for retained := range 4 {
		for removed := range 4 {
			for added := range 4 {
				for _, cached := range []bool{false, true} {
					sets := make([][]string, 3)
					for kind, count := range []int{added, removed, retained} {
						for i := range count {
							sets[kind] = append(sets[kind], fmt.Sprintf("192.0.%d.%d", kind, i+1))
						}
					}
					text := strings.Join(renderDelta(sets[0], sets[1], sets[2], cached), "\n")
					for _, set := range sets {
						for _, value := range set {
							require.Equal(t, 1, strings.Count(text, value))
						}
					}
					if added == 1 && removed == 1 {
						require.Contains(t, text, "Updated:")
						require.NotContains(t, text, "Added:")
						require.Contains(t, text, " → ")
					} else {
						require.NotContains(t, text, " → ")
					}
					if retained > 0 {
						require.Contains(t, text, "Unchanged")
					}
					require.NotContains(t, text, "[")
					require.NotContains(t, text, "{")
					if !cached {
						require.NotContains(t, text, "cached")
					}
				}
			}
		}
	}
}

func TestRenderWAFMissingInputRemainsDistinct(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{ID: "new", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: ""}
	var o wafOutcome
	o.configured = map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
	o.code = setter.ResponseUpdated
	o.active = map[ipnet.Family]bool{ipnet.IP4: true}
	o.configured = map[ipnet.Family]bool{ipnet.IP4: true}
	o.result.Before = &api.WAFSnapshot{Exists: false, Cached: false, Items: nil}
	o.result.Batches = []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: item.Prefix, Comment: ""}}, Delete: nil, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item}}, ReturnedItems: []api.WAFListItem{item}}}}
	require.Contains(t, renderWAF(o), "did not exist before the update")
	o.result.Before.Exists = true
	require.NotContains(t, renderWAF(o), "list does not exist")
}

func TestRenderWAFCleanupAndMembership(t *testing.T) {
	t.Parallel()
	for _, status := range []api.WAFBatchStatus{api.WAFBatchCompleted, api.WAFBatchAccepted} {
		var o wafOutcome
		o.configured = map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
		o.cleanup = true
		o.code = setter.ResponseUpdated
		o.result.Cleanup = &api.WAFCleanupResult{Code: api.WAFListCleanupUpdated, Before: nil, Stages: []api.WAFCleanupStage{{WholeList: true, Items: nil, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: status, Returned: nil, ReturnedItems: nil}}}}
		if status == api.WAFBatchAccepted {
			item := api.WAFListItem{ID: "item", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: ""}
			o.code = setter.ResponseUpdating
			o.result.Cleanup.Code = api.WAFListCleanupUpdating
			o.result.Cleanup.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item}}
			o.result.Cleanup.Stages = []api.WAFCleanupStage{
				{WholeList: true, Items: nil, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchUnconfirmed, Returned: nil, ReturnedItems: nil}},
				{WholeList: false, Items: []api.WAFListItem{item}, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchAccepted, Returned: nil, ReturnedItems: nil}},
			}
		}
		text := renderWAF(o)
		require.NotContains(t, text, "Current")
		require.NotContains(t, text, "Removed")
		if status == api.WAFBatchCompleted {
			require.Contains(t, text, "Cleanup completed")
		} else {
			require.Contains(t, text, "Cleanup started")
			require.Contains(t, text, "List deletion unconfirmed")
			require.Contains(t, text, "managed item")
			require.Contains(t, text, "192.0.2.1/32")
		}
	}
	old := api.WAFListItem{ID: "old", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "owned"}
	var o wafOutcome
	o.configured = map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
	o.code = setter.ResponseUpdated
	o.active = map[ipnet.Family]bool{ipnet.IP4: true}
	o.configured = map[ipnet.Family]bool{ipnet.IP4: true}
	o.result.Before = &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{old}}
	o.result.Batches = []setter.WAFBatch{{Create: nil, Delete: []api.WAFListItem{old}, Result: &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: nil, ReturnedItems: nil}}}
	text := renderWAF(o)
	require.Contains(t, text, "Removal completed")
	require.Contains(t, text, "Could not read the returned list contents")
	require.NotContains(t, text, "Removed:")
}

func TestRenderContentBlocks(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("host.example", setter.ResponseUpdated, reportingSnapshot("192.0.2.1"))
	o.result.Before.Cached = true
	text := Compose(NewReportingState(true).Assess(reportingMessage(o)), LifecycleContext{Kind: LifecycleStartup, Schedule: "@every 1h"}).Notification().Format()
	require.Len(t, strings.Split(text, "\n\n"), 2)
	require.Contains(t, text, "Updated:")
	require.Contains(t, text, "192.0.2.1 (cached) → 192.0.2.2")
	require.NotContains(t, text, "Before:")
	require.NotContains(t, text, ";")
}

func TestRenderFamilyProblemsPrecedeWAF(t *testing.T) {
	t.Parallel()
	m := generateDetectMessage(ipnet.IP6, false)
	var waf wafOutcome
	waf.code = setter.ResponseFailed
	m.waf = []wafOutcome{waf}
	text := Compose(NewReportingState(false).Assess(m), LifecycleContext{Kind: LifecycleNone, Schedule: ""}).Notification().Format()
	require.Less(t, strings.Index(text, "IPv6"), strings.Index(text, "WAF"))
}

func TestRenderPartialPairsAndEmptyChronology(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("host.example", setter.ResponseFailed, reportingSnapshot("192.0.2.1"))
	o.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordUpdate, setter.DNSOperationUnconfirmed, "record", "192.0.2.2")}
	text := renderDNS(makeDNSRow(assessedDNS{outcome: o, firstKnown: false, unexplained: false}))
	require.Contains(t, text, "Update unconfirmed:")
	require.Contains(t, text, "192.0.2.1 → 192.0.2.2")
	require.NotContains(t, text, "Updated:")
	for _, status := range []setter.DNSOperationStatus{setter.DNSOperationBlocked, setter.DNSOperationUnconfirmed} {
		o.result.Before = reportingSnapshot("")
		o.result.Progress.Operations = []setter.DNSOperation{reportingOperation(api.RecordCreate, status, "", "192.0.2.2")}
		text = renderDNS(makeDNSRow(assessedDNS{outcome: o, firstKnown: true, unexplained: false}))
		if status == setter.DNSOperationUnconfirmed {
			require.Contains(t, text, "No addresses before the update")
		} else {
			require.Contains(t, text, "Current: no addresses")
		}
	}
}

func TestRenderUnknownEmptyCleanup(t *testing.T) {
	t.Parallel()
	o := reportingOutcome("host.example", setter.ResponseFailed, nil)
	o.target = nil
	text := renderDNS(makeDNSRow(assessedDNS{outcome: o, firstKnown: false, unexplained: false}))
	require.Contains(t, text, "Cleanup blocked")
	require.NotContains(t, text, "no addresses")
	require.NotContains(t, text, "Current")
}

// These are bounded renderer inputs. Ordinary create is visited before delete;
// nil create, accepted ordinary work, and blocked removal are defensive only.
func TestRenderWAFOperationGroups(t *testing.T) {
	t.Parallel()
	statuses := []struct {
		name   string
		result *api.WAFBatchResult
	}{
		{"not attempted", nil},
		{"blocked", &api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchNoEffect, Returned: nil, ReturnedItems: nil}},
		{"unconfirmed", &api.WAFBatchResult{PrerequisiteMutation: false, OK: false, Status: api.WAFBatchUnconfirmed, Returned: nil, ReturnedItems: nil}},
		{"completed", &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchCompleted, Returned: nil, ReturnedItems: nil}},
		{"started", &api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchAccepted, Returned: nil, ReturnedItems: nil}},
	}
	operands := []string{"", "192.0.2.1/32", "192.0.2.1/32 and 192.0.2.2/32", "192.0.2.1/32, 192.0.2.2/32, and 192.0.2.3/32"}
	for _, status := range statuses {
		for count, values := range operands {
			for _, cached := range []bool{false, true} {
				for _, removal := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%d/cached-%t/removal-%t", status.name, count, cached, removal), func(t *testing.T) {
						t.Parallel()
						var items []api.WAFListItem
						var creates []api.WAFListCreateItem
						for i := range count {
							prefix := netip.MustParsePrefix(fmt.Sprintf("192.0.2.%d/32", i+1))
							items = append(items, api.WAFListItem{ID: api.ID(strconv.Itoa(i)), Prefix: prefix, Comment: "owned"})
							creates = append(creates, api.WAFListCreateItem{Prefix: prefix, Comment: "owned"})
						}
						var o wafOutcome
						o.configured = map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
						o.code = setter.ResponseFailed
						o.result.Before = &api.WAFSnapshot{Exists: true, Cached: cached, Items: items}
						batch := setter.WAFBatch{Create: creates, Delete: nil, Result: status.result}
						label := "Addition " + status.name
						if removal {
							batch.Create, batch.Delete = nil, items
							label = "Removal " + status.name
							if cached {
								label += " (cached)"
							}
						}
						o.result.Batches = []setter.WAFBatch{batch}
						text := renderWAF(o)
						require.NotContains(t, text, "batch")
						if count == 0 {
							require.NotContains(t, text, "Addition ")
							require.NotContains(t, text, "Removal ")
						} else {
							require.Contains(t, text, label+": "+values)
							require.NotContains(t, text, "Added:")
							require.NotContains(t, text, "Removed:")
							if status.name == "completed" {
								require.Contains(t, text, "Could not read the returned list contents")
							}
						}
					})
				}
			}
		}
	}
}

// Empty accepted item stages are defensive: cleanup producers require item IDs.
func TestRenderWAFAcceptedCleanupGroups(t *testing.T) {
	t.Parallel()
	operands := []string{"", "192.0.2.1/32", "192.0.2.1/32 and 192.0.2.2/32", "192.0.2.1/32, 192.0.2.2/32, and 192.0.2.3/32"}
	for count, values := range operands {
		for _, cached := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/cached-%t", count, cached), func(t *testing.T) {
				t.Parallel()
				var items []api.WAFListItem
				for i := range count {
					items = append(items, api.WAFListItem{ID: api.ID(strconv.Itoa(i)), Prefix: netip.MustParsePrefix(fmt.Sprintf("192.0.2.%d/32", i+1)), Comment: "owned"})
				}
				var o wafOutcome
				o.configured = map[ipnet.Family]bool{ipnet.IP4: true, ipnet.IP6: true}
				o.cleanup, o.code = true, setter.ResponseUpdating
				o.result.Cleanup = &api.WAFCleanupResult{Code: api.WAFListCleanupUpdating, Before: &api.WAFSnapshot{Exists: true, Cached: cached, Items: items}, Stages: []api.WAFCleanupStage{{WholeList: false, Items: items, Result: api.WAFBatchResult{PrerequisiteMutation: false, OK: true, Status: api.WAFBatchAccepted, Returned: nil, ReturnedItems: nil}}}}
				text := renderWAF(o)
				require.Equal(t, 1, strings.Count(text, "Cleanup started"))
				require.NotContains(t, text, "Removal")
				require.NotContains(t, text, "Removed")
				require.NotContains(t, text, "completed")
				require.NotContains(t, text, "Current")
				if count == 0 {
					require.Contains(t, text, "before cleanup")
				} else {
					require.Contains(t, text, ": "+values)
					require.Contains(t, text, "managed item")
					require.Equal(t, 1, strings.Count(text, "192.0.2.1/32"))
				}
				if cached {
					require.Contains(t, text, "(cached)")
				} else {
					require.NotContains(t, text, "(cached)")
				}
			})
		}
	}
}

func TestRenderDNSUnexpectedNoopFamily(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		family         ipnet.Family
		address, label string
	}{
		{ipnet.IP4, "192.0.2.1", "No IPv4 update needed"},
		{ipnet.IP6, "2001:db8::1", "No IPv6 update needed"},
	} {
		for _, cached := range []bool{false, true} {
			o := reportingOutcome("host.example", setter.ResponseNoop, reportingSnapshot(tc.address))
			o.scope.family, o.result.Before.Cached = tc.family, cached
			row := makeDNSRow(assessedDNS{outcome: o, firstKnown: false, unexplained: true})
			text := renderDNS(row)
			label := tc.label
			if cached {
				label += " (cached)"
			}
			require.Contains(t, text, label+": "+tc.address)
			require.Contains(t, text, "Detected an unexpected address change")
			row.unexplained = false
			ordinary := renderDNS(row)
			require.Contains(t, ordinary, "Current")
			require.NotContains(t, ordinary, "update needed")
		}
	}
}
