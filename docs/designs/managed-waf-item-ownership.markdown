# Design Note: Managed WAF List Item Ownership

`MANAGED_WAF_LIST_ITEM_COMMENT_REGEX` is the WAF counterpart of managed DNS record ownership. It lets one updater instance decide which existing WAF list items it owns within a configured custom WAF list.

## Goal

Safely isolate ownership when multiple updater instances may touch the same WAF list. Ownership affects item discovery, updates, stale-item deletion, and shutdown cleanup.

## Core Model

- `WAF_LIST_ITEM_COMMENT` is the comment this instance writes to WAF list items that it creates.
- `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX` is the selector used to decide which existing WAF list items are managed by this instance.
- These settings are intentionally separate: one controls what this instance writes, and the other controls what it may mutate.

The selector uses Go `regexp` RE2 syntax with `MatchString` semantics. It is not an implicit full-match pattern.

The empty default matches all comments, preserving pre-feature behavior. Ownership isolation is opt-in.

## Required Invariants

- `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX` is compiled during configuration normalization and stored in canonical form.
- After successful normalization, the compiled regex is always non-nil, including the default empty template.
- `WAF_LIST_ITEM_COMMENT` must match `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX`.

The last rule prevents self-orphaning.

## Reconciliation Semantics

Managed-item filtering happens immediately after listing WAF list items from Cloudflare.

Only matched items participate in:

- coverage checks for detected IP addresses
- stale-item deletion during list reconciliation
- comment-aware warnings about managed items

Unmatched items are invisible to WAF mutation logic. As a result, the updater may create a new managed item even if an unmanaged item already covers the desired IP address.

## Shutdown Deletion Semantics

With `DELETE_ON_STOP`, shutdown cleanup should delete only managed WAF list items matched by `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX`.

The updater should not assume whole-list ownership once WAF item ownership exists. Deleting or clearing the full list is only correct when the ownership boundary also covers the whole list.

## Caching Contract

WAF list item caches store already-filtered managed items.

This relies on one handle and its bound setter using one stable managed-item filter for their lifetime. The current cache key does not include filter identity.

Cloudflare item-creation and item-deletion APIs return the whole list content. Managed-item filtering must therefore be reapplied before refreshing the cache.

## Tradeoffs

- The design prefers strict ownership isolation over reusing foreign items. This may leave parallel items that cover the same IP address, but it avoids mutating another deployment's list entries.
- Regex selectors allow flexible grouping, but exact ownership boundaries require explicit anchors such as `^managed-by-a$`.
- The selector name is intentionally distinct from `WAF_LIST_ITEM_COMMENT` to reduce operator confusion.

## Naming Notes

`MANAGED_WAF_LIST_ITEM_COMMENT_REGEX` was kept instead of a name closer to `WAF_LIST_ITEM_COMMENT` or a boolean-style form such as `WAF_LIST_ITEM_COMMENT_AS_FILTER`.

The main reason is operator safety: the selector is about management scope, not the default comment for newly written list items. A name that is too close to `WAF_LIST_ITEM_COMMENT` increases the risk of copy-paste and scanning mistakes in environment-variable-heavy setups.

## Ownership-Specific Warning Triggers

Following the project-wide warning policy in [`codebase-architecture.markdown`](codebase-architecture.markdown), this ownership feature should warn only when the configuration or observed list content strongly suggests a shared-ownership mistake.

### Recommended Warnings

| Scope | Trigger | Proposed message |
| --- | --- | --- |
| config time | `WAF_LISTS` is non-empty, `MANAGED_RECORDS_COMMENT_REGEX` is non-empty, and `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX` is empty | `MANAGED_RECORDS_COMMENT_REGEX enables DNS ownership isolation, but MANAGED_WAF_LIST_ITEM_COMMENT_REGEX is empty for configured WAF lists. All items in WAF_LISTS will still be treated as managed.` |
| config time | `WAF_LISTS` is non-empty, `WAF_LIST_ITEM_COMMENT` is non-empty, and `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX` is empty | `WAF_LIST_ITEM_COMMENT=%q only affects newly created WAF list items. Existing items with any comment are still managed because MANAGED_WAF_LIST_ITEM_COMMENT_REGEX is empty.` |
| config time | `DELETE_ON_STOP=true`, `WAF_LISTS` is non-empty, and `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX` is empty | `DELETE_ON_STOP=true with an empty MANAGED_WAF_LIST_ITEM_COMMENT_REGEX will delete all items in WAF_LISTS, including items created by other deployments.` |
| runtime | `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX` is empty, and a listed WAF list contains multiple distinct non-empty item comments | `The list %s contains multiple distinct non-empty item comments, but MANAGED_WAF_LIST_ITEM_COMMENT_REGEX is empty. The list may be shared with other deployments.` |

### Warnings to Avoid

- Do not warn on every empty `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX`. The empty default is valid and intentionally preserves pre-feature behavior.
- Do not warn based only on heuristic regex style, such as missing `^...$` anchors.
- Do not warn merely because DNS and WAF comment values differ. Different write-comments are often intentional.

## Scope Boundary

This design applies only to WAF list item ownership based on WAF list item comments.

It is not a general ownership abstraction for all managed resources. DNS record ownership remains separate, and WAF-less or DNS-only runs do not use this selector.

## Future Development Notes

- If one process ever needs multiple ownership scopes for the same WAF list, the cache design must change so filter identity becomes part of the caching model.
- Future warnings should continue to distinguish between "what this instance writes" and "what this instance manages", especially for shared WAF lists.
- If future work needs shared ownership rules across DNS and WAF resources, that should be designed as a new abstraction instead of coupling the two selectors implicitly.
