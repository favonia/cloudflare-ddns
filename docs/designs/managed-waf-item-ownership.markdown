# Design Note: Managed WAF List Item Ownership

`MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` lets each updater instance decide which existing WAF list items it owns.

## Goal

Isolate ownership safely when multiple updater instances may touch the same WAF list. Ownership affects item discovery, updates, stale-item deletion, and shutdown cleanup.

## Core Model

- `WAF_LIST_ITEM_COMMENT` is the comment this instance writes to WAF list items that it creates.
- `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` is the selector used to decide which existing WAF list items are managed by this instance.
- These settings are separate by design: one controls writes, and one controls mutation scope.

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

- coverage checks for detected IP addresses
- stale-item deletion during list reconciliation
- comment-aware warnings about managed items
- `DELETE_ON_STOP` in ownership-aware mode

Unmatched items are invisible to WAF mutation logic, so the updater may create a new managed item even if an unmanaged item already covers the target IP address.

### Metadata Reconciliation for New Creates

WAF reconciliation resolves create metadata independently per `(list, IP family)` unit.

- In this scope, the only managed metadata field is item `comment`.
- Create comment resolution uses family-local items scheduled for deletion:
  - empty source set: use configured `WAF_LIST_ITEM_COMMENT`
  - unanimous source comment: inherit source comment
  - non-unanimous source comments: use configured comment and emit one ambiguity warning for that family field

### Path-Independence Boundary

Path-independence is a secondary stability goal for comment reconciliation, after coverage safety and ownership isolation.

For successful create-then-delete rounds, when a drift step creates managed items and a later drift step makes those items stale, the resolved create comment should match the direct one-step transition outcome from the earlier stale source.

This is intentionally narrower than full state canonicalization. Keep-and-fill still preserves any managed ranges that cover current targets, so retained range sets can remain history-dependent even when create-comment resolution is path-stable under the drift pattern above.

Execution order remains create-before-delete to reduce temporary coverage gaps.

This ordering is intentional for interruption resilience:

1. create missing coverage first,
2. then delete stale items.

Under timeouts or ambiguous network failures, partial execution therefore favors coverage over cleanup.

## Shutdown Deletion Semantics

`DELETE_ON_STOP` has two WAF modes:

- With a non-empty `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`, shutdown cleanup deletes only matched managed items.
- With the empty default selector, shutdown cleanup first tries deleting the whole list.
- The mode switch uses only the configured selector template being empty or non-empty.
- Do not infer "match-all" behavior from general regex semantics when selecting cleanup mode.

The empty default is preserved for backward compatibility, but it is ambiguous in shared-list deployments and should be documented and warned about carefully.

### Final Cleanup Execution Model

Both modes share one cleanup state machine after list discovery:

1. Check whether the target list exists.
2. If missing, treat cleanup as already complete (`Noop`).
3. Select cleanup scope (managed items for shared ownership; all items for whole-list fallback).
4. Start asynchronous item deletion for that scope.

The operational difference between the two modes is only one pre-step:

- Whole-list ownership tries deleting the whole list first.
- Shared ownership skips that pre-step.

If whole-list ownership cannot find the list during final cleanup, it emits a warning and returns `Noop`. This keeps cleanup idempotent while still surfacing drift.

User-facing cleanup messages should prefer the operator-facing phrase "items managed by this updater" over internal shorthand like "managed items."

## Caching Contract

WAF list item caches store already-filtered managed items.

This relies on one handle and its bound setter using one stable managed-item filter for their lifetime. The cache key does not include filter identity.

Cloudflare item-creation and item-deletion APIs return whole-list content, so managed-item filtering must be reapplied before refreshing the cache.

## Tradeoffs

- The design favors strict ownership isolation over reusing foreign items. This may leave parallel items that cover the same IP address, but it avoids mutating another deployment's entries.
- Regex selectors allow flexible grouping, but exact ownership boundaries require explicit anchors such as `^managed-by-a$`.
- The selector name is intentionally distinct from `WAF_LIST_ITEM_COMMENT` to reduce operator confusion.

## Naming Notes

`MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` follows the shared naming convention in [`codebase-architecture.markdown`](codebase-architecture.markdown): write-side settings stay singular, while ownership selectors stay plural.

The main reason is operator safety: the selector defines management scope across a set of items, not the default comment for one newly written item. Keeping write-side settings singular and ownership selectors plural makes that distinction easier to scan in environment-variable-heavy setups.

## Ownership-Specific Warning Triggers

Following the project-wide warning policy in [`codebase-architecture.markdown`](codebase-architecture.markdown), this feature should warn only when configuration or observed list content strongly suggests a shared-ownership mistake.

### Recommended Warnings

| Scope       | Trigger                                                                                                                     | Proposed message                                                                                                                                                                                      |
| ----------- | --------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| config time | `WAF_LISTS` is non-empty, `MANAGED_RECORDS_COMMENT_REGEX` is non-empty, and `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` is empty | `MANAGED_RECORDS_COMMENT_REGEX enables DNS ownership isolation, but MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX is empty for configured WAF lists. All items in WAF_LISTS will still be treated as managed.` |
| config time | `WAF_LISTS` is non-empty, `WAF_LIST_ITEM_COMMENT` is non-empty, and `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` is empty         | `WAF_LIST_ITEM_COMMENT (%s) does not change which existing items are managed. Existing items with any comment are still managed because MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX is empty.`                |
| config time | `DELETE_ON_STOP=true`, `WAF_LISTS` is non-empty, and `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` is empty                        | `DELETE_ON_STOP=true with an empty MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX will delete all items in WAF_LISTS, including items created by other deployments.`                                            |
| runtime     | `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` is empty, and a listed WAF list contains multiple distinct non-empty item comments   | `The list %s contains multiple distinct non-empty item comments, but MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX is empty. The list may be shared with other deployments.`                                   |

### Warnings to Avoid

- Do not warn on every empty `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`. The empty default is valid and preserves pre-feature behavior.
- Do not warn based only on heuristic regex style, such as missing `^...$` anchors.
- Do not warn merely because DNS and WAF comment values differ. Different write-comments are often intentional.

## Scope Boundary

This design applies only to WAF list item ownership based on WAF list item comments.

It is not a general ownership abstraction for all managed resources. DNS record ownership remains separate, and WAF-less or DNS-only runs do not use this selector.

## Future Development Notes

- If one process ever needs multiple ownership scopes for the same WAF list, the cache design must change so filter identity becomes part of the caching model.
- Future configuration and UI work should continue to keep ownership selection separate from the parameters written to WAF list items.
- If future work needs shared ownership rules across DNS and WAF resources, that should be designed as a new abstraction instead of coupling the two selectors implicitly.
