package updater

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// This tests display canonicalization, not setter planning. Its contract compares
// evidence independently of encounter order, including updates with equal targets.
func TestDNSDisplayOperationsCanonicalizeEqualTargets(t *testing.T) {
	t.Parallel()
	var evidence dnsEvidence
	for _, old := range []string{"192.0.2.3", "192.0.2.1"} {
		evidence.operations = append(evidence.operations, dnsOperationEvidence{
			operation: reportingOperation(api.RecordUpdate, setter.DNSOperationCompleted, old, "192.0.2.9"),
			old:       api.RecordFact{ID: api.ID(old), IP: netip.MustParseAddr(old)}, paired: true, identityKnown: true,
		})
	}
	forward := evidence.displayOperations()
	slices.Reverse(evidence.operations)
	require.Equal(t, forward, evidence.displayOperations())
	require.Len(t, forward, 2, "distinct old operands must not collapse into one update")
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), forward[0].old)
	require.Equal(t, netip.MustParseAddr("192.0.2.3"), forward[1].old)
}

func TestRenderNoLifecycleHasNoText(t *testing.T) {
	t.Parallel()
	require.Empty(t, renderLifecycle(NewNoLifecycleContext()))
}

// Completed work and observed contents are independent evidence. A parsed empty
// response must not be replaced by the requested addition, even on successful work.
func TestRenderWAFCompletedAdditionCanObserveEmptyContents(t *testing.T) {
	t.Parallel()
	for _, existed := range []bool{false, true} {
		for _, filtered := range []bool{false, true} {
			var outcome wafOutcome
			outcome.filtered = filtered
			outcome.scope = wafScope{ipnet.IP4: true}
			outcome.result.Code = setter.ResponseUpdated
			outcome.result.Before = &api.WAFSnapshot{Exists: existed, Cached: true, Items: nil}
			outcome.result.Batches = []setter.WAFBatch{{
				Create: []api.WAFListCreateItem{{Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}}, Delete: nil,
				Result: &api.WAFBatchResult{
					ListCreationDispatched: !existed, OK: true, Status: api.WAFBatchCompleted,
					ReturnedManaged: &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}, ReturnedAllItems: nil,
				},
			}}
			content := outcome.content()
			require.True(t, content.deltaKnown)
			require.Empty(t, content.added)
			require.Empty(t, content.removed)
			require.Empty(t, content.unchanged)
			require.NotNil(t, content.observed)
			require.True(t, content.observed.Exists)
			require.Empty(t, content.observed.Items)
			text := renderWAF(outcome)
			require.Contains(t, text, "Current")
			if filtered {
				require.Contains(t, text, "managed items: none")
			} else {
				require.Contains(t, text, ": no items")
			}
			require.NotContains(t, text, "192.0.2.1")
			require.NotContains(t, text, "Added:")
			require.NotContains(t, text, "unconfirmed")
			require.NotContains(t, text, "Current (cached)")
		}
	}
}

// These formatter fixtures test absent inventory and historical absence directly;
// they do not claim that a cleanup producer emits a missing-list write operation.
func TestRenderInactiveWAFRequiresExistingInventory(t *testing.T) {
	t.Parallel()
	var outcome wafOutcome
	outcome.scope = wafScope{ipnet.IP4: true, ipnet.IP6: false}
	var content wafContent
	require.Empty(t, outcome.renderInactiveWAF(content))
	content.before = &api.WAFSnapshot{Exists: false, Cached: true, Items: nil}
	require.Empty(t, outcome.renderInactiveWAF(content))
	content.observed = &api.WAFSnapshot{Exists: false, Cached: false, Items: nil}
	require.Empty(t, outcome.renderInactiveWAF(content))
}

func TestRenderWAFHistoricalAbsenceUsesOperationTiming(t *testing.T) {
	t.Parallel()
	for _, cached := range []bool{false, true} {
		var outcome wafOutcome
		outcome.cleanup = true
		snapshot := &api.WAFSnapshot{Exists: false, Cached: cached, Items: nil}
		text := strings.Join(renderWAFContext(outcome, snapshot, wafInventoryBeforeOperation), "\n")
		require.Contains(t, text, "did not exist before cleanup")
		require.Equal(t, cached, strings.Contains(text, "(cached)"))
		require.NotContains(t, text, "Current")
		outcome.cleanup = false
		text = strings.Join(renderWAFContext(outcome, snapshot, wafInventoryBeforeOperation), "\n")
		require.Contains(t, text, "did not exist before the update")
		require.NotContains(t, text, "cleanup")
	}
}
