package setter

import (
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
)

// WAFBatch owns one planned batch. Create carries requested prefixes/comments;
// Delete carries original IDs, actual prefixes and comments. Required create
// work precedes required delete work. Result is nil for an unvisited batch.
// An existing empty delete call may have a nonnil NoEffect result; it is not
// blocked required work. Reached results retain actual returned content and
// membership evidence independently of their work Boolean.
type WAFBatch struct {
	Create []api.WAFListCreateItem
	Delete []api.WAFListItem
	Result *api.WAFBatchResult
}

// WAFResult owns Code, input and progress independently of caches and API
// storage. Before is nil on initial-read failure and full selected input on
// success, including unavailable/out-of-scope families. Consumers project
// active families; target coverage must not replace actual item prefixes.
// Batches retains required work including the unvisited suffix after failure;
// no-op has no batches. Cleanup owns final-cleanup evidence instead of a
// reconciliation plan. Cleanup may occur with nil Before. Code retains the
// original work, heartbeat and waiting policy.
type WAFResult struct {
	Code    ResponseCode
	Before  *api.WAFSnapshot
	Batches []WAFBatch
	Cleanup *api.WAFCleanupResult
}

func cloneWAFSnapshot(snapshot *api.WAFSnapshot) *api.WAFSnapshot {
	if snapshot == nil {
		return nil
	}
	result := *snapshot
	result.Items = slices.Clone(snapshot.Items)
	return &result
}

func cloneWAFBatchResult(result api.WAFBatchResult) *api.WAFBatchResult {
	result.Returned = cloneWAFSnapshot(result.Returned)
	result.ReturnedItems = slices.Clone(result.ReturnedItems)
	return &result
}

func cloneWAFCleanupResult(result api.WAFCleanupResult) *api.WAFCleanupResult {
	result.Before = cloneWAFSnapshot(result.Before)
	result.Stages = slices.Clone(result.Stages)
	for i := range result.Stages {
		stage := &result.Stages[i]
		stage.Items = slices.Clone(stage.Items)
		stage.Result = *cloneWAFBatchResult(stage.Result)
	}
	return &result
}
