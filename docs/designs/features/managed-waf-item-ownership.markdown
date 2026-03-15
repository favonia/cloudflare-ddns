# Design Note: Managed WAF List Item Ownership

Read when: changing WAF list ownership, managed-item filtering, or ownership-aware WAF cleanup semantics.

Defines: the durable contract for `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`, `WAF_LIST_ITEM_COMMENT`, and ownership-aware WAF reconciliation.

Does not define: exact warning text or repository-wide naming policy.

`MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` lets each updater instance decide which existing WAF list items it owns.

## Goal

Isolate ownership safely when multiple updater instances may touch the same WAF list. Ownership affects item discovery, mutation scope, and shutdown cleanup, but it is only one part of the WAF semantic model.

## Core Model

- `WAF_LIST_ITEM_COMMENT` is the comment this instance writes to WAF list items that it creates.
- `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` is the selector used to decide which existing WAF list items are managed by this instance.
- These settings are separate by design: one controls writes, and one controls mutation scope.

Ownership is orthogonal to family intent and lifecycle ownership.

- Ownership scope answers which existing items this updater may touch.
- Family scope answers whether an IP family is in scope for this run.
- Desired-target intent answers which target IPs the updater wants covered for an in-scope family.
- Lifecycle ownership answers whether shutdown may delete only matched items or the whole list root.

Shared family-scope and desired-target semantics are defined in [IP Family Intent and Target Providers](ip-family-intent-and-target-providers.markdown). This note defines WAF ownership and its interaction with family and lifecycle scope.

Shared reconciliation and lifecycle-ownership semantics are defined in [Unified Reconciliation and Lifecycle Ownership](unified-reconciliation-and-lifecycle-ownership.markdown). This note defines the WAF-specific instantiation.

The selector uses Go `regexp` RE2 syntax with `MatchString` semantics, not implicit full-match behavior.

The empty default matches all comments, preserving pre-feature behavior. Ownership isolation is opt-in.

## Required Invariants

- `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` is compiled during config building and stored in runtime form.
- After successful config building, the compiled regex is always non-nil, including the default empty template.
- `WAF_LIST_ITEM_COMMENT` must match `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`.

The last rule prevents self-orphaning.

## Reconciliation Semantics

Managed-item filtering happens immediately after listing items from Cloudflare.

Only matched items participate in:

- coverage checks for desired target IPs
- stale-item deletion during list reconciliation
- comment-aware warnings about managed items
- `DELETE_ON_STOP` in active cleanup scope

Unmatched items are invisible to WAF mutation logic, so the updater may create a new managed item even if an unmanaged item already covers the target IP address.

### WAF Instantiation

WAF instantiates the unified reconciliation model with these resource-specific rules:

- the resource unit is `(list, IP family)`
- a managed item satisfies a desired target when it covers that desired target IP
- overlapping managed coverage may remain
- retained coverage sets may stay history-dependent
- already-satisfying item metadata is soft unless another WAF-specific contract overrides it

### Metadata for New Creates

WAF reconciliation resolves create metadata independently per `(list, IP family)` unit from recyclable managed items only.

- In this scope, the only managed metadata field is item `comment`.
- Create comment resolution uses family-local items scheduled for deletion:
  - empty source set: use configured `WAF_LIST_ITEM_COMMENT`
  - unanimous source comment: inherit source comment
  - non-unanimous source comments: use configured comment and emit one ambiguity warning for that family field

### Path-Independence Boundary

Path-independence is a secondary stability goal for WAF comment reconciliation, after coverage safety and ownership isolation.

For successful create-then-delete rounds, when a drift step creates managed items and a later drift step makes those items recyclable, the resolved create comment should match the direct one-step transition outcome from the earlier recyclable source.

This boundary is intentionally narrower than full state canonicalization. Keep-and-cover still preserves any managed items that already cover desired targets, so retained coverage sets may remain history-dependent even when create-comment resolution is path-stable under the drift pattern above.

### Interruption-Aware Priority

WAF reconciliation should minimize residual risk under ambiguous partial execution.

- Missing desired coverage is higher risk than stale managed coverage.
- Stale managed coverage is higher risk than metadata or hygiene residue.

Any implementation should order work so higher-risk residual states are reduced before lower-risk ones. This note intentionally records risk order, not one exact operation list.

### Failure and Shutdown Semantics

When family-scope and desired-target semantics are defined elsewhere, WAF ownership interacts with them as follows:

- Out-of-scope family intent preserves existing managed items of that family.
- Explicit-empty family intent reconciles that family to no managed items.
- Temporary target-set unavailability preserves existing managed items because desired targets are unknown.

## Shutdown Deletion Semantics

Lifecycle ownership determines shutdown authority.

- WAF lists are potentially root-owned resources because the updater can create them from configuration alone.
- A WAF list is only root-owned for a run when the updater's current cleanup scope covers all semantic content it owns in that list.
- Shared ownership or out-of-scope family preservation makes the list member-owned for that run.
- Root-owned cleanup may delete the whole list.
- Member-owned cleanup deletes only managed items in active family scope.

The empty selector default can still imply full ownership, but selector emptiness alone is not the semantic rule. Cleanup authority comes from ownership plus active family scope.

## Caching Contract

WAF list item caches store already-filtered managed items.

This relies on one handle and its bound setter using one stable managed-item filter for their lifetime. The cache key does not include filter identity.

Cloudflare item-creation and item-deletion APIs return whole-list content, so managed-item filtering must be reapplied before refreshing the cache.

## Tradeoffs

- The design favors strict ownership isolation over reusing foreign items. This may leave parallel items that cover the same IP address, but it avoids mutating another deployment's entries.
- Regex selectors allow flexible grouping, but exact ownership boundaries require explicit anchors such as `^managed-by-a$`.
- The selector name is intentionally distinct from `WAF_LIST_ITEM_COMMENT` to reduce operator confusion.

## Scope Boundary

This design applies only to WAF list item ownership based on WAF list item comments.

It is not a general ownership abstraction for all managed resources. DNS record ownership remains separate, and WAF-less or DNS-only runs do not use this selector.

## Extension Points

- If one process ever needs multiple ownership scopes for the same WAF list, the cache design must change so filter identity becomes part of the caching model.
- Future configuration and UI work should continue to keep ownership selection separate from the parameters written to WAF list items.
- If future work needs shared ownership rules across DNS and WAF resources, that should be designed as a new abstraction instead of coupling the two selectors implicitly.
