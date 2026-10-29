package api

import "slices"

// WAFSnapshot owns comment-selected item identities, actual prefixes and comments.
// Items span all families; consumers project their configured or active scope.
// A nil snapshot is unknown; a nonnil empty snapshot is selected-empty when
// Exists, and known missing otherwise. Cached is the provenance at admission,
// not a claim about remote contents at reporting time.
type WAFSnapshot struct {
	Exists bool
	Cached bool
	Items  []WAFListItem
}

// WAFBatchStatus describes evidence for a reached item batch or cleanup stage.
type WAFBatchStatus uint8

const (
	// WAFBatchNoEffect means the relevant content mutation was not dispatched.
	// This includes an empty batch or a failed prerequisite; structural list
	// creation during a prerequisite does not establish any item content.
	WAFBatchNoEffect WAFBatchStatus = iota
	// WAFBatchUnconfirmed means the mutation was dispatched but its completion
	// could not be confirmed. No individual item's success follows from this.
	WAFBatchUnconfirmed
	// WAFBatchCompleted means the SDK mutation returned successfully or cleanup
	// confirmation succeeded. This alone does not establish intended membership.
	WAFBatchCompleted
	// WAFBatchAccepted means asynchronous cleanup was accepted without waiting.
	WAFBatchAccepted
)

// WAFBatchResult owns evidence from a reached operation. OK preserves the work
// Boolean, including false when a successful SDK response cannot be parsed.
// Such a parser failure is Completed with unknown returned contents.
// Returned contains selected managed items; ReturnedItems contains ALL parsed
// response items, including comments outside ownership. Neither is inferred
// from the request. Both are nil when unavailable; a nonnil Returned with zero
// items establishes selected-empty. ReturnedItems must not be reported as owned
// content without checking Returned membership. All slices are detached from
// caches and request/response storage; returned snapshots are fresh and existing.
type WAFBatchResult struct {
	// PrerequisiteMutation is true when a structural list creation request was
	// dispatched before item work, including when that creation is unconfirmed.
	// NoEffect alone therefore does not establish unchanged list existence.
	PrerequisiteMutation bool
	OK                   bool
	Status               WAFBatchStatus
	Returned             *WAFSnapshot
	ReturnedItems        []WAFListItem
}

// WAFCleanupStage owns one reached cleanup mutation. WholeList denotes list
// deletion; otherwise Items retains the exact selected item IDs and prefixes
// submitted for deletion. A failed whole-list stage remains even if fallback
// succeeds. No content read is added to describe whole-list cleanup.
type WAFCleanupStage struct {
	WholeList bool
	Items     []WAFListItem
	Result    WAFBatchResult
}

// WAFCleanupResult owns admitted input and reached cleanup stages in order.
// Code preserves work policy. Before is nil when no contents were admitted;
// nil does not imply no cleanup. Missing lists have a known-missing Before.
// Before is full selected input, before projection to cleanup's family scope.
// With whole-list fallback it is admitted AFTER the failed whole-list stage,
// before item cleanup; it is not input evidence for that earlier stage.
// Stages has no unvisited work; item cleanup follows any whole-list attempt.
type WAFCleanupResult struct {
	Code   WAFListCleanupCode
	Before *WAFSnapshot
	Stages []WAFCleanupStage
}

func snapshotWAFItems(items []WAFListItem, exists, cached bool) *WAFSnapshot {
	return &WAFSnapshot{Exists: exists, Cached: cached, Items: slices.Clone(items)}
}
