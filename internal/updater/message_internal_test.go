package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestMessageFailed(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{ID: "item", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "owned"}
	for _, tc := range []struct {
		code        setter.ResponseCode
		cleanupCode api.WAFListCleanupCode
		status      api.WAFBatchStatus
	}{
		{setter.ResponseNoop, api.WAFListCleanupNoop, api.WAFBatchNoEffect},
		{setter.ResponseUpdated, api.WAFListCleanupUpdated, api.WAFBatchCompleted},
		{setter.ResponseUpdating, api.WAFListCleanupUpdating, api.WAFBatchAccepted},
		{setter.ResponseFailed, api.WAFListCleanupFailed, api.WAFBatchUnconfirmed},
	} {
		before := &api.WAFSnapshot{Exists: true, Cached: false, Items: []api.WAFListItem{item}}
		cleanup := &api.WAFCleanupResult{Code: tc.cleanupCode, Before: before, Stages: nil}
		if tc.code == setter.ResponseNoop {
			before.Items = nil
		} else {
			cleanup.Stages = []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{item}, Result: api.WAFBatchResult{
				PrerequisiteMutation: false, OK: tc.code != setter.ResponseFailed, Status: tc.status,
				Returned: nil, ReturnedItems: nil,
			}}}
		}
		if tc.code == setter.ResponseUpdated {
			cleanup.Stages[0].Result.Returned = &api.WAFSnapshot{Exists: true, Cached: false, Items: nil}
		}
		message := newMessage()
		message.cleanup = true
		message.waf = []wafOutcome{{
			result:   setter.WAFResult{Code: tc.code, Before: before, Cleanup: cleanup, Batches: nil},
			filtered: true, scope: wafScope{ipnet.IP4: true}, list: api.WAFList{AccountID: "account", Name: "list"}, cleanup: true,
		}}
		message = message.withHeartbeat()
		require.Equal(t, tc.code == setter.ResponseFailed, message.Failed())
		require.Equal(t, !message.Failed(), message.HeartbeatMessage.OK)
	}
	require.False(t, mergeMessages().Failed())
	require.True(t, mergeMessages(generateDetectMessage(ipnet.IP4, false), generateDetectMessage(ipnet.IP6, true)).Failed())
	require.True(t, generateIP6DerivationFailureMessage(nil).Failed())
	require.True(t, generateFilterAbortDetectMessage(ipnet.IP4).Failed())
}

// Work decisions must depend on retained semantic evidence, even if a consumer
// modifies the public heartbeat projection before inspecting or merging results.
func TestMessageWorkResultIndependentOfHeartbeat(t *testing.T) {
	t.Parallel()
	failed := reportingMessage(reportingOutcome("failed.example", setter.ResponseFailed, nil))
	failed.HeartbeatMessage.OK = true
	failed.HeartbeatMessage.Lines = []string{"unrelated delivery text"}
	require.True(t, failed.Failed())
	merged := mergeMessages(failed)
	require.True(t, merged.Failed())
	require.False(t, merged.HeartbeatMessage.OK)
	require.NotContains(t, merged.HeartbeatMessage.Format(), "unrelated delivery text")

	succeeded := reportingMessage(reportingOutcome("success.example", setter.ResponseNoop, reportingSnapshot("192.0.2.1")))
	succeeded.HeartbeatMessage.OK = false
	require.False(t, succeeded.Failed())
	merged = mergeMessages(succeeded)
	require.False(t, merged.Failed())
	require.True(t, merged.HeartbeatMessage.OK)
}
